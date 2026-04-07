package vm

import (
	"fmt"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

var connectSSH = func(sshCfg config.SSHConfig, host string) (*ssh.Client, error) {
	return ssh.Connect(
		sshCfg.User,
		host,
		sshCfg.KeyPath,
		sshCfg.HostKeyChecking,
		sshCfg.KnownHosts,
		sshCfg.Port,
		sshCfg.TimeoutSec,
	)
}

// Run executes the complete VM deployment workflow for all servers
func Run(cfg *config.Config) error {
	return RunWithOptions(cfg, RunOptions{})
}

func RunWithOptions(cfg *config.Config, opts RunOptions) error {
	for _, server := range cfg.Servers {
		logger.GlobalInfo("--- Starting %s for %s (%s) ---", phaseLabel(opts), server.Name, server.Host)

		serverSSH := server.SSHSettings(cfg.SSH)
		if err := runSingle(serverSSH, cfg.Bastion, server, opts); err != nil {
			logger.Error(server.Host, "%s failed: %v", phaseTitle(opts), err)
			return err
		}

		logger.GlobalInfo("--- Completed %s for %s (%s) ---", phaseLabel(opts), server.Name, server.Host)
	}

	if err := syncBastionWithOptions(cfg, opts); err != nil {
		return err
	}

	return nil
}

func runSingle(sshCfg config.SSHConfig, bastionCfg config.BastionConfig, server config.ServerConfig, opts RunOptions) error {
	if opts.DryRun {
		logger.Info(server.Host, "DRY-RUN: would connect via SSH as %s on port %d", sshCfg.User, sshCfg.Port)
		if err := CreateDirectories(nil, &server.App, opts, server.Host); err != nil {
			return fmt.Errorf("plan directories: %w", err)
		}
		if err := DeployJar(nil, &server.App, opts, server.Host); err != nil {
			return fmt.Errorf("plan jar deployment: %w", err)
		}
		if err := DeployConfigFiles(nil, &server.App, opts, server.Host); err != nil {
			return fmt.Errorf("plan config deployment: %w", err)
		}
		if err := DeployScripts(nil, &server.App, opts, server.Host); err != nil {
			return fmt.Errorf("plan script deployment: %w", err)
		}
		if err := ensureServerKnowsBastion(nil, bastionCfg, opts, server.Host); err != nil {
			return fmt.Errorf("plan bastion host key registration: %w", err)
		}
		return nil
	}

	logger.Info(server.Host, "Connecting via SSH on port %d...", sshCfg.Port)
	client, err := connectSSH(sshCfg, server.Host)
	if err != nil {
		return fmt.Errorf("ssh connect: %w", err)
	}
	defer client.Close()
	logger.Ok(server.Host, "SSH Connected")

	if err := CreateDirectories(client, &server.App, opts, server.Host); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	if err := DeployJar(client, &server.App, opts, server.Host); err != nil {
		return fmt.Errorf("deploy jar: %w", err)
	}

	if err := DeployConfigFiles(client, &server.App, opts, server.Host); err != nil {
		return fmt.Errorf("deploy configs: %w", err)
	}

	if err := DeployScripts(client, &server.App, opts, server.Host); err != nil {
		return fmt.Errorf("deploy scripts: %w", err)
	}

	if err := ensureServerKnowsBastion(client, bastionCfg, opts, server.Host); err != nil {
		return fmt.Errorf("register bastion host key: %w", err)
	}

	return nil
}

func phaseLabel(opts RunOptions) string {
	if opts.DryRun {
		return "dry-run"
	}
	return "deployment"
}

func phaseTitle(opts RunOptions) string {
	if opts.DryRun {
		return "dry-run"
	}
	return "deployment"
}
