package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultSSHTimeout = 30
	DefaultJvmMin     = "256m"
	DefaultJvmMax     = "1g"
	DefaultSSHPort    = 22
	HostKeyStrict     = "strict"
	HostKeyAcceptNew  = "accept-new"
	HostKeyInsecure   = "insecure"
)

// expandHome expands a path that starts with `~/` to the absolute path of the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func validatePort(port int, fieldName string, errs *[]string) {
	if port < 1 || port > 65535 {
		*errs = append(*errs, fmt.Sprintf("%s must be between 1 and 65535", fieldName))
	}
}

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

func ValidateAndApplyDefaults(cfg *Config) error {
	var errs []string

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

	// 2. Server Configs
	if len(cfg.Servers) == 0 {
		errs = append(errs, "at least one server must be specified")
	}

	for i, s := range cfg.Servers {
		prefix := fmt.Sprintf("servers[%d]", i)
		if s.Host == "" {
			errs = append(errs, prefix+".host is required")
		}

		checkUnresolvedEnv(s.App.Name, prefix+".app.name")
		if s.App.Name == "" {
			errs = append(errs, prefix+".app.name is required")
		}

		checkUnresolvedEnv(s.App.BaseDir, prefix+".app.base_dir")
		if s.App.BaseDir == "" {
			errs = append(errs, prefix+".app.base_dir is required")
		}

		checkUnresolvedEnv(s.App.Jar.LocalPath, prefix+".app.jar.local_path")
		if s.App.Jar.LocalPath == "" {
			errs = append(errs, prefix+".app.jar.local_path is required")
		} else {
			validateExistingFile(s.App.Jar.LocalPath, prefix+".app.jar.local_path", &errs)
		}

		checkUnresolvedEnv(s.App.Jar.RemotePath, prefix+".app.jar.remote_path")
		if s.App.Jar.RemotePath == "" {
			errs = append(errs, prefix+".app.jar.remote_path is required")
		}

		if s.App.Jvm.MinHeap == "" {
			cfg.Servers[i].App.Jvm.MinHeap = DefaultJvmMin
		}
		if s.App.Jvm.MaxHeap == "" {
			cfg.Servers[i].App.Jvm.MaxHeap = DefaultJvmMax
		}
		for j, opt := range s.App.Jvm.JavaOpts {
			optField := fmt.Sprintf("%s.app.jvm.java_opts[%d]", prefix, j)
			checkUnresolvedEnv(opt, optField)
			if strings.TrimSpace(opt) == "" {
				errs = append(errs, optField+" must not be empty")
			}
		}

		if s.App.Port == 0 {
			errs = append(errs, prefix+".app.port is required")
		} else {
			validatePort(s.App.Port, prefix+".app.port", &errs)
		}

		for j, opt := range s.App.ExtraOpts {
			optField := fmt.Sprintf("%s.app.extra_opts[%d]", prefix, j)
			checkUnresolvedEnv(opt, optField)
			if strings.TrimSpace(opt) == "" {
				errs = append(errs, optField+" must not be empty")
			}
		}

		for j, cf := range s.App.ConfigFiles {
			cfPrefix := fmt.Sprintf("%s.app.config_files[%d]", prefix, j)
			checkUnresolvedEnv(cf.Local, cfPrefix+".local")
			if cf.Local == "" {
				errs = append(errs, cfPrefix+".local is required")
			} else {
				validateExistingFile(cf.Local, cfPrefix+".local", &errs)
			}
			checkUnresolvedEnv(cf.Remote, cfPrefix+".remote")
			if cf.Remote == "" {
				errs = append(errs, cfPrefix+".remote is required")
			}
		}

		if s.App.Script.Template != "" {
			checkUnresolvedEnv(s.App.Script.Template, prefix+".app.script.template")
			validateExistingFile(s.App.Script.Template, prefix+".app.script.template", &errs)
		}

		if s.App.Script.RemoteDir == "" {
			cfg.Servers[i].App.Script.RemoteDir = filepath.ToSlash(filepath.Join(s.App.BaseDir, "scripts"))
		}

		// If Name is not given, default it to Host to help logging
		if s.Name == "" {
			cfg.Servers[i].Name = s.Host
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n- %s", strings.Join(errs, "\n- "))
	}

	return nil
}
