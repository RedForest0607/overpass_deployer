package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	templatepkg "go-deployer/internal/template"
)

const (
	DefaultSSHTimeout = 30
	DefaultJvmMin     = "256m"
	DefaultJvmMax     = "1g"
	DefaultSSHPort    = 22
	DefaultJDKVendor  = "corretto"
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
	aliasNames := make(map[string]int, len(cfg.Servers))

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
		validateExtraFiles(&cfg.Servers[i].ExtraFiles, prefix+".extra_files", &errs, checkUnresolvedEnv)

		if s.UsesLegacyApp() && len(s.Apps) > 0 {
			errs = append(errs, prefix+" cannot define both app and apps")
		}
		if !s.UsesLegacyApp() && len(s.Apps) == 0 && len(cfg.Servers[i].Directories) == 0 && len(cfg.Servers[i].ExtraFiles) == 0 && !hasBootstrapSettings(cfg.Servers[i].Bootstrap) {
			errs = append(errs, prefix+" must define app, apps, directories, extra_files, or bootstrap settings")
		}

		if s.UsesLegacyApp() {
			validateAppConfig(&cfg.Servers[i].App, prefix+".app", &errs, checkUnresolvedEnv)
		}
		for j := range cfg.Servers[i].Apps {
			validateAppConfig(&cfg.Servers[i].Apps[j], fmt.Sprintf("%s.apps[%d]", prefix, j), &errs, checkUnresolvedEnv)
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

func validateExtraFiles(extraFiles *[]ExtraFile, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
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
	}
}

func hasBootstrapSettings(bootstrap BootstrapConfig) bool {
	if len(bootstrap.Packages) > 0 {
		return true
	}
	if bootstrap.JDK.Vendor != "" || bootstrap.JDK.Major != 0 || bootstrap.JDK.Headless != nil {
		return true
	}
	return bootstrap.OSUpdate.Enabled != nil && *bootstrap.OSUpdate.Enabled
}

func validateAppConfig(app *AppConfig, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	checkUnresolvedEnv(app.Name, prefix+".name")
	if app.Name == "" {
		*errs = append(*errs, prefix+".name is required")
	}
	validateTags(&app.Tags, prefix+".tags", errs, checkUnresolvedEnv)

	checkUnresolvedEnv(app.BaseDir, prefix+".base_dir")
	if app.BaseDir == "" {
		*errs = append(*errs, prefix+".base_dir is required")
	}

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
			validateExistingFile(cf.LocalPath, cfPrefix+".local_path", errs)
		}
		checkUnresolvedEnv(cf.RemotePath, cfPrefix+".remote_path")
		if cf.RemotePath == "" {
			*errs = append(*errs, cfPrefix+".remote_path is required")
		}
	}

	validateExtraFiles(&app.ExtraFiles, prefix+".extra_files", errs, checkUnresolvedEnv)

	validateScriptConfig(app, prefix, errs, checkUnresolvedEnv)
}

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

	if cfg.JDK.Vendor != "" {
		cfg.JDK.Vendor = strings.ToLower(strings.TrimSpace(cfg.JDK.Vendor))
		checkUnresolvedEnv(cfg.JDK.Vendor, prefix+".jdk.vendor")
		if cfg.JDK.Vendor != DefaultJDKVendor {
			*errs = append(*errs, fmt.Sprintf("%s.vendor must be %q", prefix+".jdk", DefaultJDKVendor))
		}
	}

	if cfg.JDK.Major < 0 {
		*errs = append(*errs, prefix+".jdk.major must be greater than zero")
	}
	if cfg.JDK.Vendor != "" && cfg.JDK.Major == 0 {
		*errs = append(*errs, prefix+".jdk.major is required when "+prefix+".jdk.vendor is set")
	}
	if cfg.JDK.Major > 0 && cfg.JDK.Vendor == "" {
		cfg.JDK.Vendor = DefaultJDKVendor
	}
	if cfg.JDK.Headless == nil && (cfg.JDK.Vendor != "" || cfg.JDK.Major > 0) {
		cfg.JDK.Headless = boolPtr(true)
	}
	if cfg.OSUpdate.Enabled == nil {
		cfg.OSUpdate.Enabled = boolPtr(false)
	}
}

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

func normalizeTag(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

var bastionAliasPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
var fileModePattern = regexp.MustCompile(`^[0-7]{3,4}$`)

func isValidBastionAlias(value string) bool {
	return bastionAliasPattern.MatchString(strings.TrimSpace(value))
}

func isValidTag(value string) bool {
	return bastionAliasPattern.MatchString(strings.TrimSpace(value))
}

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
}
