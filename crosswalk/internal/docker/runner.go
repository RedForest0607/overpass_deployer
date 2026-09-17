package docker

import (
	"errors"
	"fmt"
	"io"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

var ErrApplyNotImplemented = errors.New("docker apply is not implemented yet; use --dry-run or --preflight")

type Options struct {
	DryRun    bool
	Preflight bool
}

type remoteRunner interface {
	Run(command string) (string, error)
	RunWithStdin(command string, input io.Reader) (string, error)
	Close() error
}

var connectDockerSSH = func(sshCfg config.SSHConfig, host string) (remoteRunner, error) {
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

// Run은 dry-run과 명시적 preflight만 실행한다. pull/reconcile이 완성될 때까지 일반 apply는 차단한다.
func Run(cfg *config.Config, opts Options) error {
	if opts.DryRun && opts.Preflight {
		return errors.New("docker mode accepts only one of --dry-run or --preflight")
	}
	plan, err := BuildPlan(cfg)
	if err != nil {
		return err
	}
	if opts.DryRun {
		for _, action := range plan.Actions {
			logger.Info(action.Server, "DRY-RUN [%s] %s", action.Kind, action.Summary)
		}
		return nil
	}
	if !opts.Preflight {
		return ErrApplyNotImplemented
	}

	token, err := registryToken(cfg.Docker.Registry.TokenEnv)
	if err != nil {
		return err
	}
	for _, server := range cfg.Servers {
		if len(server.Containers) == 0 {
			continue
		}
		if err := preflightServer(cfg, server, token); err != nil {
			return fmt.Errorf("docker preflight for %s: %w", server.Name, err)
		}
	}
	return nil
}

func preflightServer(cfg *config.Config, server config.ServerConfig, token string) error {
	sshCfg := server.SSHSettings(cfg.SSH)
	runner, err := connectDockerSSH(sshCfg, server.Host)
	if err != nil {
		return fmt.Errorf("ssh connect: %w", err)
	}
	defer runner.Close()

	if err := verifyDockerEngine(runner); err != nil {
		return err
	}
	logger.Ok(server.Name, "Docker daemon preflight passed")

	if err := loginRegistry(runner, cfg.Docker.Registry, token); err != nil {
		return err
	}
	logger.Ok(server.Name, "Registry authentication preflight passed for %s", cfg.Docker.Registry.Host)
	return nil
}
