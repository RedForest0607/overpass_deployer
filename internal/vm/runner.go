package vm

import (
	"fmt"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

// Run executes the complete VM deployment workflow for all servers
func Run(cfg *config.Config) error {
	for _, server := range cfg.Servers {
		logger.GlobalInfo("--- Starting deployment for %s (%s) ---", server.Name, server.Host)

		if err := runSingle(cfg.SSH, server); err != nil {
			logger.Error(server.Host, "Deployment failed: %v", err)
			return err
		}

		logger.GlobalInfo("--- Completed deployment for %s (%s) ---", server.Name, server.Host)
	}
	return nil
}

func runSingle(sshCfg config.SSHConfig, server config.ServerConfig) error {
	logger.Info(server.Host, "Connecting via SSH...")
	client, err := ssh.Connect(sshCfg.User, server.Host, sshCfg.KeyPath, sshCfg.HostKeyChecking, sshCfg.KnownHosts, sshCfg.Port, sshCfg.TimeoutSec)
	if err != nil {
		return fmt.Errorf("ssh connect: %w", err)
	}
	defer client.Close()
	logger.Ok(server.Host, "SSH Connected")

	if err := CreateDirectories(client, &server.App); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	if err := DeployJar(client, &server.App); err != nil {
		return fmt.Errorf("deploy jar: %w", err)
	}

	if err := DeployConfigFiles(client, &server.App); err != nil {
		return fmt.Errorf("deploy configs: %w", err)
	}

	if err := DeployScripts(client, &server.App); err != nil {
		return fmt.Errorf("deploy scripts: %w", err)
	}

	return nil
}
