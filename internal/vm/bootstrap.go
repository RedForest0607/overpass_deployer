package vm

import (
	"fmt"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

const packageManagerDNF = "dnf"
const packageManagerYUM = "yum"
const defaultJDKVendor = "corretto"

// BootstrapHost는 서버의 OS 업데이트와 필수 패키지/JDK 설치를 준비하거나 실행한다.
func BootstrapHost(runner ssh.Runner, bootstrap config.BootstrapConfig, opts RunOptions, host string) error {
	host = runnerHost(runner, host)
	packages, err := packagesForBootstrap(bootstrap)
	if err != nil {
		return err
	}

	if !bootstrapEnabled(bootstrap, packages) {
		logger.Skip(host, "Bootstrap unchanged")
		return nil
	}

	logger.Info(host, "%s bootstrap packages...", actionLabel(opts, "checking"))

	if opts.DryRun {
		logBootstrapPlan(host, bootstrap, packages)
		logger.Ok(host, "%s bootstrap packages", resultLabel(opts, "checked", "planned"))
		return nil
	}

	if err := applyPackageBootstrap(runner, bootstrap, packages, host); err != nil {
		return err
	}
	if err := applyTimezoneBootstrap(runner, bootstrap.Timezone, host); err != nil {
		return err
	}
	if err := applySwapBootstrap(runner, bootstrap.Swap, host); err != nil {
		return err
	}
	if err := applyDockerComposeBootstrap(runner, bootstrap.DockerCompose, host); err != nil {
		return err
	}
	if err := applyServiceBootstrap(runner, bootstrap.Services, host); err != nil {
		return err
	}

	logger.Ok(host, "Completed bootstrap")
	return nil
}

// bootstrapEnabled는 설치할 패키지나 OS 업데이트 작업이 실제로 있는지 판단한다.
func bootstrapEnabled(bootstrap config.BootstrapConfig, packages []string) bool {
	if len(packages) > 0 {
		return true
	}
	if len(bootstrap.Services) > 0 {
		return true
	}
	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		return true
	}
	if bootstrap.Timezone.Name != "" {
		return true
	}
	if bootstrap.Swap.Enabled != nil && *bootstrap.Swap.Enabled {
		return true
	}
	return bootstrap.DockerCompose.Enabled != nil && *bootstrap.DockerCompose.Enabled
}

// logBootstrapPlan은 dry-run에서 실행 예정인 bootstrap 명령을 원격 변경 없이 출력한다.
func logBootstrapPlan(host string, bootstrap config.BootstrapConfig, packages []string) {
	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		logger.Info(host, "DRY-RUN: would run sudo yum/dnf update -y")
	}
	if len(packages) > 0 {
		logger.Info(host, "DRY-RUN: would check installed packages via rpm -q for %s", strings.Join(packages, ", "))
		logger.Info(host, "DRY-RUN: would run sudo yum/dnf install -y %s", strings.Join(packages, " "))
	} else {
		logger.Info(host, "DRY-RUN: no bootstrap packages to install")
	}
	if bootstrap.Timezone.Name != "" {
		logger.Info(host, "DRY-RUN: would set timezone to %s", bootstrap.Timezone.Name)
	}
	if bootstrap.Swap.Enabled != nil && *bootstrap.Swap.Enabled {
		logger.Info(host, "DRY-RUN: would ensure swap file %s with size %s", bootstrap.Swap.Path, bootstrap.Swap.Size)
	}
	if bootstrap.DockerCompose.Enabled != nil && *bootstrap.DockerCompose.Enabled {
		logger.Info(host, "DRY-RUN: would install Docker Compose %s from %s to %s", bootstrap.DockerCompose.Version, bootstrap.DockerCompose.SourceURL, bootstrap.DockerCompose.InstallPath)
	}
	for _, service := range bootstrap.Services {
		logger.Info(host, "DRY-RUN: would ensure service %s is %s and enabled=%t", service.Name, serviceState(service), serviceEnabled(service))
	}
}

func applyPackageBootstrap(runner ssh.Runner, bootstrap config.BootstrapConfig, packages []string, host string) error {
	if len(packages) == 0 && (bootstrap.OSUpdate.Enabled == nil || !*bootstrap.OSUpdate.Enabled) {
		return nil
	}

	manager, err := detectPackageManager(runner)
	if err != nil {
		return err
	}
	if manager != packageManagerDNF && manager != packageManagerYUM {
		return fmt.Errorf("bootstrap requires yum/dnf-compatible host; apt support is not implemented yet")
	}

	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		logger.Info(host, "Running bootstrap OS update via %s...", manager)
		if _, err := runner.RunSudo(manager + " update -y"); err != nil {
			return fmt.Errorf("running bootstrap os update: %w", err)
		}
	}
	if len(packages) == 0 {
		return nil
	}

	missingPackages, err := detectMissingPackages(runner, packages)
	if err != nil {
		return err
	}
	if len(missingPackages) == 0 {
		logger.Skip(host, "Bootstrap packages already installed")
		return nil
	}

	installCommand := buildPackageInstallCommand(manager, missingPackages)
	logger.Info(host, "Installing bootstrap packages: %s", strings.Join(missingPackages, ", "))
	if _, err := runner.RunSudo(installCommand); err != nil {
		return fmt.Errorf("installing bootstrap packages: %w", err)
	}

	logger.Ok(host, "Installed bootstrap packages")
	return nil
}

func applyTimezoneBootstrap(runner ssh.Runner, timezone config.TimezoneConfig, host string) error {
	if timezone.Name == "" {
		return nil
	}

	current, err := runner.Run("timedatectl show -p Timezone --value")
	if err != nil {
		return fmt.Errorf("checking timezone: %w", err)
	}
	if strings.TrimSpace(current) == timezone.Name {
		logger.Skip(host, "Timezone already set to %s", timezone.Name)
		return nil
	}

	logger.Info(host, "Setting timezone to %s...", timezone.Name)
	if _, err := runner.RunSudo("timedatectl set-timezone " + ssh.ShellQuote(timezone.Name)); err != nil {
		return fmt.Errorf("setting timezone: %w", err)
	}
	logger.Ok(host, "Set timezone to %s", timezone.Name)
	return nil
}

func applySwapBootstrap(runner ssh.Runner, swap config.SwapConfig, host string) error {
	if swap.Enabled == nil || !*swap.Enabled {
		return nil
	}

	active, err := runner.Run("swapon --noheadings --show=NAME")
	if err != nil {
		return fmt.Errorf("checking active swap: %w", err)
	}
	if containsLine(active, swap.Path) {
		logger.Skip(host, "Swap already active at %s", swap.Path)
		return ensureSwapInFstab(runner, swap.Path)
	}

	logger.Info(host, "Ensuring swap file %s (%s)...", swap.Path, swap.Size)
	if _, err := runner.RunSudo(buildSwapEnsureCommand(swap)); err != nil {
		activeAfter, checkErr := runner.Run("swapon --noheadings --show=NAME")
		if checkErr != nil || !containsLine(activeAfter, swap.Path) {
			return fmt.Errorf("enabling swap file: %w", err)
		}
		logger.Skip(host, "Swap already active at %s", swap.Path)
	}
	if err := ensureSwapInFstab(runner, swap.Path); err != nil {
		return err
	}

	logger.Ok(host, "Enabled swap file %s", swap.Path)
	return nil
}

func buildSwapEnsureCommand(swap config.SwapConfig) string {
	quotedPath := ssh.ShellQuote(swap.Path)
	quotedSize := ssh.ShellQuote(swap.Size)
	innerCommand := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("swap_path=%s", quotedPath),
		fmt.Sprintf("swap_size=%s", quotedSize),
		`if [ ! -f "${swap_path}" ]; then fallocate -l "${swap_size}" "${swap_path}"; fi`,
		`chmod 600 "${swap_path}"`,
		`if ! file -s "${swap_path}" | grep -qi 'swap file'; then mkswap "${swap_path}"; fi`,
		`swapon "${swap_path}" || swapon --noheadings --show=NAME | grep -Fxq "${swap_path}"`,
	}, "; ")

	return "sh -lc " + ssh.ShellQuote(innerCommand)
}

func applyDockerComposeBootstrap(runner ssh.Runner, compose config.DockerComposeConfig, host string) error {
	if compose.Enabled == nil || !*compose.Enabled {
		return nil
	}

	installed, err := dockerComposeInstalled(runner, compose)
	if err != nil {
		return err
	}
	if installed {
		logger.Skip(host, "Docker Compose %s already installed", compose.Version)
		return nil
	}

	logger.Info(host, "Installing Docker Compose %s...", compose.Version)
	if _, err := runner.RunSudo(buildDockerComposeInstallCommand(compose)); err != nil {
		return fmt.Errorf("installing docker compose: %w", err)
	}
	if _, err := runner.Run("docker compose version"); err != nil {
		return fmt.Errorf("verifying docker compose: %w", err)
	}

	logger.Ok(host, "Installed Docker Compose %s", compose.Version)
	return nil
}

func dockerComposeInstalled(runner ssh.Runner, compose config.DockerComposeConfig) (bool, error) {
	command := "test -x " + ssh.ShellQuote(compose.InstallPath) + " && docker compose version --short"
	out, err := runner.Run(command)
	if err != nil {
		return false, nil
	}
	return normalizeDockerComposeVersion(out) == normalizeDockerComposeVersion(compose.Version), nil
}

func normalizeDockerComposeVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}

func buildDockerComposeInstallCommand(compose config.DockerComposeConfig) string {
	installDir := ssh.ShellQuote(parentDir(compose.InstallPath))
	installPath := ssh.ShellQuote(compose.InstallPath)
	sourceURL := ssh.ShellQuote(compose.SourceURL)

	innerCommand := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("mkdir -p %s", installDir),
		fmt.Sprintf("if command -v curl >/dev/null 2>&1; then curl -fL %s -o %s; elif command -v wget >/dev/null 2>&1; then wget -O %s %s; else echo 'curl or wget is required to install Docker Compose' >&2; exit 1; fi", sourceURL, installPath, installPath, sourceURL),
		fmt.Sprintf("chmod 0755 %s", installPath),
	}, "; ")

	return "sh -lc " + ssh.ShellQuote(innerCommand)
}

func parentDir(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func applyServiceBootstrap(runner ssh.Runner, services []config.ServiceConfig, host string) error {
	for _, service := range services {
		if err := applySingleServiceBootstrap(runner, service, host); err != nil {
			return err
		}
	}
	return nil
}

func applySingleServiceBootstrap(runner ssh.Runner, service config.ServiceConfig, host string) error {
	if service.Name == "" {
		return nil
	}

	if serviceEnabled(service) {
		if out, err := runner.Run("systemctl is-enabled " + ssh.ShellQuote(service.Name)); err != nil || strings.TrimSpace(out) != "enabled" {
			logger.Info(host, "Enabling service %s...", service.Name)
			if _, err := runner.RunSudo("systemctl enable " + ssh.ShellQuote(service.Name)); err != nil {
				return fmt.Errorf("enabling service %s: %w", service.Name, err)
			}
		} else {
			logger.Skip(host, "Service %s already enabled", service.Name)
		}
	} else {
		if out, err := runner.Run("systemctl is-enabled " + ssh.ShellQuote(service.Name)); err == nil && strings.TrimSpace(out) == "enabled" {
			logger.Info(host, "Disabling service %s...", service.Name)
			if _, err := runner.RunSudo("systemctl disable " + ssh.ShellQuote(service.Name)); err != nil {
				return fmt.Errorf("disabling service %s: %w", service.Name, err)
			}
		} else {
			logger.Skip(host, "Service %s already disabled", service.Name)
		}
	}

	switch serviceState(service) {
	case config.ServiceStateStarted:
		if out, err := runner.Run("systemctl is-active " + ssh.ShellQuote(service.Name)); err == nil && strings.TrimSpace(out) == "active" {
			logger.Skip(host, "Service %s already active", service.Name)
			return nil
		}
		logger.Info(host, "Starting service %s...", service.Name)
		if _, err := runner.RunSudo("systemctl start " + ssh.ShellQuote(service.Name)); err != nil {
			return fmt.Errorf("starting service %s: %w", service.Name, err)
		}
		logger.Ok(host, "Started service %s", service.Name)
	case config.ServiceStateStopped:
		if out, err := runner.Run("systemctl is-active " + ssh.ShellQuote(service.Name)); err == nil && strings.TrimSpace(out) == "inactive" {
			logger.Skip(host, "Service %s already inactive", service.Name)
			return nil
		}
		logger.Info(host, "Stopping service %s...", service.Name)
		if _, err := runner.RunSudo("systemctl stop " + ssh.ShellQuote(service.Name)); err != nil {
			return fmt.Errorf("stopping service %s: %w", service.Name, err)
		}
		logger.Ok(host, "Stopped service %s", service.Name)
	case config.ServiceStateRestarted:
		logger.Info(host, "Restarting service %s...", service.Name)
		if _, err := runner.RunSudo("systemctl restart " + ssh.ShellQuote(service.Name)); err != nil {
			return fmt.Errorf("restarting service %s: %w", service.Name, err)
		}
		logger.Ok(host, "Restarted service %s", service.Name)
	}

	return nil
}

func serviceEnabled(service config.ServiceConfig) bool {
	return service.Enabled == nil || *service.Enabled
}

func serviceState(service config.ServiceConfig) string {
	if service.State == "" {
		if !serviceEnabled(service) {
			return config.ServiceStateStopped
		}
		return config.ServiceStateStarted
	}
	return service.State
}

func remoteFileExists(runner ssh.Runner, path string) (bool, error) {
	_, err := runner.Run("test -f " + ssh.ShellQuote(path))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func ensureSwapInFstab(runner ssh.Runner, path string) error {
	line := path + " none swap sw 0 0"
	command := "sh -lc " + ssh.ShellQuote("grep -Fxq "+ssh.ShellQuote(line)+" /etc/fstab || printf '%s\n' "+ssh.ShellQuote(line)+" >> /etc/fstab")
	if _, err := runner.RunSudo(command); err != nil {
		return fmt.Errorf("ensuring swap fstab entry: %w", err)
	}
	return nil
}

func containsLine(output string, expected string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == expected {
			return true
		}
	}
	return false
}

// detectPackageManager는 원격 서버가 지원하는 패키지 관리자를 확인한다.
func detectPackageManager(runner ssh.Runner) (string, error) {
	command := "if test -x /usr/bin/dnf || test -x /bin/dnf; then echo dnf; elif test -x /usr/bin/yum || test -x /bin/yum; then echo yum; elif test -x /usr/bin/apt-get || test -x /bin/apt-get; then echo apt; fi"
	out, err := runner.Run(command)
	if err != nil {
		return "", fmt.Errorf("detecting package manager: %w", err)
	}

	manager := parsePackageManagerOutput(out)
	if manager == "" {
		return "", fmt.Errorf("bootstrap requires yum/dnf-compatible host; apt support is not implemented yet (detected output: %q)", strings.TrimSpace(out))
	}

	return manager, nil
}

func parsePackageManagerOutput(out string) string {
	for _, field := range strings.Fields(out) {
		switch field {
		case packageManagerDNF, packageManagerYUM, "apt":
			return field
		}
	}
	return ""
}

// detectMissingPackages는 rpm 기준으로 아직 설치되지 않은 패키지만 골라낸다.
func detectMissingPackages(runner ssh.Runner, packages []string) ([]string, error) {
	missing := make([]string, 0, len(packages))
	for _, pkg := range packages {
		command := fmt.Sprintf("rpm -q %s", ssh.ShellQuote(pkg))
		if _, err := runner.Run(command); err != nil {
			missing = append(missing, pkg)
		}
	}
	return missing, nil
}

// buildDNFInstallCommand는 패키지명을 쉘 안전하게 인용해 dnf 설치 명령을 만든다.
func buildDNFInstallCommand(packages []string) string {
	return buildPackageInstallCommand(packageManagerDNF, packages)
}

func buildPackageInstallCommand(manager string, packages []string) string {
	quoted := make([]string, 0, len(packages))
	for _, pkg := range packages {
		quoted = append(quoted, ssh.ShellQuote(pkg))
	}
	return manager + " install -y " + strings.Join(quoted, " ")
}

// packagesForBootstrap은 사용자가 지정한 패키지와 JDK 설정에서 유도한 패키지를 합친다.
func packagesForBootstrap(bootstrap config.BootstrapConfig) ([]string, error) {
	packages := append([]string{}, bootstrap.Packages...)
	jdkPackage, err := resolveJDKPackage(bootstrap.JDK)
	if err != nil {
		return nil, err
	}
	if jdkPackage != "" {
		packages = configMergePackages(packages, []string{jdkPackage})
	}

	return packages, nil
}

// resolveJDKPackage는 JDK 설정을 Amazon Corretto 패키지명으로 변환한다.
func resolveJDKPackage(jdk config.JDKConfig) (string, error) {
	if jdk.Enabled != nil && !*jdk.Enabled {
		return "", nil
	}
	if jdk.Major == 0 && jdk.Vendor == "" {
		return "", nil
	}
	if jdk.Vendor != defaultJDKVendor {
		return "", fmt.Errorf("bootstrap.jdk.vendor must be %q", defaultJDKVendor)
	}
	if jdk.Major <= 0 {
		return "", fmt.Errorf("bootstrap.jdk.major must be greater than zero")
	}

	headless := true
	if jdk.Headless != nil {
		headless = *jdk.Headless
	}
	if headless {
		return fmt.Sprintf("java-%d-amazon-corretto-headless", jdk.Major), nil
	}

	return fmt.Sprintf("java-%d-amazon-corretto", jdk.Major), nil
}

// configMergePackages는 bootstrap 패키지 목록을 중복 없이 선언 순서대로 병합한다.
func configMergePackages(base []string, extras []string) []string {
	seen := make(map[string]struct{}, len(base)+len(extras))
	merged := make([]string, 0, len(base)+len(extras))
	for _, pkg := range append(append([]string{}, base...), extras...) {
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		merged = append(merged, pkg)
	}
	return merged
}
