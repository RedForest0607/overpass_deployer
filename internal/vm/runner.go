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
	deployServerFilesStep = DeployServerExtraFiles
	createDirectoriesStep = CreateDirectories
	deployJarStep         = DeployJar
	deployConfigFilesStep = DeployConfigFiles
	deployExtraFilesStep  = DeployExtraFiles
	deployScriptsStep     = DeployScripts
)

// Run은 기본 옵션으로 모든 서버에 대한 VM 배포 워크플로를 실행한다.
func Run(cfg *config.Config) error {
	return RunWithOptions(cfg, RunOptions{})
}

// RunWithOptions는 태그 필터와 dry-run 옵션을 적용한 뒤 서버별 배포와 배스천 동기화를 수행한다.
func RunWithOptions(cfg *config.Config, opts RunOptions) error {
	filteredCfg, err := filterConfig(cfg, opts)
	if err != nil {
		return err
	}
	estimate, err := estimateConfigDeployment(filteredCfg)
	if err != nil {
		return fmt.Errorf("estimate deployment time: %w", err)
	}
	logger.GlobalInfo("--- Estimated deployment time: %s ---", formatEstimate(estimate))

	for _, server := range filteredCfg.Servers {
		logger.GlobalInfo("--- Starting %s for %s (%s) ---", phaseLabel(opts), server.Name, server.Host)
		serverEstimate, err := estimateServerDeployment(server)
		if err != nil {
			return fmt.Errorf("estimate deployment time for %s: %w", server.Name, err)
		}
		logger.Info(server.Host, "Estimated deployment time: %s", formatEstimate(serverEstimate))

		serverSSH := server.SSHSettings(filteredCfg.SSH)
		if err := runSingle(serverSSH, filteredCfg.Bastion, server, opts); err != nil {
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

// runSingle은 한 서버에 대해 bootstrap, 디렉터리 생성, 파일 전송, 스크립트 배포를 순서대로 실행한다.
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
		if len(server.ExtraFiles) > 0 {
			if err := deployServerFilesStep(nil, server.ExtraFiles, opts, server.Host); err != nil {
				return fmt.Errorf("plan server extra files: %w", err)
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
	if len(server.ExtraFiles) > 0 {
		if err := deployServerFilesStep(client, server.ExtraFiles, opts, server.Host); err != nil {
			return fmt.Errorf("deploy server extra files: %w", err)
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

// phaseLabel은 로그 헤더에 사용할 현재 실행 모드 이름을 반환한다.
func phaseLabel(opts RunOptions) string {
	if opts.DryRun {
		return "dry-run"
	}
	return "deployment"
}

// phaseTitle은 오류 로그에 사용할 현재 실행 모드 제목을 반환한다.
func phaseTitle(opts RunOptions) string {
	if opts.DryRun {
		return "dry-run"
	}
	return "deployment"
}
