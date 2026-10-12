package vm

import (
	"fmt"
	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
	"strings"
)

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
