package vm

import (
	"fmt"
	"path/filepath"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

// CreateDirectories는 앱 실행에 필요한 표준 하위 디렉터리들을 생성한다.
func CreateDirectories(runner ssh.Runner, app *config.AppConfig, opts RunOptions, host string) error {
	dirs := []string{
		filepath.ToSlash(filepath.Join(app.BaseDir, "bin")),
		filepath.ToSlash(filepath.Join(app.BaseDir, "config")),
		filepath.ToSlash(filepath.Join(app.BaseDir, "logs")),
	}

	return createDirectoryPaths(runner, dirs, opts, host, app.BaseDir)
}

// CreateServerDirectories는 앱과 무관한 서버 레벨 디렉터리 목록을 생성한다.
func CreateServerDirectories(runner ssh.Runner, directories []string, opts RunOptions, host string) error {
	host = runnerHost(runner, host)
	if len(directories) == 0 {
		logger.Skip(host, "No server directories configured")
		return nil
	}

	logger.Info(host, "%s server directories...", actionLabel(opts, "creating"))
	for _, dir := range directories {
		if err := createDirectoryPaths(runner, []string{dir}, opts, host, dir); err != nil {
			return err
		}
	}
	logger.Ok(host, "%s server directories", resultLabel(opts, "created", "planned"))
	return nil
}

// createDirectoryPaths는 권한 보정이 필요한 기준 디렉터리를 준비한 뒤 실제 디렉터리를 만든다.
func createDirectoryPaths(runner ssh.Runner, directories []string, opts RunOptions, host string, ownerBase string) error {
	host = runnerHost(runner, host)
	logger.Info(host, "%s directories in %s...", actionLabel(opts, "creating"), ownerBase)

	setupCmd := buildPrivilegedDirectorySetupCommand(ownerBase)
	if opts.DryRun {
		logger.Info(host, "DRY-RUN: would try sudo setup command: %s", setupCmd)
	} else if _, err := runner.RunSudo(setupCmd); err != nil {
		logger.Warn(host, "Failed to run sudo mkdir, falling back to normal mkdir: %v", err)
	}

	for _, dir := range directories {
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

// buildPrivilegedDirectorySetupCommand는 sudo로 기준 디렉터리 생성과 소유권 보정을 수행할 명령을 만든다.
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
