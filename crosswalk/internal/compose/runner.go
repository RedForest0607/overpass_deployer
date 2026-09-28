package compose

import (
	"errors"
	"fmt"
	"sync"

	"go-deployer/internal/config"
	"go-deployer/internal/docker"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

type Options struct {
	DryRun          bool
	Preflight       bool
	Sync            bool
	Apply           bool
	ServerTags      []string
	Projects        []string
	ParallelServers int
}

var connectComposeSSH = func(sshCfg config.SSHConfig, host string) (docker.RemoteRunner, error) {
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

func Run(cfg *config.Config, options Options) error {
	if enabledModeCount(options) > 1 {
		return errors.New("compose mode accepts only one of --dry-run, --preflight, --sync, or --apply")
	}
	if options.ParallelServers < 0 {
		return errors.New("compose parallel server count must be zero or greater")
	}
	filtered, err := filterConfig(cfg, options)
	if err != nil {
		return err
	}
	plan, err := BuildPlan(filtered)
	if err != nil {
		return err
	}
	if options.DryRun {
		logDryRun(plan)
		return nil
	}
	if !options.Preflight && !options.Sync && !options.Apply {
		return errors.New("compose workload changes require explicit --apply")
	}
	if options.Apply {
		for _, server := range filtered.Servers {
			for _, project := range server.ComposeProjects {
				if _, err := readDesiredCompose(project.LocalPath, filtered.Docker.Registry.Host); err != nil {
					return fmt.Errorf("%s/%s: %w", server.Name, project.Name, err)
				}
			}
		}
	}

	token, err := docker.RegistryToken(filtered.Docker.Registry.TokenEnv)
	if err != nil {
		return err
	}
	summary := runServers(filtered, token, options)
	summary.log()
	return summary.err()
}

func enabledModeCount(options Options) int {
	count := 0
	for _, enabled := range []bool{options.DryRun, options.Preflight, options.Sync, options.Apply} {
		if enabled {
			count++
		}
	}
	return count
}

func logDryRun(plan Plan) {
	for _, action := range plan.Actions {
		scope := action.Server
		if action.Project != "" {
			scope += "/" + action.Project
		}
		logger.Info(scope, "DRY-RUN [%s] %s", action.Kind, action.Summary)
	}
}

func runServers(cfg *config.Config, token string, options Options) deploymentSummary {
	summary := deploymentSummary{results: make([]serverResult, len(cfg.Servers))}
	execute := func(index int, server config.ServerConfig) {
		changed, err := runServer(cfg, server, token, options)
		if err != nil {
			err = fmt.Errorf("compose operation: %w", err)
		}
		summary.results[index] = resultFor(server, options.Preflight || changed, err)
	}
	workers := options.ParallelServers
	if workers == 0 {
		workers = 1
	}
	if workers == 1 || len(cfg.Servers) == 1 {
		for index, server := range cfg.Servers {
			execute(index, server)
		}
		return summary
	}
	jobs := make(chan int)
	var group sync.WaitGroup
	for range min(workers, len(cfg.Servers)) {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				execute(index, cfg.Servers[index])
			}
		}()
	}
	for index := range cfg.Servers {
		jobs <- index
	}
	close(jobs)
	group.Wait()
	return summary
}

func runServer(cfg *config.Config, server config.ServerConfig, token string, options Options) (bool, error) {
	sshCfg := server.SSHSettings(cfg.SSH)
	runner, err := connectComposeSSH(sshCfg, server.Host)
	if err != nil {
		return false, fmt.Errorf("ssh connect: %w", err)
	}
	defer runner.Close()

	if err := docker.VerifyEngine(runner); err != nil {
		return false, err
	}
	logger.Ok(server.Name, "Docker daemon preflight passed")
	if err := verifyComposePlugin(runner); err != nil {
		return false, err
	}
	logger.Ok(server.Name, "Docker Compose v2 preflight passed")
	if err := docker.LoginRegistry(runner, cfg.Docker.Registry, token); err != nil {
		return false, err
	}
	logger.Ok(server.Name, "Registry authentication preflight passed for %s", cfg.Docker.Registry.Host)
	if options.Preflight {
		return false, nil
	}
	if options.Apply {
		changed := false
		for _, project := range server.ComposeProjects {
			desired, err := readDesiredCompose(project.LocalPath, cfg.Docker.Registry.Host)
			if err != nil {
				return changed, fmt.Errorf("project %s: %w", project.Name, err)
			}
			applied, err := applyProject(runner, project, desired)
			if err != nil {
				return changed, fmt.Errorf("project %s: %w", project.Name, err)
			}
			changed = changed || applied
		}
		return changed, nil
	}

	changed := false
	for _, project := range server.ComposeProjects {
		remoteFile, fileChanged, err := syncComposeFile(runner, project)
		if err != nil {
			return false, fmt.Errorf("project %s: %w", project.Name, err)
		}
		changed = changed || fileChanged
		if err := validateComposeProject(runner, project.Name, remoteFile); err != nil {
			return false, fmt.Errorf("project %s: %w", project.Name, err)
		}
		logger.Ok(server.Name+"/"+project.Name, "Compose file synchronized and validated")
	}
	return changed, nil
}
