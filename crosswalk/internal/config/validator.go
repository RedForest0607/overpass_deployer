package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	templatepkg "go-deployer/internal/template"
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

// expandHome은 ~/로 시작하는 경로를 현재 사용자 홈 디렉터리의 절대 경로로 확장한다.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// validatePort는 SSH와 앱 포트가 TCP 포트 범위 안에 있는지 누적 검증한다.
func validatePort(port int, fieldName string, errs *[]string) {
	if port < 1 || port > 65535 {
		*errs = append(*errs, fmt.Sprintf("%s must be between 1 and 65535", fieldName))
	}
}

// validateExistingFile은 로컬 파일 경로가 존재하며 디렉터리가 아닌지 확인한다.
func validateExistingFile(path string, fieldName string, errs *[]string) {
	info, err := os.Stat(path)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s does not exist: %s", fieldName, path))
		return
	}
	if info.IsDir() {
		*errs = append(*errs, fmt.Sprintf("%s must be a file: %s", fieldName, path))
	}
}

// validateHostKeyCheckingMode는 호스트 키 검증 모드를 기본값과 허용 목록 기준으로 정규화한다.
func validateHostKeyCheckingMode(mode string, errs *[]string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return HostKeyAcceptNew
	}

	switch normalized {
	case HostKeyStrict, HostKeyAcceptNew, HostKeyInsecure:
		return normalized
	default:
		*errs = append(*errs, fmt.Sprintf("ssh.host_key_checking must be one of %q, %q, or %q", HostKeyStrict, HostKeyAcceptNew, HostKeyInsecure))
		return normalized
	}
}

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

	// 1. SSH Config
	checkUnresolvedEnv(cfg.SSH.User, "ssh.user")
	checkUnresolvedEnv(cfg.SSH.KeyPath, "ssh.key_path")
	if cfg.SSH.User == "" {
		errs = append(errs, "ssh.user is required")
	}
	if cfg.SSH.KeyPath == "" {
		errs = append(errs, "ssh.key_path is required")
	} else {
		cfg.SSH.KeyPath = expandHome(cfg.SSH.KeyPath)
		validateExistingFile(cfg.SSH.KeyPath, "ssh.key_path", &errs)
	}
	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = DefaultSSHPort
	} else {
		validatePort(cfg.SSH.Port, "ssh.port", &errs)
	}
	if cfg.SSH.TimeoutSec == 0 {
		cfg.SSH.TimeoutSec = DefaultSSHTimeout
	} else if cfg.SSH.TimeoutSec < 0 {
		errs = append(errs, "ssh.timeout_sec must be zero or greater")
	}
	cfg.SSH.HostKeyChecking = validateHostKeyCheckingMode(cfg.SSH.HostKeyChecking, &errs)
	if cfg.SSH.HostKeyChecking != HostKeyInsecure {
		if cfg.SSH.KnownHosts == "" {
			cfg.SSH.KnownHosts = "~/.ssh/known_hosts"
		}
		cfg.SSH.KnownHosts = expandHome(cfg.SSH.KnownHosts)
		checkUnresolvedEnv(cfg.SSH.KnownHosts, "ssh.known_hosts_path")
		if cfg.SSH.HostKeyChecking == HostKeyStrict {
			validateExistingFile(cfg.SSH.KnownHosts, "ssh.known_hosts_path", &errs)
		}
	}

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

		if s.UsesLegacyApp() && len(s.Apps) > 0 {
			errs = append(errs, prefix+" cannot define both app and apps")
		}
		if !s.UsesLegacyApp() && len(s.Apps) == 0 && len(cfg.Servers[i].Containers) == 0 && len(cfg.Servers[i].Directories) == 0 && len(cfg.Servers[i].ExtraFiles) == 0 && len(cfg.Servers[i].Utils) == 0 && !hasBootstrapSettings(cfg.Servers[i].Bootstrap) {
			errs = append(errs, prefix+" must define app, apps, containers, directories, extra_files, utils, or bootstrap settings")
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

	checkUnresolvedEnv(cfg.SSH.User, "ssh.user")
	checkUnresolvedEnv(cfg.SSH.KeyPath, "ssh.key_path")
	if cfg.SSH.User == "" {
		errs = append(errs, "ssh.user is required")
	}
	if cfg.SSH.KeyPath == "" {
		errs = append(errs, "ssh.key_path is required")
	} else {
		cfg.SSH.KeyPath = expandHome(cfg.SSH.KeyPath)
		validateExistingFile(cfg.SSH.KeyPath, "ssh.key_path", &errs)
	}
	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = DefaultSSHPort
	} else {
		validatePort(cfg.SSH.Port, "ssh.port", &errs)
	}
	if cfg.SSH.TimeoutSec == 0 {
		cfg.SSH.TimeoutSec = DefaultSSHTimeout
	} else if cfg.SSH.TimeoutSec < 0 {
		errs = append(errs, "ssh.timeout_sec must be zero or greater")
	}
	cfg.SSH.HostKeyChecking = validateHostKeyCheckingMode(cfg.SSH.HostKeyChecking, &errs)
	if cfg.SSH.HostKeyChecking != HostKeyInsecure {
		if cfg.SSH.KnownHosts == "" {
			cfg.SSH.KnownHosts = "~/.ssh/known_hosts"
		}
		cfg.SSH.KnownHosts = expandHome(cfg.SSH.KnownHosts)
		if cfg.SSH.HostKeyChecking == HostKeyStrict {
			validateExistingFile(cfg.SSH.KnownHosts, "ssh.known_hosts_path", &errs)
		}
	}

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

func validateDockerDeploymentConfig(cfg *Config, errs *[]string, checkUnresolvedEnv func(string, string)) {
	hasContainers := false
	for _, server := range cfg.Servers {
		if len(server.Containers) > 0 {
			hasContainers = true
			break
		}
	}
	if !hasContainers {
		return
	}

	registry := &cfg.Docker.Registry
	registry.Host = strings.ToLower(strings.TrimSpace(registry.Host))
	registry.Username = strings.TrimSpace(registry.Username)
	registry.TokenEnv = strings.TrimSpace(registry.TokenEnv)
	checkUnresolvedEnv(registry.Host, "docker.registry.host")
	checkUnresolvedEnv(registry.Username, "docker.registry.username")
	if registry.Host == "" {
		*errs = append(*errs, "docker.registry.host is required when containers are configured")
	} else if !isValidDockerRegistryHost(registry.Host) {
		*errs = append(*errs, "docker.registry.host must be a hostname with an optional port")
	}
	if registry.Username == "" {
		*errs = append(*errs, "docker.registry.username is required when containers are configured")
	} else if !dockerRegistryUsernamePattern.MatchString(registry.Username) {
		*errs = append(*errs, "docker.registry.username contains unsupported characters")
	}
	if !environmentNamePattern.MatchString(registry.TokenEnv) {
		*errs = append(*errs, "docker.registry.token_env must be an environment variable name")
	}
}

func validateContainerConfigs(containers []ContainerConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	names := make(map[string]struct{}, len(containers))
	hostPorts := make(map[string]string)
	for index := range containers {
		container := &containers[index]
		fieldPrefix := fmt.Sprintf("%s[%d]", prefix, index)
		container.Name = strings.TrimSpace(container.Name)
		container.Image = strings.ToLower(strings.TrimSpace(container.Image))
		container.Digest = strings.ToLower(strings.TrimSpace(container.Digest))
		container.RestartPolicy = strings.ToLower(strings.TrimSpace(container.RestartPolicy))
		validateTags(&container.Tags, fieldPrefix+".tags", errs, checkUnresolvedEnv)
		checkUnresolvedEnv(container.Name, fieldPrefix+".name")
		checkUnresolvedEnv(container.Image, fieldPrefix+".image")
		checkUnresolvedEnv(container.Digest, fieldPrefix+".digest")

		if !containerNamePattern.MatchString(container.Name) {
			*errs = append(*errs, fieldPrefix+".name must be a valid Docker container name")
		} else if strings.HasSuffix(container.Name, DockerBackupContainerSuffix) {
			*errs = append(*errs, fieldPrefix+".name uses the reserved Crosswalk backup suffix")
		} else if _, exists := names[container.Name]; exists {
			*errs = append(*errs, fmt.Sprintf("%s.name %q must be unique on the server", fieldPrefix, container.Name))
		}
		names[container.Name] = struct{}{}
		if !dockerRepositoryPattern.MatchString(container.Image) {
			*errs = append(*errs, fieldPrefix+".image must be a lowercase repository path without a tag or digest")
		}
		if !dockerDigestPattern.MatchString(container.Digest) {
			*errs = append(*errs, fieldPrefix+".digest must be a sha256 digest")
		}
		if container.RestartPolicy == "" {
			container.RestartPolicy = "unless-stopped"
		} else if !allowedRestartPolicies[container.RestartPolicy] {
			*errs = append(*errs, fieldPrefix+".restart must be one of no, always, on-failure, or unless-stopped")
		}
		if container.HealthTimeoutSec == 0 {
			container.HealthTimeoutSec = DefaultDockerHealthTimeoutSec
		} else if container.HealthTimeoutSec < 1 || container.HealthTimeoutSec > 3600 {
			*errs = append(*errs, fieldPrefix+".health_timeout_sec must be between 1 and 3600")
		}

		for portIndex := range container.Ports {
			port := &container.Ports[portIndex]
			portPrefix := fmt.Sprintf("%s.ports[%d]", fieldPrefix, portIndex)
			validatePort(port.Host, portPrefix+".host", errs)
			validatePort(port.Container, portPrefix+".container", errs)
			port.Protocol = strings.ToLower(strings.TrimSpace(port.Protocol))
			if port.Protocol == "" {
				port.Protocol = "tcp"
			} else if port.Protocol != "tcp" && port.Protocol != "udp" {
				*errs = append(*errs, portPrefix+".protocol must be tcp or udp")
			}
			key := fmt.Sprintf("%d/%s", port.Host, port.Protocol)
			if previous, exists := hostPorts[key]; exists {
				*errs = append(*errs, fmt.Sprintf("%s.host conflicts with %s on %s", portPrefix, previous, key))
			} else {
				hostPorts[key] = portPrefix
			}
		}

		for name, value := range container.Environment {
			if !environmentNamePattern.MatchString(name) {
				*errs = append(*errs, fmt.Sprintf("%s.environment key %q is invalid", fieldPrefix, name))
			}
			checkUnresolvedEnv(value, fmt.Sprintf("%s.environment.%s", fieldPrefix, name))
			if strings.ContainsAny(value, "\r\n\x00") {
				*errs = append(*errs, fmt.Sprintf("%s.environment.%s must not contain newlines or NUL bytes", fieldPrefix, name))
			}
		}
	}
}

func isValidDockerRegistryHost(value string) bool {
	host := value
	if strings.Count(value, ":") > 1 {
		return false
	}
	if before, port, found := strings.Cut(value, ":"); found {
		host = before
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed < 1 || parsed > 65535 {
			return false
		}
	}
	if !dockerRegistryHostPattern.MatchString(host) || strings.Contains(host, "..") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}

func validateMonitoringApp(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	app.Name = strings.TrimSpace(app.Name)
	app.BaseDir = filepath.ToSlash(strings.TrimSpace(app.BaseDir))
	app.Script.RemotePath = filepath.ToSlash(strings.TrimSpace(app.Script.RemotePath))
	checkUnresolvedEnv(app.Name, prefix+".name")
	checkUnresolvedEnv(app.BaseDir, prefix+".base_dir")
	checkUnresolvedEnv(app.Script.RemotePath, prefix+".script.remote_path")
	if app.Name == "" {
		*errs = append(*errs, prefix+".name is required")
	}
	if app.BaseDir == "" || !strings.HasPrefix(app.BaseDir, "/") {
		*errs = append(*errs, prefix+".base_dir must be an absolute path")
	}
	if app.Script.RemotePath == "" || !strings.HasPrefix(app.Script.RemotePath, "/") {
		*errs = append(*errs, prefix+".script.remote_path must be an absolute path")
	}
	if app.Port != 0 {
		validatePort(app.Port, prefix+".port", errs)
	}
	validateMonitorConfig(app, prefix, errs, checkUnresolvedEnv)
}

// validateMonitorConfig는 원격 shell에 전달되는 관측 기준과 로그 경로를 정규화한다.
func validateMonitorConfig(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	rawProcessMatch := app.Monitor.ProcessMatch
	rawLogPath := app.Monitor.LogPath
	app.Monitor.ProcessMatch = strings.TrimSpace(rawProcessMatch)
	app.Monitor.LogPath = filepath.ToSlash(strings.TrimSpace(rawLogPath))

	checkUnresolvedEnv(app.Monitor.ProcessMatch, prefix+".monitor.process_match")
	checkUnresolvedEnv(app.Monitor.LogPath, prefix+".monitor.log_path")
	if strings.ContainsAny(rawProcessMatch, "\r\n\x00") {
		*errs = append(*errs, prefix+".monitor.process_match must not contain newlines or NUL bytes")
	}
	if strings.ContainsAny(rawLogPath, "\r\n\x00") {
		*errs = append(*errs, prefix+".monitor.log_path must not contain newlines or NUL bytes")
	} else if app.Monitor.LogPath != "" && !strings.HasPrefix(app.Monitor.LogPath, "/") {
		*errs = append(*errs, prefix+".monitor.log_path must be an absolute path")
	}
}

// validateDirectoryDefaultsConfig는 경로 prefix별 기본 소유권 규칙을 검증하고 정규화한다.
func validateDirectoryDefaultsConfig(cfg *DirectoryDefaultsConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if cfg == nil {
		return
	}

	seenPrefixes := make(map[string]struct{}, len(cfg.Rules))
	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		fieldPrefix := fmt.Sprintf("%s.rules[%d]", prefix, i)

		rule.Prefix = filepath.ToSlash(filepath.Clean(strings.TrimSpace(rule.Prefix)))
		rule.Owner = strings.TrimSpace(rule.Owner)
		rule.Group = strings.TrimSpace(rule.Group)

		checkUnresolvedEnv(rule.Prefix, fieldPrefix+".prefix")
		checkUnresolvedEnv(rule.Owner, fieldPrefix+".owner")
		checkUnresolvedEnv(rule.Group, fieldPrefix+".group")

		if rule.Prefix == "." || rule.Prefix == "" {
			*errs = append(*errs, fieldPrefix+".prefix is required")
			continue
		}
		if !strings.HasPrefix(rule.Prefix, "/") {
			*errs = append(*errs, fieldPrefix+".prefix must be an absolute path")
		}
		if _, ok := seenPrefixes[rule.Prefix]; ok {
			*errs = append(*errs, fmt.Sprintf("%s.prefix must be unique: %s", fieldPrefix, rule.Prefix))
		}
		seenPrefixes[rule.Prefix] = struct{}{}

		if rule.Owner == "" {
			*errs = append(*errs, fieldPrefix+".owner is required")
		}
		if rule.Group == "" {
			rule.Group = rule.Owner
		}
		if !isSafeOwnerToken(rule.Owner) {
			*errs = append(*errs, fieldPrefix+".owner contains unsupported characters")
		}
		if !isSafeOwnerToken(rule.Group) {
			*errs = append(*errs, fieldPrefix+".group contains unsupported characters")
		}
	}
}

// validateDirectories는 서버 레벨 디렉터리 목록의 빈 값과 미해결 환경 변수를 검사하고 경로 표기를 정리한다.
func validateDirectories(directories *[]string, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if directories == nil {
		return
	}

	for i, dir := range *directories {
		field := fmt.Sprintf("%s[%d]", prefix, i)
		checkUnresolvedEnv(dir, field)
		if strings.TrimSpace(dir) == "" {
			*errs = append(*errs, field+" must not be empty")
			continue
		}
		(*directories)[i] = filepath.ToSlash(strings.TrimSpace(dir))
	}
}

// validateExtraFiles는 추가 파일 배포 항목의 로컬/원격 경로, chmod, 압축 해제 설정을 검증한다.
func validateExtraFiles(extraFiles *[]ExtraFile, prefix string, errs *[]string, checkUnresolvedEnv func(string, string), prodLike bool) {
	if extraFiles == nil {
		return
	}

	for i := range *extraFiles {
		ef := &(*extraFiles)[i]
		fieldPrefix := fmt.Sprintf("%s[%d]", prefix, i)
		checkUnresolvedEnv(ef.LocalPath, fieldPrefix+".local_path")
		if ef.LocalPath == "" {
			*errs = append(*errs, fieldPrefix+".local_path is required")
		} else {
			if strings.HasPrefix(ef.LocalPath, "~/") {
				ef.LocalPath = expandHome(ef.LocalPath)
			}
			rejectProdSampleFile(ef.LocalPath, fieldPrefix+".local_path", prodLike, errs)
			validateExistingFile(ef.LocalPath, fieldPrefix+".local_path", errs)
		}

		checkUnresolvedEnv(ef.RemotePath, fieldPrefix+".remote_path")
		if ef.RemotePath == "" {
			*errs = append(*errs, fieldPrefix+".remote_path is required")
		}

		if ef.Chmod != "" {
			ef.Chmod = strings.TrimSpace(ef.Chmod)
			if !fileModePattern.MatchString(ef.Chmod) {
				*errs = append(*errs, fieldPrefix+".chmod must be a 3 or 4 digit octal mode")
			}
		}

		validateExtractConfig(ef, fieldPrefix+".extract", errs, checkUnresolvedEnv)
	}
}

// validateExtractConfig는 전송 후 tar 압축 해제 옵션을 검증하고 기본 대상 디렉터리를 채운다.
func validateExtractConfig(ef *ExtraFile, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if !ef.Extract.Enabled {
		return
	}

	if !isSupportedTarArchive(ef.RemotePath) {
		*errs = append(*errs, prefix+".enabled supports only .tar, .tar.gz, or .tgz archives")
	}

	checkUnresolvedEnv(ef.Extract.RemoteDir, prefix+".remote_dir")
	if strings.TrimSpace(ef.Extract.RemoteDir) == "" {
		ef.Extract.RemoteDir = filepath.ToSlash(filepath.Dir(ef.RemotePath))
	} else {
		ef.Extract.RemoteDir = filepath.ToSlash(strings.TrimSpace(ef.Extract.RemoteDir))
	}
	if !strings.HasPrefix(ef.Extract.RemoteDir, "/") {
		*errs = append(*errs, prefix+".remote_dir must be an absolute path")
	}

	if ef.Extract.StripComponents < 0 {
		*errs = append(*errs, prefix+".strip_components must be greater than or equal to zero")
	}

	checkUnresolvedEnv(ef.Extract.StagingPath, prefix+".staging_path")
	ef.Extract.StagingPath = filepath.ToSlash(strings.TrimSpace(ef.Extract.StagingPath))
	if ef.Extract.StagingPath != "" && !strings.HasPrefix(ef.Extract.StagingPath, "/") {
		*errs = append(*errs, prefix+".staging_path must be an absolute path")
	}
	if ef.Extract.Sudo && ef.Extract.StagingPath == "" {
		ef.Extract.StagingPath = filepath.ToSlash(filepath.Join("/tmp", filepath.Base(ef.RemotePath)))
	}

	ef.Extract.Owner = strings.TrimSpace(ef.Extract.Owner)
	ef.Extract.Group = strings.TrimSpace(ef.Extract.Group)
	checkUnresolvedEnv(ef.Extract.Owner, prefix+".owner")
	checkUnresolvedEnv(ef.Extract.Group, prefix+".group")
	if ef.Extract.Group == "" {
		ef.Extract.Group = ef.Extract.Owner
	}
	if ef.Extract.Owner != "" && !isSafeOwnerToken(ef.Extract.Owner) {
		*errs = append(*errs, prefix+".owner contains unsupported characters")
	}
	if ef.Extract.Group != "" && !isSafeOwnerToken(ef.Extract.Group) {
		*errs = append(*errs, prefix+".group contains unsupported characters")
	}
}

// isSupportedTarArchive는 원격 tar 명령으로 처리할 압축 파일 확장자인지 확인한다.
func isSupportedTarArchive(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz")
}

// hasBootstrapSettings는 앱 없이 서버 초기화만 수행할 수 있는 bootstrap 설정이 있는지 판단한다.
func hasBootstrapSettings(bootstrap BootstrapConfig) bool {
	if len(bootstrap.Packages) > 0 {
		return true
	}
	if len(bootstrap.Users) > 0 {
		return true
	}
	if len(bootstrap.Sysctls) > 0 {
		return true
	}
	if len(bootstrap.Services) > 0 {
		return true
	}
	if bootstrap.JDK.Enabled != nil || bootstrap.JDK.Vendor != "" || bootstrap.JDK.Major != 0 || bootstrap.JDK.Headless != nil {
		return true
	}
	if bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled {
		return true
	}
	if bootstrap.Timezone.Name != "" {
		return true
	}
	if bootstrap.Swap.Enabled != nil && *bootstrap.Swap.Enabled {
		return true
	}
	return bootstrap.DockerCompose.Enabled != nil && *bootstrap.DockerCompose.Enabled
}

// validateAppConfig는 앱 배포에 필요한 jar, JVM, 포트, 설정 파일, 스크립트 값을 검증하고 기본값을 적용한다.
func validateAppConfig(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string), prodLike bool) {
	checkUnresolvedEnv(app.Name, prefix+".name")
	if app.Name == "" {
		*errs = append(*errs, prefix+".name is required")
	}
	validateTags(&app.Tags, prefix+".tags", errs, checkUnresolvedEnv)

	checkUnresolvedEnv(app.BaseDir, prefix+".base_dir")
	if app.BaseDir == "" {
		*errs = append(*errs, prefix+".base_dir is required")
	}
	validateMonitorConfig(app, prefix, errs, checkUnresolvedEnv)

	checkUnresolvedEnv(app.Jar.LocalPath, prefix+".jar.local_path")
	if app.Jar.LocalPath == "" {
		*errs = append(*errs, prefix+".jar.local_path is required")
	} else {
		validateExistingFile(app.Jar.LocalPath, prefix+".jar.local_path", errs)
	}

	checkUnresolvedEnv(app.Jar.RemotePath, prefix+".jar.remote_path")
	if app.Jar.RemotePath == "" {
		*errs = append(*errs, prefix+".jar.remote_path is required")
	}

	if app.Jvm.MinHeap == "" {
		app.Jvm.MinHeap = DefaultJvmMin
	}
	if app.Jvm.MaxHeap == "" {
		app.Jvm.MaxHeap = DefaultJvmMax
	}
	for j, opt := range app.Jvm.JavaOpts {
		optField := fmt.Sprintf("%s.jvm.java_opts[%d]", prefix, j)
		checkUnresolvedEnv(opt, optField)
		if strings.TrimSpace(opt) == "" {
			*errs = append(*errs, optField+" must not be empty")
		}
	}

	if app.Port == 0 {
		*errs = append(*errs, prefix+".port is required")
	} else {
		validatePort(app.Port, prefix+".port", errs)
	}

	for j, opt := range app.ExtraOpts {
		optField := fmt.Sprintf("%s.extra_opts[%d]", prefix, j)
		checkUnresolvedEnv(opt, optField)
		if strings.TrimSpace(opt) == "" {
			*errs = append(*errs, optField+" must not be empty")
		}
	}

	for j := range app.ConfigFiles {
		cf := &app.ConfigFiles[j]
		cf.Normalize()
		cfPrefix := fmt.Sprintf("%s.config_files[%d]", prefix, j)
		checkUnresolvedEnv(cf.LocalPath, cfPrefix+".local_path")
		if cf.LocalPath == "" {
			*errs = append(*errs, cfPrefix+".local_path is required")
		} else {
			if strings.HasPrefix(cf.LocalPath, "~/") {
				cf.LocalPath = expandHome(cf.LocalPath)
			}
			rejectProdSampleFile(cf.LocalPath, cfPrefix+".local_path", prodLike, errs)
			validateExistingFile(cf.LocalPath, cfPrefix+".local_path", errs)
		}
		checkUnresolvedEnv(cf.RemotePath, cfPrefix+".remote_path")
		if cf.RemotePath == "" {
			*errs = append(*errs, cfPrefix+".remote_path is required")
		}
	}

	validateExtraFiles(&app.ExtraFiles, prefix+".extra_files", errs, checkUnresolvedEnv, prodLike)

	validateScriptConfig(app, prefix, errs, checkUnresolvedEnv)
}

// validateScriptConfig는 template/local-file 스크립트 모드별로 허용되는 필드와 파일 경로를 검증한다.
func validateScriptConfig(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	mode := strings.ToLower(strings.TrimSpace(app.Script.Mode))
	if mode == "" {
		mode = ScriptModeTemplate
	}

	switch mode {
	case ScriptModeTemplate, ScriptModeLocalFile:
		app.Script.Mode = mode
	default:
		*errs = append(*errs, fmt.Sprintf("%s.script.mode must be %q or %q", prefix, ScriptModeTemplate, ScriptModeLocalFile))
		return
	}

	app.Script.Normalize(app.BaseDir)

	switch app.Script.Mode {
	case ScriptModeTemplate:
		if app.Script.LocalPath != "" {
			checkUnresolvedEnv(app.Script.LocalPath, prefix+".script.local_path")
			if strings.HasPrefix(app.Script.LocalPath, "~/") {
				app.Script.LocalPath = expandHome(app.Script.LocalPath)
			}
		}

		if app.Script.Template != "" {
			checkUnresolvedEnv(app.Script.Template, prefix+".script.template")
			if strings.HasPrefix(app.Script.Template, "~/") {
				app.Script.Template = expandHome(app.Script.Template)
			}
			if strings.HasPrefix(app.Script.Template, "embedded:") {
				if err := templatepkg.ValidateEmbeddedTemplateRef(app.Script.Template); err != nil {
					*errs = append(*errs, fmt.Sprintf("%s is invalid: %v", prefix+".script.template", err))
				}
			} else {
				validateExistingFile(app.Script.Template, prefix+".script.template", errs)
			}
		}

		if app.Script.ValuesFile != "" {
			checkUnresolvedEnv(app.Script.ValuesFile, prefix+".script.values_file")
			if strings.HasPrefix(app.Script.ValuesFile, "~/") {
				app.Script.ValuesFile = expandHome(app.Script.ValuesFile)
			}
			validateExistingFile(app.Script.ValuesFile, prefix+".script.values_file", errs)
		}

	case ScriptModeLocalFile:
		checkUnresolvedEnv(app.Script.LocalPath, prefix+".script.local_path")
		if app.Script.LocalPath == "" {
			*errs = append(*errs, prefix+".script.local_path is required when script.mode is local-file")
		} else {
			if strings.HasPrefix(app.Script.LocalPath, "~/") {
				app.Script.LocalPath = expandHome(app.Script.LocalPath)
			}
			validateExistingFile(app.Script.LocalPath, prefix+".script.local_path", errs)
		}

		if app.Script.Template != "" {
			*errs = append(*errs, prefix+".script.template cannot be used when script.mode is local-file")
		}
		if app.Script.ValuesFile != "" {
			*errs = append(*errs, prefix+".script.values_file cannot be used when script.mode is local-file")
		}
	}
}

// validateBootstrapConfig는 OS 업데이트, 패키지, JDK, 시간대, swap 설정을 검증한다.
func validateBootstrapConfig(cfg *BootstrapConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if cfg == nil {
		return
	}

	for i, pkg := range cfg.Packages {
		field := fmt.Sprintf("%s.packages[%d]", prefix, i)
		checkUnresolvedEnv(pkg, field)
		if strings.TrimSpace(pkg) == "" {
			*errs = append(*errs, field+" must not be empty")
			continue
		}
		cfg.Packages[i] = strings.TrimSpace(pkg)
	}

	validateBootstrapUsers(cfg.Users, prefix+".users", errs, checkUnresolvedEnv)
	validateBootstrapSysctls(cfg.Sysctls, prefix+".sysctl", errs, checkUnresolvedEnv)

	validateServiceConfigs(cfg.Services, prefix+".services", errs, checkUnresolvedEnv)

	if cfg.JDK.Enabled != nil && !*cfg.JDK.Enabled {
		if cfg.JDK.Vendor != "" || cfg.JDK.Major != 0 || cfg.JDK.Headless != nil {
			*errs = append(*errs, prefix+".jdk cannot set vendor, major, or headless when enabled is false")
		}
	} else if cfg.JDK.Vendor != "" {
		cfg.JDK.Vendor = strings.ToLower(strings.TrimSpace(cfg.JDK.Vendor))
		checkUnresolvedEnv(cfg.JDK.Vendor, prefix+".jdk.vendor")
		if cfg.JDK.Vendor != DefaultJDKVendor {
			*errs = append(*errs, fmt.Sprintf("%s.vendor must be %q", prefix+".jdk", DefaultJDKVendor))
		}
	}

	if cfg.JDK.Enabled == nil || *cfg.JDK.Enabled {
		if cfg.JDK.Major < 0 {
			*errs = append(*errs, prefix+".jdk.major must be greater than zero")
		}
	}
	if (cfg.JDK.Enabled == nil || *cfg.JDK.Enabled) && cfg.JDK.Vendor != "" && cfg.JDK.Major == 0 {
		*errs = append(*errs, prefix+".jdk.major is required when "+prefix+".jdk.vendor is set")
	}
	if (cfg.JDK.Enabled == nil || *cfg.JDK.Enabled) && cfg.JDK.Major > 0 && cfg.JDK.Vendor == "" {
		cfg.JDK.Vendor = DefaultJDKVendor
	}
	if (cfg.JDK.Enabled == nil || *cfg.JDK.Enabled) && cfg.JDK.Headless == nil && (cfg.JDK.Vendor != "" || cfg.JDK.Major > 0) {
		cfg.JDK.Headless = boolPtr(true)
	}

	cfg.Timezone.Name = strings.TrimSpace(cfg.Timezone.Name)
	checkUnresolvedEnv(cfg.Timezone.Name, prefix+".timezone.name")

	cfg.Swap.Path = strings.TrimSpace(cfg.Swap.Path)
	cfg.Swap.Size = strings.TrimSpace(cfg.Swap.Size)
	checkUnresolvedEnv(cfg.Swap.Path, prefix+".swap.path")
	checkUnresolvedEnv(cfg.Swap.Size, prefix+".swap.size")
	if cfg.Swap.Enabled != nil && *cfg.Swap.Enabled {
		if cfg.Swap.Path == "" {
			cfg.Swap.Path = "/swapfile"
		}
		if cfg.Swap.Size == "" {
			cfg.Swap.Size = "4G"
		}
	}
	if cfg.Swap.Path != "" && !strings.HasPrefix(cfg.Swap.Path, "/") {
		*errs = append(*errs, prefix+".swap.path must be an absolute path")
	}
	if cfg.Swap.Size != "" && !isValidSwapSize(cfg.Swap.Size) {
		*errs = append(*errs, prefix+".swap.size must be a positive size like 4G, 512M, or 1024K")
	}

	validateDockerComposeConfig(&cfg.DockerCompose, prefix+".docker_compose", errs, checkUnresolvedEnv)
}

// validateUtilityConfigs는 유틸리티 이름의 중복과 파일/디렉터리/서비스 선언을 검증한다.
func validateUtilityConfigs(utils []UtilityConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string), prodLikeServer bool) {
	seen := make(map[string]struct{}, len(utils))
	for i := range utils {
		utilityPrefix := fmt.Sprintf("%s[%d]", prefix, i)
		utils[i].Name = strings.TrimSpace(utils[i].Name)
		checkUnresolvedEnv(utils[i].Name, utilityPrefix+".name")
		if utils[i].Name == "" {
			*errs = append(*errs, utilityPrefix+".name must not be empty")
		} else if !isValidBastionAlias(utils[i].Name) {
			*errs = append(*errs, utilityPrefix+".name must contain only letters, numbers, dots, hyphens, or underscores")
		} else if _, exists := seen[utils[i].Name]; exists {
			*errs = append(*errs, fmt.Sprintf("%s.name %q must be unique per server", utilityPrefix, utils[i].Name))
		} else {
			seen[utils[i].Name] = struct{}{}
		}

		validateDirectories(&utils[i].Directories, utilityPrefix+".directories", errs, checkUnresolvedEnv)
		validateExtraFiles(&utils[i].Files, utilityPrefix+".files", errs, checkUnresolvedEnv, prodLikeServer)
		validateServiceConfigs(utils[i].Services, utilityPrefix+".services", errs, checkUnresolvedEnv)
		if len(utils[i].Directories) == 0 && len(utils[i].Files) == 0 && len(utils[i].Services) == 0 {
			*errs = append(*errs, utilityPrefix+" must define directories, files, or services")
		}
	}
}

// validateServiceConfigs는 bootstrap과 utils가 공유하는 systemd 서비스 선언을 정규화하고 검증한다.
func validateServiceConfigs(services []ServiceConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	for i := range services {
		field := fmt.Sprintf("%s[%d]", prefix, i)
		services[i].Name = strings.TrimSpace(services[i].Name)
		services[i].State = strings.ToLower(strings.TrimSpace(services[i].State))
		checkUnresolvedEnv(services[i].Name, field+".name")
		checkUnresolvedEnv(services[i].State, field+".state")

		if services[i].Name == "" {
			*errs = append(*errs, field+".name must not be empty")
		}
		if services[i].State == "" {
			services[i].State = defaultServiceState(services[i])
		}
		switch services[i].State {
		case ServiceStateStarted, ServiceStateStopped, ServiceStateRestarted:
		default:
			*errs = append(*errs, fmt.Sprintf("%s.state must be one of %q, %q, or %q", field, ServiceStateStarted, ServiceStateStopped, ServiceStateRestarted))
		}
		if services[i].Enabled == nil {
			services[i].Enabled = boolPtr(true)
		}
	}
}

func validateBootstrapUsers(users []UserConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	seen := make(map[string]struct{}, len(users))
	for i := range users {
		user := &users[i]
		field := fmt.Sprintf("%s[%d]", prefix, i)
		user.Name = strings.TrimSpace(user.Name)
		user.Group = strings.TrimSpace(user.Group)
		user.Home = filepath.ToSlash(strings.TrimSpace(user.Home))
		user.Shell = filepath.ToSlash(strings.TrimSpace(user.Shell))
		checkUnresolvedEnv(user.Name, field+".name")
		checkUnresolvedEnv(user.Group, field+".group")
		checkUnresolvedEnv(user.Home, field+".home")
		checkUnresolvedEnv(user.Shell, field+".shell")

		if user.Name == "" {
			*errs = append(*errs, field+".name is required")
		} else if !isSafeOwnerToken(user.Name) {
			*errs = append(*errs, field+".name contains unsupported characters")
		}
		if user.Group == "" {
			user.Group = user.Name
		} else if !isSafeOwnerToken(user.Group) {
			*errs = append(*errs, field+".group contains unsupported characters")
		}
		if user.Name != "" {
			if _, ok := seen[user.Name]; ok {
				*errs = append(*errs, field+".name must be unique")
			}
			seen[user.Name] = struct{}{}
		}
		if user.UID < 0 {
			*errs = append(*errs, field+".uid must be greater than or equal to zero")
		}
		if user.GID < 0 {
			*errs = append(*errs, field+".gid must be greater than or equal to zero")
		}
		if user.Home != "" && !strings.HasPrefix(user.Home, "/") {
			*errs = append(*errs, field+".home must be an absolute path")
		}
		if user.Shell != "" && !strings.HasPrefix(user.Shell, "/") {
			*errs = append(*errs, field+".shell must be an absolute path")
		}
		if user.CreateHome == nil {
			user.CreateHome = boolPtr(false)
		}
		if user.System == nil {
			user.System = boolPtr(true)
		}
	}
}

func validateBootstrapSysctls(sysctls []SysctlConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	seen := make(map[string]struct{}, len(sysctls))
	for i := range sysctls {
		sysctl := &sysctls[i]
		field := fmt.Sprintf("%s[%d]", prefix, i)
		sysctl.Name = strings.TrimSpace(sysctl.Name)
		sysctl.Value = strings.TrimSpace(sysctl.Value)
		sysctl.PersistPath = filepath.ToSlash(strings.TrimSpace(sysctl.PersistPath))
		checkUnresolvedEnv(sysctl.Name, field+".name")
		checkUnresolvedEnv(sysctl.Value, field+".value")
		checkUnresolvedEnv(sysctl.PersistPath, field+".persist_path")

		if sysctl.Name == "" {
			*errs = append(*errs, field+".name is required")
		} else if !sysctlNamePattern.MatchString(sysctl.Name) {
			*errs = append(*errs, field+".name contains unsupported characters")
		}
		if sysctl.Value == "" {
			*errs = append(*errs, field+".value is required")
		}
		if sysctl.Name != "" {
			if _, ok := seen[sysctl.Name]; ok {
				*errs = append(*errs, field+".name must be unique")
			}
			seen[sysctl.Name] = struct{}{}
		}
		if sysctl.PersistPath == "" {
			sysctl.PersistPath = "/etc/sysctl.d/99-overpass.conf"
		}
		if !strings.HasPrefix(sysctl.PersistPath, "/") {
			*errs = append(*errs, field+".persist_path must be an absolute path")
		}
	}
}

func validateDockerComposeConfig(cfg *DockerComposeConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil || !*cfg.Enabled {
		return
	}

	cfg.Version = strings.TrimSpace(cfg.Version)
	cfg.SourceURL = strings.TrimSpace(cfg.SourceURL)
	cfg.InstallPath = filepath.ToSlash(strings.TrimSpace(cfg.InstallPath))

	checkUnresolvedEnv(cfg.Version, prefix+".version")
	checkUnresolvedEnv(cfg.SourceURL, prefix+".source_url")
	checkUnresolvedEnv(cfg.InstallPath, prefix+".install_path")

	if cfg.Version == "" {
		*errs = append(*errs, prefix+".version is required when enabled is true")
	}
	if cfg.SourceURL == "" {
		*errs = append(*errs, prefix+".source_url is required when enabled is true")
	} else if !isHTTPSURL(cfg.SourceURL) {
		*errs = append(*errs, prefix+".source_url must be an https URL")
	}
	if cfg.InstallPath == "" {
		cfg.InstallPath = "/usr/local/lib/docker/cli-plugins/docker-compose"
	}
	if !strings.HasPrefix(cfg.InstallPath, "/") {
		*errs = append(*errs, prefix+".install_path must be an absolute path")
	}
}

func isHTTPSURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return parsed.Scheme == "https" && parsed.Host != ""
}

func defaultServiceState(service ServiceConfig) string {
	if service.Enabled != nil && !*service.Enabled {
		return ServiceStateStopped
	}
	return ServiceStateStarted
}

func applyBootstrapDefaults(cfg *BootstrapConfig) {
	if cfg.OSUpdate.Enabled == nil {
		cfg.OSUpdate.Enabled = boolPtr(false)
	}
	if cfg.Swap.Enabled == nil {
		cfg.Swap.Enabled = boolPtr(false)
	}
	if cfg.Swap.Enabled != nil && *cfg.Swap.Enabled {
		if cfg.Swap.Path == "" {
			cfg.Swap.Path = "/swapfile"
		}
		if cfg.Swap.Size == "" {
			cfg.Swap.Size = "4G"
		}
	}
	if cfg.DockerCompose.Enabled == nil {
		cfg.DockerCompose.Enabled = boolPtr(false)
	}
	if cfg.DockerCompose.Enabled != nil && *cfg.DockerCompose.Enabled && cfg.DockerCompose.InstallPath == "" {
		cfg.DockerCompose.InstallPath = "/usr/local/lib/docker/cli-plugins/docker-compose"
	}
}

func isValidSwapSize(value string) bool {
	if value == "" {
		return false
	}
	last := value[len(value)-1]
	if last != 'K' && last != 'M' && last != 'G' {
		return false
	}
	for _, r := range value[:len(value)-1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value[:len(value)-1] != "" && value[:len(value)-1] != "0"
}

// validateTags는 태그를 소문자로 정규화하고 빈 값, 허용되지 않은 문자, 중복을 정리한다.
func validateTags(tags *[]string, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if tags == nil {
		return
	}

	seen := make(map[string]struct{}, len(*tags))
	normalized := make([]string, 0, len(*tags))

	for i, tag := range *tags {
		field := fmt.Sprintf("%s[%d]", prefix, i)
		checkUnresolvedEnv(tag, field)

		tag = normalizeTag(tag)
		if tag == "" {
			*errs = append(*errs, field+" must not be empty")
			continue
		}
		if !isValidTag(tag) {
			*errs = append(*errs, field+" must contain only letters, numbers, dots, hyphens, or underscores")
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}

	*tags = normalized
}

// normalizeTag는 태그 비교가 일관되도록 공백을 제거하고 소문자로 맞춘다.
func normalizeTag(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

var bastionAliasPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var fileModePattern = regexp.MustCompile(`^[0-7]{3,4}$`)
var sysctlNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var containerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
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

// isValidBastionAlias는 SSH alias로 안전하게 사용할 수 있는 문자만 포함하는지 확인한다.
func isValidBastionAlias(value string) bool {
	return bastionAliasPattern.MatchString(strings.TrimSpace(value))
}

// isValidTag는 태그 필터에 사용할 수 있는 안전한 문자 집합인지 확인한다.
func isValidTag(value string) bool {
	return bastionAliasPattern.MatchString(strings.TrimSpace(value))
}

func isSafeOwnerToken(value string) bool {
	return bastionAliasPattern.MatchString(strings.TrimSpace(value))
}

func hasProdLikeTags(tags []string) bool {
	for _, tag := range tags {
		switch normalizeTag(tag) {
		case "prod", "prd":
			return true
		}
	}
	return false
}

func rejectProdSampleFile(path string, fieldName string, prodLike bool, errs *[]string) {
	if !prodLike {
		return
	}
	base := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(base, ".env.sample") {
		*errs = append(*errs, fieldName+" cannot reference .env.sample for prod/prd servers")
		return
	}
	if strings.Contains(strings.ToLower(path), string(filepath.Separator)+"env"+string(filepath.Separator)) && strings.HasSuffix(base, ".sample") {
		*errs = append(*errs, fieldName+" cannot reference sample env files for prod/prd servers")
	}
}

// validateBastionConfig는 배스천 사용 여부에 따라 필수 SSH 값과 alias 파일 경로 기본값을 검증한다.
func validateBastionConfig(cfg *Config, errs *[]string, checkUnresolvedEnv func(string, string)) {
	bastion := &cfg.Bastion
	if !bastion.Enabled() {
		for _, field := range []struct {
			name  string
			value string
		}{
			{name: "bastion.user", value: bastion.User},
			{name: "bastion.host_key_checking", value: bastion.HostKeyChecking},
			{name: "bastion.key_path", value: bastion.KeyPath},
			{name: "bastion.known_hosts_path", value: bastion.KnownHosts},
			{name: "bastion.alias_user", value: bastion.AliasUser},
			{name: "bastion.ssh_config_path", value: bastion.SSHConfigPath},
			{name: "bastion.shell_aliases_path", value: bastion.ShellAliasesPath},
			{name: "bastion.target_known_hosts_path", value: bastion.TargetKnownHosts},
			{name: "bastion.target_key_path", value: bastion.TargetKeyPath},
		} {
			if strings.TrimSpace(field.value) != "" {
				*errs = append(*errs, field.name+" requires bastion.host")
			}
		}
		if bastion.Port != 0 {
			*errs = append(*errs, "bastion.port requires bastion.host")
		}
		if bastion.TimeoutSec != 0 {
			*errs = append(*errs, "bastion.timeout_sec requires bastion.host")
		}
		return
	}

	checkUnresolvedEnv(bastion.Host, "bastion.host")
	checkUnresolvedEnv(bastion.User, "bastion.user")
	checkUnresolvedEnv(bastion.KeyPath, "bastion.key_path")
	checkUnresolvedEnv(bastion.KnownHosts, "bastion.known_hosts_path")
	checkUnresolvedEnv(bastion.AliasUser, "bastion.alias_user")
	checkUnresolvedEnv(bastion.SSHConfigPath, "bastion.ssh_config_path")
	checkUnresolvedEnv(bastion.ShellAliasesPath, "bastion.shell_aliases_path")
	checkUnresolvedEnv(bastion.TargetKnownHosts, "bastion.target_known_hosts_path")
	checkUnresolvedEnv(bastion.TargetKeyPath, "bastion.target_key_path")

	if bastion.User == "" {
		bastion.User = cfg.SSH.User
	}
	if bastion.User == "" {
		*errs = append(*errs, "bastion.user is required")
	}

	if bastion.KeyPath == "" {
		bastion.KeyPath = cfg.SSH.KeyPath
	}
	if bastion.KeyPath == "" {
		*errs = append(*errs, "bastion.key_path is required")
	} else {
		bastion.KeyPath = expandHome(bastion.KeyPath)
		validateExistingFile(bastion.KeyPath, "bastion.key_path", errs)
	}

	if bastion.Port == 0 {
		bastion.Port = cfg.SSH.Port
	}
	validatePort(bastion.Port, "bastion.port", errs)

	if bastion.TimeoutSec == 0 {
		bastion.TimeoutSec = cfg.SSH.TimeoutSec
	} else if bastion.TimeoutSec < 0 {
		*errs = append(*errs, "bastion.timeout_sec must be zero or greater")
	}

	if bastion.HostKeyChecking == "" {
		bastion.HostKeyChecking = cfg.SSH.HostKeyChecking
	}
	bastion.HostKeyChecking = validateHostKeyCheckingMode(bastion.HostKeyChecking, errs)
	if bastion.HostKeyChecking != HostKeyInsecure {
		if bastion.KnownHosts == "" {
			bastion.KnownHosts = cfg.SSH.KnownHosts
		}
		if bastion.KnownHosts == "" {
			bastion.KnownHosts = "~/.ssh/known_hosts"
		}
		bastion.KnownHosts = expandHome(bastion.KnownHosts)
		if bastion.HostKeyChecking == HostKeyStrict {
			validateExistingFile(bastion.KnownHosts, "bastion.known_hosts_path", errs)
		}
	}

	if bastion.AliasUser == "" {
		bastion.AliasUser = cfg.SSH.User
	}
	if strings.TrimSpace(bastion.AliasUser) == "" {
		*errs = append(*errs, "bastion.alias_user is required")
	}

	if bastion.SSHConfigPath == "" {
		bastion.SSHConfigPath = "~/.ssh/config"
	}

	if bastion.ShellAliasesPath == "" {
		bastion.ShellAliasesPath = "~/.bashrc"
	}

	if bastion.TargetKnownHosts == "" {
		bastion.TargetKnownHosts = "~/.ssh/known_hosts"
	}
	bastion.TargetKeyPath = strings.TrimSpace(bastion.TargetKeyPath)
}
