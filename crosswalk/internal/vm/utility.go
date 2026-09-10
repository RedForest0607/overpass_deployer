package vm

import (
	"fmt"

	"go-deployer/internal/config"
	"go-deployer/internal/ssh"
	"go-deployer/pkg/logger"
)

// DeployUtilities는 유틸리티별 디렉터리와 파일을 준비한 뒤 선언된 systemd 서비스 상태를 적용한다.
func DeployUtilities(client *ssh.Client, utilities []config.UtilityConfig, opts RunOptions, host string) error {
	for _, utility := range utilities {
		logger.Info(host, "%s utility %s...", actionLabel(opts, "deploying"), utility.Name)

		if len(utility.Directories) > 0 {
			if err := CreateServerDirectories(client, utility.Directories, opts, host); err != nil {
				return fmt.Errorf("create directories: %w", err)
			}
		}
		if len(utility.Files) > 0 {
			if err := DeployServerExtraFiles(client, utility.Files, opts, host); err != nil {
				return fmt.Errorf("deploy files: %w", err)
			}
		}
		if len(utility.Services) == 0 {
			logger.Ok(host, "%s utility %s", resultLabel(opts, "deployed", "planned"), utility.Name)
			continue
		}
		if opts.DryRun {
			for _, service := range utility.Services {
				logger.Info(host, "DRY-RUN: would ensure utility service %s is %s and enabled=%t", service.Name, serviceState(service), serviceEnabled(service))
			}
			logger.Ok(host, "Planned utility %s", utility.Name)
			continue
		}
		if err := applyServiceBootstrap(mutationTrackingRunner{Runner: client, outcomes: opts.outcomes}, utility.Services, host); err != nil {
			return fmt.Errorf("apply services: %w", err)
		}

		logger.Ok(host, "Deployed utility %s", utility.Name)
	}
	return nil
}
