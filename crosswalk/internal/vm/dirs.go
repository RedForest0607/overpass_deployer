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
		filepath.ToSlash(filepath.Join(app.BaseDir, "lib")),
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

	missingDirectories := directories
	if !opts.DryRun {
		missingDirectories = make([]string, 0, len(directories))
		for _, dir := range directories {
			if remoteDirectoryExists(runner, dir) {
				logger.Skip(host, "Directory %s already exists", dir)
				opts.outcomes.skippedOperation()
				continue
			}
			missingDirectories = append(missingDirectories, dir)
		}
		if len(missingDirectories) == 0 {
			return ensureDirectoryOwnership(runner, ownerBase, opts)
		}
	}

	setupCmd := buildPrivilegedDirectorySetupCommandWithRules(ownerBase, opts.DirectoryRules)
	if opts.DryRun {
		logger.Info(host, "DRY-RUN: would try sudo setup command: %s", setupCmd)
	} else if _, err := runner.RunSudo(setupCmd); err != nil {
		logger.Warn(host, "Failed to run sudo mkdir, falling back to normal mkdir: %v", err)
	}

	for _, dir := range missingDirectories {
		cmd := fmt.Sprintf("mkdir -p %s", ssh.ShellQuote(dir))
		if opts.DryRun {
			logger.Info(host, "DRY-RUN: would run %s", cmd)
			continue
		}
		if _, err := runner.Run(cmd); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		opts.outcomes.changedOperation()
	}

	logger.Ok(host, "%s directories", resultLabel(opts, "created", "planned"))
	return nil
}

// ensureDirectoryOwnership checks the same managed paths as directory setup,
// including existing ancestors covered by the explicit ownership rule.
func ensureDirectoryOwnership(runner ssh.Runner, base string, opts RunOptions) error {
	rule, ok := selectDirectoryOwnershipRule(base, opts.DirectoryRules)
	if !ok {
		return nil
	}
	for _, dir := range managedDirectoriesForRule(rule.Prefix, base) {
		actual, err := runner.Run("stat -c '%U:%G' " + ssh.ShellQuote(dir))
		if err != nil {
			return fmt.Errorf("checking directory ownership %s: %w", dir, err)
		}
		if strings.TrimSpace(actual) == rule.Owner+":"+rule.Group {
			continue
		}
		if _, err := runner.RunSudo("chown " + ssh.ShellQuote(rule.Owner+":"+rule.Group) + " " + ssh.ShellQuote(dir)); err != nil {
			return fmt.Errorf("correcting directory ownership %s: %w", dir, err)
		}
		opts.outcomes.changedOperation()
	}
	return nil
}

func remoteDirectoryExists(runner ssh.Runner, path string) bool {
	_, err := runner.Run("test -d " + ssh.ShellQuote(path))
	return err == nil
}

// buildPrivilegedDirectorySetupCommand는 sudo로 기준 디렉터리 생성과 소유권 보정을 수행할 명령을 만든다.
func buildPrivilegedDirectorySetupCommand(baseDir string) string {
	return buildPrivilegedDirectorySetupCommandWithRules(baseDir, nil)
}

func buildPrivilegedDirectorySetupCommandWithRules(baseDir string, rules []config.DirectoryOwnershipRule) string {
	quotedBaseDir := ssh.ShellQuote(baseDir)
	parts := []string{
		"set -eu",
		fmt.Sprintf("base_dir=%s", quotedBaseDir),
	}

	if rule, ok := selectDirectoryOwnershipRule(baseDir, rules); ok {
		managedDirs := managedDirectoriesForRule(rule.Prefix, baseDir)
		quotedDirs := make([]string, 0, len(managedDirs))
		for _, dir := range managedDirs {
			quotedDirs = append(quotedDirs, ssh.ShellQuote(dir))
		}
		ownerSpec := ssh.ShellQuote(rule.Owner + ":" + rule.Group)
		parts = append(parts,
			fmt.Sprintf("owner_spec=%s", ownerSpec),
			`mkdir -p "${base_dir}"`,
			fmt.Sprintf(`for managed_dir in %s; do [ -d "${managed_dir}" ] && chown "${owner_spec}" "${managed_dir}"; done`, strings.Join(quotedDirs, " ")),
		)
	} else {
		parts = append(parts,
			`owner_name="${SUDO_USER:-$(id -un)}"`,
			`owner_group="$(id -gn "${owner_name}")"`,
			`if [ ! -d "${base_dir}" ]; then mkdir -p "${base_dir}" && chown "${owner_name}:${owner_group}" "${base_dir}"; fi`,
		)
	}

	innerCommand := strings.Join(parts, "; ")

	return "sh -lc " + ssh.ShellQuote(innerCommand)
}

func selectDirectoryOwnershipRule(path string, rules []config.DirectoryOwnershipRule) (config.DirectoryOwnershipRule, bool) {
	cleanPath := cleanDirectoryPath(path)
	var selected config.DirectoryOwnershipRule
	selectedLength := -1

	for _, rule := range rules {
		prefix := cleanDirectoryPath(rule.Prefix)
		if !pathMatchesPrefix(cleanPath, prefix) {
			continue
		}
		if len(prefix) > selectedLength {
			rule.Prefix = prefix
			selected = rule
			selectedLength = len(prefix)
		}
	}

	return selected, selectedLength >= 0
}

func managedDirectoriesForRule(prefix, path string) []string {
	cleanPrefix := cleanDirectoryPath(prefix)
	cleanPath := cleanDirectoryPath(path)
	if !pathMatchesPrefix(cleanPath, cleanPrefix) {
		return []string{cleanPath}
	}
	if cleanPath == cleanPrefix {
		return []string{cleanPrefix}
	}

	relativePath := strings.TrimPrefix(cleanPath, cleanPrefix+"/")
	parts := strings.Split(relativePath, "/")
	dirs := []string{cleanPrefix}
	current := cleanPrefix
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.ToSlash(filepath.Join(current, part))
		dirs = append(dirs, current)
	}
	return dirs
}

func cleanDirectoryPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

func pathMatchesPrefix(path, prefix string) bool {
	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
