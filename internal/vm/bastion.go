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

	if err := syncBastionSSHConfig(client, cfg.Bastion, cfg.Servers); err != nil {
		return fmt.Errorf("sync bastion ssh config: %w", err)
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

func syncBastionSSHConfig(client ssh.Runner, bastion config.BastionConfig, servers []config.ServerConfig) error {
	block := renderBastionSSHConfigBlock(servers, bastion.AliasUser)
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

func runRemoteKnownHostRegistration(client ssh.Runner, host string, port int, logName, knownHostsPath string) error {
	command := buildKnownHostRegistrationCommand(host, port, knownHostsPath)
	if _, err := client.Run(command); err != nil {
		return fmt.Errorf("%s: %w", logName, err)
	}
	return nil
}

func renderBastionSSHConfigBlock(servers []config.ServerConfig, aliasUser string) string {
	entries := make([]config.ServerConfig, len(servers))
	copy(entries, servers)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	lines := []string{bastionAliasBlockStart}
	for _, server := range entries {
		lines = append(lines,
			"Host "+server.Name,
			"  HostName "+server.BastionTargetHost(),
			"  User "+aliasUser,
			fmt.Sprintf("  Port %d", server.BastionTargetPort()),
			"",
		)
	}
	lines = append(lines, bastionAliasBlockEnd)
	return strings.Join(lines, "\n")
}

func upsertManagedBlockCommand(targetPath, block string) string {
	dir := filepath.Dir(targetPath)
	quotedDir := shellPath(dir)
	quotedPath := shellPath(targetPath)
	quotedStart := ssh.ShellQuote(bastionAliasBlockStart)
	quotedEnd := ssh.ShellQuote(bastionAliasBlockEnd)
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
