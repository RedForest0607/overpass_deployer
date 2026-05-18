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

	logger.Ok(host, "Completed bootstrap")
	return nil
}

// bootstrapEnabled는 설치할 패키지나 OS 업데이트 작업이 실제로 있는지 판단한다.
func bootstrapEnabled(bootstrap config.BootstrapConfig, packages []string) bool {
	if len(packages) > 0 {
		return true
	}
	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		return true
	}
	if bootstrap.Timezone.Name != "" {
		return true
	}
	return bootstrap.Swap.Enabled != nil && *bootstrap.Swap.Enabled
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

	exists, err := remoteFileExists(runner, swap.Path)
	if err != nil {
		return err
	}
	if !exists {
		logger.Info(host, "Creating swap file %s (%s)...", swap.Path, swap.Size)
		if _, err := runner.RunSudo("fallocate -l " + ssh.ShellQuote(swap.Size) + " " + ssh.ShellQuote(swap.Path)); err != nil {
			return fmt.Errorf("creating swap file: %w", err)
		}
		if _, err := runner.RunSudo("chmod 600 " + ssh.ShellQuote(swap.Path)); err != nil {
			return fmt.Errorf("setting swap file permissions: %w", err)
		}
		if _, err := runner.RunSudo("mkswap " + ssh.ShellQuote(swap.Path)); err != nil {
			return fmt.Errorf("initializing swap file: %w", err)
		}
	} else {
		logger.Info(host, "Swap file %s exists; enabling without recreating...", swap.Path)
	}

	if _, err := runner.RunSudo("swapon " + ssh.ShellQuote(swap.Path)); err != nil {
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
