package vm

import (
	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
	"strings"
)

const packageManagerDNF = "dnf"
const packageManagerYUM = "yum"
const packageManagerAPT = "apt-get"
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

	runner = mutationTrackingRunner{Runner: runner, outcomes: opts.outcomes}
	if err := applyPackageBootstrap(runner, bootstrap, packages, host); err != nil {
		return err
	}
	if err := applyUserBootstrap(runner, bootstrap.Users, host); err != nil {
		return err
	}
	if err := applySysctlBootstrap(runner, bootstrap.Sysctls, host); err != nil {
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
	if len(bootstrap.Users) > 0 {
		return true
	}
	if len(bootstrap.Sysctls) > 0 {
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
		logger.Info(host, "DRY-RUN: would update packages through the detected OS package manager")
	}
	if len(packages) > 0 {
		logger.Info(host, "DRY-RUN: would check installed packages for %s", strings.Join(packages, ", "))
		logger.Info(host, "DRY-RUN: would install missing packages through the detected OS package manager")
	} else {
		logger.Info(host, "DRY-RUN: no bootstrap packages to install")
	}
	if bootstrap.Timezone.Name != "" {
		logger.Info(host, "DRY-RUN: would set timezone to %s", bootstrap.Timezone.Name)
	}
	for _, user := range bootstrap.Users {
		logger.Info(host, "DRY-RUN: would ensure user %s with group %s", user.Name, user.Group)
	}
	for _, sysctl := range bootstrap.Sysctls {
		logger.Info(host, "DRY-RUN: would set sysctl %s=%s and persist to %s", sysctl.Name, sysctl.Value, sysctl.PersistPath)
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

func parentDir(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func remoteFileExists(runner ssh.Runner, path string) (bool, error) {
	_, err := runner.Run("test -f " + ssh.ShellQuote(path))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func containsLine(output string, expected string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == expected {
			return true
		}
	}
	return false
}
