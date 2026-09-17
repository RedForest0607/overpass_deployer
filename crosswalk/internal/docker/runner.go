package docker

import (
	"errors"
	"fmt"
	"io"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

var ErrApplyNotImplemented = errors.New("docker apply is not implemented yet; use --dry-run, --preflight, or --pull")

type Options struct {
	DryRun    bool
	Preflight bool
	Pull      bool
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

// Run은 dry-run, preflight, pull 단계를 명시적으로 분리한다.
// container reconcile이 완성될 때까지 일반 apply는 차단한다.
func Run(cfg *config.Config, opts Options) error {
	if enabledModeCount(opts) > 1 {
		return errors.New("docker mode accepts only one of --dry-run, --preflight, or --pull")
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
	if !opts.Preflight && !opts.Pull {
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
		if err := runServer(cfg, server, token, opts.Pull); err != nil {
			return fmt.Errorf("docker operation for %s: %w", server.Name, err)
		}
	}
	return nil
}

func enabledModeCount(opts Options) int {
	count := 0
	for _, enabled := range []bool{opts.DryRun, opts.Preflight, opts.Pull} {
		if enabled {
			count++
		}
	}
	return count
}

func runServer(cfg *config.Config, server config.ServerConfig, token string, pull bool) error {
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
	if !pull {
		return nil
	}

	for _, container := range server.Containers {
		if err := prepareContainerImage(runner, server.Name, cfg.Docker.Registry, container); err != nil {
			return fmt.Errorf("container %s: %w", container.Name, err)
		}
	}
	return nil
}
