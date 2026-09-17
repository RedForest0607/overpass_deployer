package docker

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

type Options struct {
	DryRun          bool
	Preflight       bool
	Pull            bool
	ServerTags      []string
	ContainerTags   []string
	ParallelServers int
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

var executeDockerServer = runServer

// Run은 dry-run, preflight, pull, full reconcile 실행 경계를 분리한다.
func Run(cfg *config.Config, opts Options) error {
	if enabledModeCount(opts) > 1 {
		return errors.New("docker mode accepts only one of --dry-run, --preflight, or --pull")
	}
	if opts.ParallelServers < 0 {
		return errors.New("docker parallel server count must be zero or greater")
	}
	filteredCfg, err := filterConfig(cfg, opts)
	if err != nil {
		return err
	}
	plan, err := BuildPlan(filteredCfg)
	if err != nil {
		return err
	}
	if opts.DryRun {
		for _, action := range plan.Actions {
			logger.Info(action.Server, "DRY-RUN [%s] %s", action.Kind, action.Summary)
		}
		summary := deploymentSummary{results: make([]serverResult, 0, len(filteredCfg.Servers))}
		for _, server := range filteredCfg.Servers {
			summary.results = append(summary.results, resultFor(server, false, nil))
		}
		summary.log()
		return nil
	}
	token, err := registryToken(filteredCfg.Docker.Registry.TokenEnv)
	if err != nil {
		return err
	}
	parallelServers := opts.ParallelServers
	if parallelServers == 0 {
		parallelServers = 1
	}
	summary := runServers(filteredCfg, token, opts, parallelServers)
	summary.log()
	return summary.err()
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

func runServers(cfg *config.Config, token string, opts Options, parallelServers int) deploymentSummary {
	if parallelServers <= 1 || len(cfg.Servers) == 1 {
		summary := deploymentSummary{results: make([]serverResult, 0, len(cfg.Servers))}
		for _, server := range cfg.Servers {
			summary.results = append(summary.results, executeServer(cfg, server, token, opts))
		}
		return summary
	}

	type job struct {
		index  int
		server config.ServerConfig
	}
	jobs := make(chan job)
	results := make(chan struct {
		index  int
		result serverResult
	}, len(cfg.Servers))
	workers := parallelServers
	if workers > len(cfg.Servers) {
		workers = len(cfg.Servers)
	}
	var waitGroup sync.WaitGroup
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for item := range jobs {
				results <- struct {
					index  int
					result serverResult
				}{item.index, executeServer(cfg, item.server, token, opts)}
			}
		}()
	}
	for index, server := range cfg.Servers {
		jobs <- job{index: index, server: server}
	}
	close(jobs)
	waitGroup.Wait()
	close(results)

	summary := deploymentSummary{results: make([]serverResult, len(cfg.Servers))}
	for item := range results {
		summary.results[item.index] = item.result
	}
	return summary
}

func executeServer(cfg *config.Config, server config.ServerConfig, token string, opts Options) serverResult {
	apply := !opts.Preflight && !opts.Pull
	changed, err := executeDockerServer(cfg, server, token, opts.Pull || apply, apply)
	if err != nil {
		err = fmt.Errorf("docker operation: %w", err)
	}
	return resultFor(server, changed, err)
}

func runServer(cfg *config.Config, server config.ServerConfig, token string, prepareImages, reconcile bool) (bool, error) {
	sshCfg := server.SSHSettings(cfg.SSH)
	runner, err := connectDockerSSH(sshCfg, server.Host)
	if err != nil {
		return false, fmt.Errorf("ssh connect: %w", err)
	}
	defer runner.Close()

	if err := verifyDockerEngine(runner); err != nil {
		return false, err
	}
	logger.Ok(server.Name, "Docker daemon preflight passed")

	if err := loginRegistry(runner, cfg.Docker.Registry, token); err != nil {
		return false, err
	}
	logger.Ok(server.Name, "Registry authentication preflight passed for %s", cfg.Docker.Registry.Host)
	if !prepareImages {
		return true, nil
	}

	changed := false
	for _, container := range server.Containers {
		pulled, err := prepareContainerImageWithOutcome(runner, server.Name, cfg.Docker.Registry, container)
		if err != nil {
			return false, fmt.Errorf("container %s: %w", container.Name, err)
		}
		changed = changed || pulled
	}
	if !reconcile {
		return changed, nil
	}
	for _, container := range server.Containers {
		reconciled, err := reconcileContainerWithOutcome(runner, server.Name, cfg.Docker.Registry, container)
		if err != nil {
			return false, fmt.Errorf("container %s: %w", container.Name, err)
		}
		changed = changed || reconciled
	}
	return changed, nil
}
