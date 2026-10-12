package vm

import (
	"fmt"
	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
	"strings"
)

func applyPackageBootstrap(runner ssh.Runner, bootstrap config.BootstrapConfig, packages []string, host string) error {
	if len(packages) == 0 && (bootstrap.OSUpdate.Enabled == nil || !*bootstrap.OSUpdate.Enabled) {
		return nil
	}

	manager, err := detectPackageManager(runner)
	if err != nil {
		return err
	}
	packages, err = packagesForPackageManager(bootstrap, manager)
	if err != nil {
		return err
	}

	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		logger.Info(host, "Running bootstrap OS update via %s...", manager)
		if _, err := runner.RunSudo(buildPackageUpdateCommand(manager)); err != nil {
			return fmt.Errorf("running bootstrap os update: %w", err)
		}
	}
	if len(packages) == 0 {
		return nil
	}

	missingPackages, err := detectMissingPackages(runner, manager, packages)
	if err != nil {
		return err
	}
	if len(missingPackages) == 0 {
		logger.Skip(host, "Bootstrap packages already installed")
		return nil
	}

	if manager == packageManagerAPT {
		logger.Info(host, "Refreshing apt package index before installation...")
		if _, err := runner.RunSudo(buildPackageUpdateCommand(manager)); err != nil {
			return fmt.Errorf("refreshing apt package index: %w", err)
		}
	}

	installCommand := buildPackageInstallCommand(manager, missingPackages)
	logger.Info(host, "Installing bootstrap packages: %s", strings.Join(missingPackages, ", "))
	if _, err := runner.RunSudo(installCommand); err != nil {
		return fmt.Errorf("installing bootstrap packages: %w", err)
	}

	logger.Ok(host, "Installed bootstrap packages")
	return nil
}

// detectPackageManager는 원격 서버가 지원하는 패키지 관리자를 확인한다.
func detectPackageManager(runner ssh.Runner) (string, error) {
	checks := []struct {
		manager string
		command string
	}{
		{manager: packageManagerDNF, command: "test -x /usr/bin/dnf || test -x /bin/dnf"},
		{manager: packageManagerYUM, command: "test -x /usr/bin/yum || test -x /bin/yum"},
		{manager: packageManagerAPT, command: "test -x /usr/bin/apt-get || test -x /bin/apt-get"},
	}

	var failures []string
	for _, check := range checks {
		out, err := runner.Run(check.command)
		if err == nil {
			return check.manager, nil
		}
		if trimmed := strings.TrimSpace(out); trimmed != "" {
			failures = append(failures, fmt.Sprintf("%s output: %s", check.manager, trimmed))
		}
	}

	if len(failures) > 0 {
		return "", fmt.Errorf("bootstrap requires a supported package manager (dnf, yum, or apt-get): %s", strings.Join(failures, "; "))
	}
	return "", fmt.Errorf("bootstrap requires a supported package manager (dnf, yum, or apt-get)")
}

// detectMissingPackages는 OS 패키지 관리자에 맞춰 아직 설치되지 않은 패키지만 골라낸다.
func detectMissingPackages(runner ssh.Runner, manager string, packages []string) ([]string, error) {
	missing := make([]string, 0, len(packages))
	for _, pkg := range packages {
		command, err := packageInstalledCommand(manager, pkg)
		if err != nil {
			return nil, err
		}
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

func buildPackageUpdateCommand(manager string) string {
	if manager == packageManagerAPT {
		return manager + " update"
	}
	return manager + " update -y"
}

func packageInstalledCommand(manager, pkg string) (string, error) {
	switch manager {
	case packageManagerDNF, packageManagerYUM:
		return "rpm -q " + ssh.ShellQuote(pkg), nil
	case packageManagerAPT:
		return "dpkg-query -W -f='${db:Status-Status}' " + ssh.ShellQuote(pkg) + " | grep -Fxq installed", nil
	default:
		return "", fmt.Errorf("unsupported package manager %q", manager)
	}
}

// packagesForBootstrap은 사용자가 지정한 패키지와 JDK 설정에서 유도한 패키지를 합친다.
func packagesForBootstrap(bootstrap config.BootstrapConfig) ([]string, error) {
	return packagesForPackageManager(bootstrap, packageManagerDNF)
}

// packagesForPackageManager는 OS별 기본 JDK 패키지명까지 포함한 설치 대상 목록을 만든다.
func packagesForPackageManager(bootstrap config.BootstrapConfig, manager string) ([]string, error) {
	packages := append([]string{}, bootstrap.Packages...)
	jdkPackage, err := resolveJDKPackageForManager(manager, bootstrap.JDK)
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
	return resolveJDKPackageForManager(packageManagerDNF, jdk)
}

// resolveJDKPackageForManager는 패키지 관리자가 제공하는 JDK 패키지명으로 변환한다.
// Ubuntu/Debian 계열은 배포판 표준 OpenJDK 패키지를 사용하고, RPM 계열은 Amazon Corretto 패키지를 사용한다.
func resolveJDKPackageForManager(manager string, jdk config.JDKConfig) (string, error) {
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
	if manager == packageManagerAPT {
		if headless {
			return fmt.Sprintf("openjdk-%d-jdk-headless", jdk.Major), nil
		}
		return fmt.Sprintf("openjdk-%d-jdk", jdk.Major), nil
	}
	if manager != packageManagerDNF && manager != packageManagerYUM {
		return "", fmt.Errorf("unsupported package manager %q", manager)
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
