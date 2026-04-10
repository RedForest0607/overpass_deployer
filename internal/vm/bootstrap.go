package vm

import (
	"fmt"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

const packageManagerDNF = "dnf"
const defaultJDKVendor = "corretto"

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

	manager, err := detectPackageManager(runner)
	if err != nil {
		return err
	}
	if manager != packageManagerDNF {
		return fmt.Errorf("bootstrap requires dnf-compatible host; apt support is not implemented yet")
	}

	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		logger.Info(host, "Running bootstrap OS update via dnf...")
		if _, err := runner.RunSudo("dnf update -y"); err != nil {
			return fmt.Errorf("running bootstrap os update: %w", err)
		}
	}

	missingPackages, err := detectMissingPackages(runner, packages)
	if err != nil {
		return err
	}
	if len(missingPackages) == 0 {
		logger.Skip(host, "Bootstrap packages already installed")
		return nil
	}

	installCommand := buildDNFInstallCommand(missingPackages)
	logger.Info(host, "Installing bootstrap packages: %s", strings.Join(missingPackages, ", "))
	if _, err := runner.RunSudo(installCommand); err != nil {
		return fmt.Errorf("installing bootstrap packages: %w", err)
	}

	logger.Ok(host, "Installed bootstrap packages")
	return nil
}

func bootstrapEnabled(bootstrap config.BootstrapConfig, packages []string) bool {
	if len(packages) > 0 {
		return true
	}
	return bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled
}

func logBootstrapPlan(host string, bootstrap config.BootstrapConfig, packages []string) {
	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		logger.Info(host, "DRY-RUN: would run sudo dnf update -y")
	}
	if len(packages) == 0 {
		logger.Info(host, "DRY-RUN: no bootstrap packages to install")
		return
	}
	logger.Info(host, "DRY-RUN: would check installed packages via rpm -q for %s", strings.Join(packages, ", "))
	logger.Info(host, "DRY-RUN: would run sudo %s", buildDNFInstallCommand(packages))
}

func detectPackageManager(runner ssh.Runner) (string, error) {
	command := "sh -lc " + ssh.ShellQuote("if command -v dnf >/dev/null 2>&1; then echo dnf; elif command -v apt-get >/dev/null 2>&1; then echo apt; fi")
	out, err := runner.Run(command)
	if err != nil {
		return "", fmt.Errorf("detecting package manager: %w", err)
	}

	manager := strings.TrimSpace(out)
	if manager == "" {
		return "", fmt.Errorf("bootstrap requires dnf-compatible host; apt support is not implemented yet")
	}

	return manager, nil
}

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

func buildDNFInstallCommand(packages []string) string {
	quoted := make([]string, 0, len(packages))
	for _, pkg := range packages {
		quoted = append(quoted, ssh.ShellQuote(pkg))
	}
	return "dnf install -y " + strings.Join(quoted, " ")
}

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
