package vm

import (
	"fmt"
	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
	"strings"
)

func applyUserBootstrap(runner ssh.Runner, users []config.UserConfig, host string) error {
	for _, user := range users {
		if user.Name == "" {
			continue
		}
		logger.Info(host, "Ensuring bootstrap user %s...", user.Name)
		if _, err := runner.RunSudo(buildUserEnsureCommand(user)); err != nil {
			return fmt.Errorf("ensuring bootstrap user %s: %w", user.Name, err)
		}
		logger.Ok(host, "Ensured bootstrap user %s", user.Name)
	}
	return nil
}

func buildUserEnsureCommand(user config.UserConfig) string {
	groupName := user.Group
	if groupName == "" {
		groupName = user.Name
	}

	systemGroupFlag := ""
	systemUserFlag := ""
	if user.System != nil && *user.System {
		systemGroupFlag = " -r"
		systemUserFlag = " -r"
	}

	groupIDFlag := ""
	if user.GID > 0 {
		groupIDFlag = " -g " + ssh.ShellQuote(fmt.Sprintf("%d", user.GID))
	}

	userIDFlag := ""
	if user.UID > 0 {
		userIDFlag = " -u " + ssh.ShellQuote(fmt.Sprintf("%d", user.UID))
	}

	homeFlag := ""
	if user.Home != "" {
		homeFlag = " -d " + ssh.ShellQuote(user.Home)
	}

	shellFlag := ""
	if user.Shell != "" {
		shellFlag = " -s " + ssh.ShellQuote(user.Shell)
	}

	createHomeFlag := " -M"
	if user.CreateHome != nil && *user.CreateHome {
		createHomeFlag = " -m"
	}

	innerCommand := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("group_name=%s", ssh.ShellQuote(groupName)),
		fmt.Sprintf("user_name=%s", ssh.ShellQuote(user.Name)),
		fmt.Sprintf(`if ! getent group "${group_name}" >/dev/null 2>&1; then groupadd%s%s "${group_name}"; fi`, systemGroupFlag, groupIDFlag),
		fmt.Sprintf(`if ! id -u "${user_name}" >/dev/null 2>&1; then useradd%s%s -g "${group_name}"%s%s%s "${user_name}"; fi`, systemUserFlag, userIDFlag, createHomeFlag, homeFlag, shellFlag),
	}, "; ")

	return "sh -lc " + ssh.ShellQuote(innerCommand)
}

func applySysctlBootstrap(runner ssh.Runner, sysctls []config.SysctlConfig, host string) error {
	for _, sysctl := range sysctls {
		if sysctl.Name == "" {
			continue
		}
		logger.Info(host, "Ensuring sysctl %s=%s...", sysctl.Name, sysctl.Value)
		if _, err := runner.RunSudo(buildSysctlEnsureCommand(sysctl)); err != nil {
			return fmt.Errorf("ensuring sysctl %s: %w", sysctl.Name, err)
		}
		logger.Ok(host, "Ensured sysctl %s", sysctl.Name)
	}
	return nil
}

func buildSysctlEnsureCommand(sysctl config.SysctlConfig) string {
	innerCommand := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("key=%s", ssh.ShellQuote(sysctl.Name)),
		fmt.Sprintf("value=%s", ssh.ShellQuote(sysctl.Value)),
		fmt.Sprintf("persist_path=%s", ssh.ShellQuote(sysctl.PersistPath)),
		`sysctl -w "${key}=${value}" >/dev/null`,
		`persist_dir="$(dirname "${persist_path}")"`,
		`mkdir -p "${persist_dir}"`,
		`touch "${persist_path}"`,
		`key_regex="$(printf '%s\n' "${key}" | sed 's/[][.\*^$()+?{}|]/\\&/g')"`,
		`if grep -Eq "^${key_regex}[[:space:]]*=" "${persist_path}"; then sed -i -E "s#^${key_regex}[[:space:]]*=.*#${key} = ${value}#" "${persist_path}"; else printf '%s = %s\n' "${key}" "${value}" >> "${persist_path}"; fi`,
	}, "; ")

	return "sh -lc " + ssh.ShellQuote(innerCommand)
}
