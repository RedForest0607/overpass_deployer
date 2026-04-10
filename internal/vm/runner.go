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

var (
	bootstrapHostStep     = BootstrapHost
	createServerDirsStep  = CreateServerDirectories
	createDirectoriesStep = CreateDirectories
	deployJarStep         = DeployJar
	deployConfigFilesStep = DeployConfigFiles
	deployExtraFilesStep  = DeployExtraFiles
	deployScriptsStep     = DeployScripts
)

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
	apps := server.EffectiveApps()
	if opts.DryRun {
		logger.Info(server.Host, "DRY-RUN: would connect via SSH as %s on port %d", sshCfg.User, sshCfg.Port)
		if err := bootstrapHostStep(nil, server.Bootstrap, opts, server.Host); err != nil {
			return fmt.Errorf("plan bootstrap: %w", err)
		}
		if len(server.Directories) > 0 {
			if err := createServerDirsStep(nil, server.Directories, opts, server.Host); err != nil {
				return fmt.Errorf("plan server directories: %w", err)
			}
		}
		for i := range apps {
			app := &apps[i]
			logger.Info(server.Host, "DRY-RUN: would deploy app %s", app.Name)
			if err := createDirectoriesStep(nil, app, opts, server.Host); err != nil {
				return fmt.Errorf("plan directories for app %s: %w", app.Name, err)
			}
			if err := deployJarStep(nil, app, opts, server.Host); err != nil {
				return fmt.Errorf("plan jar deployment for app %s: %w", app.Name, err)
			}
			if err := deployConfigFilesStep(nil, app, opts, server.Host); err != nil {
				return fmt.Errorf("plan config deployment for app %s: %w", app.Name, err)
			}
			if err := deployExtraFilesStep(nil, app, opts, server.Host); err != nil {
				return fmt.Errorf("plan extra file deployment for app %s: %w", app.Name, err)
			}
			if err := deployScriptsStep(nil, app, opts, server.Host); err != nil {
				return fmt.Errorf("plan script deployment for app %s: %w", app.Name, err)
			}
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

	if err := bootstrapHostStep(client, server.Bootstrap, opts, server.Host); err != nil {
		return fmt.Errorf("bootstrap host: %w", err)
	}
	if len(server.Directories) > 0 {
		if err := createServerDirsStep(client, server.Directories, opts, server.Host); err != nil {
			return fmt.Errorf("create server directories: %w", err)
		}
	}

	for i := range apps {
		app := &apps[i]
		logger.Info(server.Host, "Deploying app %s...", app.Name)

		if err := createDirectoriesStep(client, app, opts, server.Host); err != nil {
			return fmt.Errorf("create directories for app %s: %w", app.Name, err)
		}

		if err := deployJarStep(client, app, opts, server.Host); err != nil {
			return fmt.Errorf("deploy jar for app %s: %w", app.Name, err)
		}

		if err := deployConfigFilesStep(client, app, opts, server.Host); err != nil {
			return fmt.Errorf("deploy configs for app %s: %w", app.Name, err)
		}
		if err := deployExtraFilesStep(client, app, opts, server.Host); err != nil {
			return fmt.Errorf("deploy extra files for app %s: %w", app.Name, err)
		}

		if err := deployScriptsStep(client, app, opts, server.Host); err != nil {
			return fmt.Errorf("deploy scripts for app %s: %w", app.Name, err)
		}
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
