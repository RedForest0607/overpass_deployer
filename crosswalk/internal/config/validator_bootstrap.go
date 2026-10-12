package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

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
