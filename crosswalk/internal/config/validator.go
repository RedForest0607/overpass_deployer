package config

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	ChartFormatLegacy             = "crosswalk/legacy"
	ChartFormatV1                 = "crosswalk/v1"
	DefaultSSHTimeout             = 30
	DefaultJvmMin                 = "256m"
	DefaultJvmMax                 = "1g"
	DefaultSSHPort                = 22
	DefaultJDKVendor              = "corretto"
	DefaultDockerHealthTimeoutSec = 60
	ServiceStateStarted           = "started"
	ServiceStateStopped           = "stopped"
	ServiceStateRestarted         = "restarted"
	HostKeyStrict                 = "strict"
	HostKeyAcceptNew              = "accept-new"
	HostKeyInsecure               = "insecure"
)

// ValidateAndApplyDefaults는 전체 설정의 필수값, 경로, 포트, 태그를 검증하고 런타임 기본값을 채운다.
func ValidateAndApplyDefaults(cfg *Config) error {
	var errs []string
	aliasNames := make(map[string]int, len(cfg.Servers))
	validateChartFormat(cfg, &errs)

	checkUnresolvedEnv := func(val string, fieldName string) {
		if strings.Contains(val, "${") && strings.Contains(val, "}") {
			errs = append(errs, fmt.Sprintf("%s contains unresolved environment variable: %s", fieldName, val))
		}
	}

	validateSSHConfig(&cfg.SSH, &errs, checkUnresolvedEnv)

	// 1.1 Bastion Config
	validateBastionConfig(cfg, &errs, checkUnresolvedEnv)
	validateBootstrapConfig(&cfg.Bootstrap, "bootstrap", &errs, checkUnresolvedEnv)
	validateDirectoryDefaultsConfig(&cfg.DirectoryDefaults, "directory_defaults", &errs, checkUnresolvedEnv)
	validateDockerDeploymentConfig(cfg, &errs, checkUnresolvedEnv)

	// 2. Server Configs
	if len(cfg.Servers) == 0 {
		errs = append(errs, "at least one server must be specified")
	}

	for i, s := range cfg.Servers {
		prefix := fmt.Sprintf("servers[%d]", i)
		if s.Host == "" {
			errs = append(errs, prefix+".host is required")
		}
		validateTags(&cfg.Servers[i].Tags, prefix+".tags", &errs, checkUnresolvedEnv)
		if s.SSHPort == 0 {
			cfg.Servers[i].SSHPort = cfg.SSH.Port
		} else {
			validatePort(s.SSHPort, prefix+".ssh_port", &errs)
		}
		checkUnresolvedEnv(s.BastionHost, prefix+".bastion_host")
		if s.BastionSSHPort == 0 {
			cfg.Servers[i].BastionSSHPort = cfg.Servers[i].SSHPort
		} else {
			validatePort(s.BastionSSHPort, prefix+".bastion_ssh_port", &errs)
		}
		validateBootstrapConfig(&cfg.Servers[i].Bootstrap, prefix+".bootstrap", &errs, checkUnresolvedEnv)
		validateDirectories(&cfg.Servers[i].Directories, prefix+".directories", &errs, checkUnresolvedEnv)
		prodLikeServer := hasProdLikeTags(cfg.Servers[i].Tags)
		validateExtraFiles(&cfg.Servers[i].ExtraFiles, prefix+".extra_files", &errs, checkUnresolvedEnv, prodLikeServer)
		validateUtilityConfigs(cfg.Servers[i].Utils, prefix+".utils", &errs, checkUnresolvedEnv, prodLikeServer)
		validateContainerConfigs(cfg.Servers[i].Containers, prefix+".containers", &errs, checkUnresolvedEnv)
		validateComposeProjects(cfg.Servers[i].ComposeProjects, prefix+".compose_projects", &errs, checkUnresolvedEnv)

		if s.UsesLegacyApp() && len(s.Apps) > 0 {
			errs = append(errs, prefix+" cannot define both app and apps")
		}
		if !s.UsesLegacyApp() && len(s.Apps) == 0 && len(cfg.Servers[i].Containers) == 0 && len(cfg.Servers[i].ComposeProjects) == 0 && len(cfg.Servers[i].Directories) == 0 && len(cfg.Servers[i].ExtraFiles) == 0 && len(cfg.Servers[i].Utils) == 0 && !hasBootstrapSettings(cfg.Servers[i].Bootstrap) {
			errs = append(errs, prefix+" must define app, apps, containers, compose_projects, directories, extra_files, utils, or bootstrap settings")
		}

		if s.UsesLegacyApp() {
			validateAppConfig(&cfg.Servers[i].App, prefix+".app", &errs, checkUnresolvedEnv, prodLikeServer)
		}
		for j := range cfg.Servers[i].Apps {
			validateAppConfig(&cfg.Servers[i].Apps[j], fmt.Sprintf("%s.apps[%d]", prefix, j), &errs, checkUnresolvedEnv, prodLikeServer)
		}

		// If Name is not given, default it to Host to help logging
		if s.Name == "" {
			cfg.Servers[i].Name = s.Host
		}
		if !isValidBastionAlias(cfg.Servers[i].Name) {
			errs = append(errs, prefix+".name must contain only letters, numbers, dots, hyphens, or underscores")
		}
		aliasNames[cfg.Servers[i].Name]++
		cfg.Servers[i].Bootstrap = cfg.Servers[i].EffectiveBootstrap(cfg.Bootstrap)
		applyBootstrapDefaults(&cfg.Servers[i].Bootstrap)
	}

	for alias, count := range aliasNames {
		if count > 1 {
			errs = append(errs, fmt.Sprintf("server name %q must be unique", alias))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n- %s", strings.Join(errs, "\n- "))
	}

	return nil
}

// ValidateForMonitoring은 대시보드가 원격 상태와 로그를 조회하는 데 필요한 설정만 검증한다.
// 이 검증 경로에서는 JAR와 설정 파일 같은 배포 원본의 로컬 존재 여부를 요구하지 않는다.
func ValidateForMonitoring(cfg *Config) error {
	var errs []string
	validateChartFormat(cfg, &errs)
	checkUnresolvedEnv := func(value, fieldName string) {
		if strings.Contains(value, "${") && strings.Contains(value, "}") {
			errs = append(errs, fmt.Sprintf("%s contains unresolved environment variable: %s", fieldName, value))
		}
	}

	validateSSHConfig(&cfg.SSH, &errs, checkUnresolvedEnv)

	if len(cfg.Servers) == 0 {
		errs = append(errs, "at least one server must be specified")
	}
	serverNames := make(map[string]struct{}, len(cfg.Servers))
	for serverIndex := range cfg.Servers {
		server := &cfg.Servers[serverIndex]
		prefix := fmt.Sprintf("servers[%d]", serverIndex)
		server.Host = strings.TrimSpace(server.Host)
		server.Name = strings.TrimSpace(server.Name)
		if server.Host == "" {
			errs = append(errs, prefix+".host is required")
		}
		if server.Name == "" {
			server.Name = server.Host
		}
		if !isValidBastionAlias(server.Name) {
			errs = append(errs, prefix+".name must contain only letters, numbers, dots, hyphens, or underscores")
		}
		if _, exists := serverNames[server.Name]; exists {
			errs = append(errs, fmt.Sprintf("server name %q must be unique", server.Name))
		}
		serverNames[server.Name] = struct{}{}
		if server.SSHPort == 0 {
			server.SSHPort = cfg.SSH.Port
		} else {
			validatePort(server.SSHPort, prefix+".ssh_port", &errs)
		}
		if server.UsesLegacyApp() && len(server.Apps) > 0 {
			errs = append(errs, prefix+" cannot define both app and apps")
		}
		if server.UsesLegacyApp() {
			validateMonitoringApp(&server.App, prefix+".app", &errs, checkUnresolvedEnv)
		}
		for appIndex := range server.Apps {
			validateMonitoringApp(&server.Apps[appIndex], fmt.Sprintf("%s.apps[%d]", prefix, appIndex), &errs, checkUnresolvedEnv)
		}
		if len(server.EffectiveApps()) == 0 {
			errs = append(errs, prefix+" must define app or apps for monitoring")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n- %s", strings.Join(errs, "\n- "))
	}
	return nil
}

func validateChartFormat(cfg *Config, errs *[]string) {
	cfg.Format = strings.ToLower(strings.TrimSpace(cfg.Format))
	if cfg.Format == "" {
		cfg.Format = ChartFormatLegacy
		return
	}
	if cfg.Format != ChartFormatV1 && cfg.Format != ChartFormatLegacy {
		*errs = append(*errs, fmt.Sprintf("format must be %q or %q", ChartFormatV1, ChartFormatLegacy))
	}
}

var bastionAliasPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var fileModePattern = regexp.MustCompile(`^[0-7]{3,4}$`)
var sysctlNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var containerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var composeProjectNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
var composeRemoteDirPattern = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
var dockerRegistryHostPattern = regexp.MustCompile(`^[a-z0-9.-]+$`)
var dockerRegistryUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_.@+$-]+$`)
var dockerRepositoryPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)
var dockerDigestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var allowedRestartPolicies = map[string]bool{
	"no":             true,
	"always":         true,
	"on-failure":     true,
	"unless-stopped": true,
}
