package vm

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

const (
	bastionAliasBlockStart = "# BEGIN overpass-deployer managed aliases"
	bastionAliasBlockEnd   = "# END overpass-deployer managed aliases"
	bastionShellBlockStart = "# BEGIN overpass-deployer managed shell aliases"
	bastionShellBlockEnd   = "# END overpass-deployer managed shell aliases"
)

func syncBastion(cfg *config.Config) error {
	return syncBastionWithOptions(cfg, RunOptions{})
}

func syncBastionWithOptions(cfg *config.Config, opts RunOptions) error {
	if !cfg.Bastion.Enabled() {
		return nil
	}

	if opts.DryRun {
		logger.GlobalInfo("--- DRY-RUN bastion sync on %s ---", cfg.Bastion.Host)
		logger.GlobalInfo("DRY-RUN: would update ssh config aliases at %s", cfg.Bastion.SSHConfigPath)
		logger.GlobalInfo("DRY-RUN: would update shell aliases at %s", cfg.Bastion.ShellAliasesPath)
		for _, server := range cfg.Servers {
			logger.GlobalInfo("DRY-RUN: would register known_hosts entry for %s (%s:%d)", server.Name, server.BastionTargetHost(), server.BastionTargetPort())
		}
		logger.GlobalInfo("--- Completed dry-run bastion sync on %s ---", cfg.Bastion.Host)
		return nil
	}

	bastionSSH := cfg.Bastion.SSHSettings(cfg.SSH)
	logger.GlobalInfo("--- Syncing bastion aliases on %s ---", cfg.Bastion.Host)

	client, err := ssh.Connect(
		bastionSSH.User,
		cfg.Bastion.Host,
		bastionSSH.KeyPath,
		bastionSSH.HostKeyChecking,
		bastionSSH.KnownHosts,
		bastionSSH.Port,
		bastionSSH.TimeoutSec,
	)
	if err != nil {
		return fmt.Errorf("connect bastion: %w", err)
	}
	defer client.Close()

	if err := syncBastionSSHConfig(client, cfg.Bastion, cfg.SSH, cfg.Servers); err != nil {
		return fmt.Errorf("sync bastion ssh config: %w", err)
	}
	if err := syncBastionShellAliases(client, cfg.Bastion, cfg.Servers); err != nil {
		return fmt.Errorf("sync bastion shell aliases: %w", err)
	}
	if err := syncBastionKnownHosts(client, cfg.Bastion.TargetKnownHosts, cfg.Servers); err != nil {
		return fmt.Errorf("sync bastion known_hosts: %w", err)
	}

	logger.GlobalInfo("--- Completed bastion sync on %s ---", cfg.Bastion.Host)
	return nil
}

func ensureServerKnowsBastion(client ssh.Runner, bastion config.BastionConfig, opts RunOptions, host string) error {
	if !bastion.Enabled() {
		return nil
	}

	port := bastion.Port
	if port == 0 {
		port = config.DefaultSSHPort
	}

	if opts.DryRun {
		logger.Info(host, "DRY-RUN: would register bastion host %s:%d in ~/.ssh/known_hosts", bastion.Host, port)
		return nil
	}

	return runRemoteKnownHostRegistration(client, bastion.Host, port, bastion.Host, "~/.ssh/known_hosts")
}

func syncBastionSSHConfig(client ssh.Runner, bastion config.BastionConfig, targetSSH config.SSHConfig, servers []config.ServerConfig) error {
	block := renderBastionSSHConfigBlock(servers, bastion.AliasUser, targetSSH.KeyPath, targetSSH.HostKeyChecking, bastion.TargetKnownHosts)
	command := upsertManagedBlockCommand(bastion.SSHConfigPath, block)
	if _, err := client.Run(command); err != nil {
		return err
	}
	return nil
}

func syncBastionKnownHosts(client ssh.Runner, knownHostsPath string, servers []config.ServerConfig) error {
	seenHosts := make(map[string]struct{}, len(servers))
	for _, server := range servers {
		hostPortKey := fmt.Sprintf("%s:%d", server.BastionTargetHost(), server.BastionTargetPort())
		if _, exists := seenHosts[hostPortKey]; exists {
			continue
		}
		seenHosts[hostPortKey] = struct{}{}

		if err := runRemoteKnownHostRegistration(client, server.BastionTargetHost(), server.BastionTargetPort(), server.Name, knownHostsPath); err != nil {
			return fmt.Errorf("%s: %w", server.BastionTargetHost(), err)
		}
	}
	return nil
}

func syncBastionShellAliases(client ssh.Runner, bastion config.BastionConfig, servers []config.ServerConfig) error {
	block := renderBastionShellAliasBlock(servers, bastion.SSHConfigPath)
	command := upsertManagedBlockCommandWithMarkers(bastion.ShellAliasesPath, block, bastionShellBlockStart, bastionShellBlockEnd)
	if _, err := client.Run(command); err != nil {
		return err
	}
	return nil
}

func runRemoteKnownHostRegistration(client ssh.Runner, host string, port int, logName, knownHostsPath string) error {
	command := buildKnownHostRegistrationCommand(host, port, knownHostsPath)
	if _, err := client.Run(command); err != nil {
		return fmt.Errorf("%s: %w", logName, err)
	}
	return nil
}

func renderBastionSSHConfigBlock(servers []config.ServerConfig, aliasUser, keyPath, hostKeyChecking, knownHostsPath string) string {
	entries := sortedServersByName(servers)
	lines := []string{bastionAliasBlockStart}
	for _, server := range entries {
		lines = append(lines,
			"Host "+server.Name,
			"  HostName "+server.BastionTargetHost(),
			"  User "+aliasUser,
			fmt.Sprintf("  Port %d", server.BastionTargetPort()),
			"  IdentityFile "+keyPath,
			"  IdentitiesOnly yes",
		)
		lines = append(lines, hostKeyConfigLines(hostKeyChecking, knownHostsPath)...)
		lines = append(lines, "")
	}
	lines = append(lines, bastionAliasBlockEnd)
	return strings.Join(lines, "\n")
}

func renderBastionShellAliasBlock(servers []config.ServerConfig, sshConfigPath string) string {
	entries := sortedServersByName(servers)
	lines := []string{bastionShellBlockStart}
	for _, server := range entries {
		lines = append(lines, fmt.Sprintf("alias %s='ssh -F %s %s'", server.Name, singleQuoteShellValue(sshConfigPath), singleQuoteShellValue(server.Name)))
	}
	lines = append(lines, bastionShellBlockEnd)
	return strings.Join(lines, "\n")
}

func upsertManagedBlockCommand(targetPath, block string) string {
	return upsertManagedBlockCommandWithMarkers(targetPath, block, bastionAliasBlockStart, bastionAliasBlockEnd)
}

func upsertManagedBlockCommandWithMarkers(targetPath, block, blockStart, blockEnd string) string {
	dir := filepath.Dir(targetPath)
	quotedDir := shellPath(dir)
	quotedPath := shellPath(targetPath)
	quotedStart := ssh.ShellQuote(blockStart)
	quotedEnd := ssh.ShellQuote(blockEnd)
	quotedBlock := ssh.ShellQuote(block)

	return fmt.Sprintf(
		"mkdir -p %s && touch %s && { [ ! -O %s ] || chmod 700 %s; } && chmod 600 %s && tmp=$(mktemp) && awk -v start=%s -v end=%s 'BEGIN {skip=0} $0 == start {skip=1; next} $0 == end {skip=0; next} skip == 0 {print}' %s > \"$tmp\" && printf '%%s\\n' %s >> \"$tmp\" && mv \"$tmp\" %s",
		quotedDir,
		quotedPath,
		quotedDir,
		quotedDir,
		quotedPath,
		quotedStart,
		quotedEnd,
		quotedPath,
		quotedBlock,
		quotedPath,
	)
}

func sortedServersByName(servers []config.ServerConfig) []config.ServerConfig {
	entries := make([]config.ServerConfig, len(servers))
	copy(entries, servers)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

func singleQuoteShellValue(value string) string {
	return strings.ReplaceAll(value, "'", `'\''`)
}

func hostKeyConfigLines(mode, knownHostsPath string) []string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case config.HostKeyInsecure:
		return []string{
			"  StrictHostKeyChecking no",
			"  UserKnownHostsFile /dev/null",
		}
	case config.HostKeyStrict:
		return []string{
			"  StrictHostKeyChecking yes",
			"  UserKnownHostsFile " + knownHostsPath,
		}
	default:
		return []string{
			"  StrictHostKeyChecking accept-new",
			"  UserKnownHostsFile " + knownHostsPath,
		}
	}
}

func buildKnownHostRegistrationCommand(host string, port int, knownHostsPath string) string {
	dir := filepath.Dir(knownHostsPath)
	quotedDir := shellPath(dir)
	quotedPath := shellPath(knownHostsPath)
	quotedHost := ssh.ShellQuote(host)

	return fmt.Sprintf(
		"mkdir -p %s && touch %s && { [ ! -O %s ] || chmod 700 %s; } && chmod 600 %s && ssh-keygen -R %s -f %s >/dev/null 2>&1 || true && ssh-keyscan -H -p %d %s >> %s",
		quotedDir,
		quotedPath,
		quotedDir,
		quotedDir,
		quotedPath,
		quotedHost,
		quotedPath,
		port,
		quotedHost,
		quotedPath,
	)
}

func shellPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		return "\"$HOME/" + escapeDoubleQuotedShellPath(path[2:]) + "\""
	}
	if path == "~" {
		return "\"$HOME\""
	}
	return ssh.ShellQuote(path)
}

func escapeDoubleQuotedShellPath(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`"`, `\"`,
		`$`, `\$`,
		"`", "\\`",
	)
	return replacer.Replace(value)
}
