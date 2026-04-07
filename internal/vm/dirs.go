package vm

import (
	"fmt"
	"path/filepath"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

func CreateDirectories(runner ssh.Runner, app *config.AppConfig, opts RunOptions, host string) error {
	host = runnerHost(runner, host)
	logger.Info(host, "%s directories in %s...", actionLabel(opts, "creating"), app.BaseDir)

	setupCmd := buildPrivilegedDirectorySetupCommand(app.BaseDir)
	if opts.DryRun {
		logger.Info(host, "DRY-RUN: would try sudo setup command: %s", setupCmd)
	} else if _, err := runner.RunSudo(setupCmd); err != nil {
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
		if opts.DryRun {
			logger.Info(host, "DRY-RUN: would run %s", cmd)
			continue
		}
		if _, err := runner.Run(cmd); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	logger.Ok(host, "%s directories", resultLabel(opts, "created", "planned"))
	return nil
}

func buildPrivilegedDirectorySetupCommand(baseDir string) string {
	quotedBaseDir := ssh.ShellQuote(baseDir)
	innerCommand := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("base_dir=%s", quotedBaseDir),
		`owner_name="${SUDO_USER:-$(id -un)}"`,
		`owner_group="$(id -gn "${owner_name}")"`,
		`mkdir -p "${base_dir}"`,
		`chown -R "${owner_name}:${owner_group}" "${base_dir}"`,
	}, "; ")

	return "sh -lc " + ssh.ShellQuote(innerCommand)
}
