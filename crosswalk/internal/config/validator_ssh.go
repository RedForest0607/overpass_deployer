package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func validateSSHConfig(cfg *SSHConfig, errs *[]string, checkUnresolvedEnv func(string, string)) {
	checkUnresolvedEnv(cfg.User, "ssh.user")
	checkUnresolvedEnv(cfg.KeyPath, "ssh.key_path")
	if cfg.User == "" {
		*errs = append(*errs, "ssh.user is required")
	}
	if cfg.KeyPath == "" {
		*errs = append(*errs, "ssh.key_path is required")
	} else {
		cfg.KeyPath = expandHome(cfg.KeyPath)
		validateExistingFile(cfg.KeyPath, "ssh.key_path", errs)
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultSSHPort
	} else {
		validatePort(cfg.Port, "ssh.port", errs)
	}
	if cfg.TimeoutSec == 0 {
		cfg.TimeoutSec = DefaultSSHTimeout
	} else if cfg.TimeoutSec < 0 {
		*errs = append(*errs, "ssh.timeout_sec must be zero or greater")
	}
	cfg.HostKeyChecking = validateHostKeyCheckingMode(cfg.HostKeyChecking, errs)
	if cfg.HostKeyChecking != HostKeyInsecure {
		if cfg.KnownHosts == "" {
			cfg.KnownHosts = "~/.ssh/known_hosts"
		}
		cfg.KnownHosts = expandHome(cfg.KnownHosts)
		checkUnresolvedEnv(cfg.KnownHosts, "ssh.known_hosts_path")
		if cfg.HostKeyChecking == HostKeyStrict {
			validateExistingFile(cfg.KnownHosts, "ssh.known_hosts_path", errs)
		}
	}
}
