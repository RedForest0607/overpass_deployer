package dashboard

import (
	"fmt"
	"go-deployer/internal/config"
	"go-deployer/internal/monitor"
)

func hasApplications(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	for _, server := range cfg.Servers {
		if len(server.EffectiveApps()) > 0 {
			return true
		}
	}
	return false
}

// selectedDeploymentConfig은 선택 앱과 그 배포에 필요한 SSH/디렉터리 기본값만 새 설정으로 복사한다.
// 서버 bootstrap, 공통 파일, utils, bastion 동기화는 대시보드의 앱 단위 배포 범위에서 제외한다.
func selectedDeploymentConfig(cfg *config.Config, target monitor.Target) (*config.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("deployment configuration is required")
	}
	if target.ServerIndex < 0 || target.ServerIndex >= len(cfg.Servers) {
		return nil, fmt.Errorf("server selection is out of range")
	}
	server := cfg.Servers[target.ServerIndex]
	apps := server.EffectiveApps()
	if target.AppIndex < 0 || target.AppIndex >= len(apps) {
		return nil, fmt.Errorf("application selection is out of range")
	}

	selectedServer := config.ServerConfig{
		Host:    server.Host,
		Name:    server.Name,
		Tags:    append([]string(nil), server.Tags...),
		SSHPort: server.SSHPort,
		Apps:    []config.AppConfig{cloneAppConfig(apps[target.AppIndex])},
	}
	return &config.Config{
		SSH: cfg.SSH,
		DirectoryDefaults: config.DirectoryDefaultsConfig{
			Rules: append([]config.DirectoryOwnershipRule(nil), cfg.DirectoryDefaults.Rules...),
		},
		Servers: []config.ServerConfig{selectedServer},
	}, nil
}

func cloneAppConfig(app config.AppConfig) config.AppConfig {
	cloned := app
	cloned.Tags = append([]string(nil), app.Tags...)
	cloned.Jvm.JavaOpts = append([]string(nil), app.Jvm.JavaOpts...)
	cloned.ExtraOpts = append(config.StringList(nil), app.ExtraOpts...)
	cloned.ConfigFiles = append([]config.ConfigFile(nil), app.ConfigFiles...)
	cloned.ExtraFiles = append([]config.ExtraFile(nil), app.ExtraFiles...)
	return cloned
}
