package vm

import (
	"fmt"
	"path/filepath"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

func CreateDirectories(runner ssh.Runner, app *config.AppConfig) error {
	host := runner.Host()
	logger.Info(host, "Creating directories in %s...", app.BaseDir)

	baseDir := ssh.ShellQuote(app.BaseDir)
	setupCmd := fmt.Sprintf("mkdir -p %s && chown -R $(whoami) %s", baseDir, baseDir)
	if _, err := runner.RunSudo(setupCmd); err != nil {
		logger.Warn(host, "Failed to run sudo mkdir, falling back to normal mkdir: %v", err)
	}

	dirs := []string{
		filepath.ToSlash(filepath.Join(app.BaseDir, "bin")),
		filepath.ToSlash(filepath.Join(app.BaseDir, "config")),
		filepath.ToSlash(filepath.Join(app.BaseDir, "scripts")),
		filepath.ToSlash(filepath.Join(app.BaseDir, "logs")),
		filepath.ToSlash(filepath.Join(app.BaseDir, "run")),
	}

	for _, dir := range dirs {
		cmd := fmt.Sprintf("mkdir -p %s", ssh.ShellQuote(dir))
		if _, err := runner.Run(cmd); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	logger.Ok(host, "Created directories")
	return nil
}
