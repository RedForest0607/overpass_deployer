package config

import "path/filepath"

type Config struct {
	SSH       SSHConfig       `yaml:"ssh"`
	Bastion   BastionConfig   `yaml:"bastion"`
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
	Servers   []ServerConfig  `yaml:"servers"`
}

type SSHConfig struct {
	User            string `yaml:"user"`
	KeyPath         string `yaml:"key_path"`
	KnownHosts      string `yaml:"known_hosts_path"`
	HostKeyChecking string `yaml:"host_key_checking"`
	Port            int    `yaml:"port"`
	TimeoutSec      int    `yaml:"timeout_sec"`
}

type BastionConfig struct {
	Host             string `yaml:"host"`
	User             string `yaml:"user"`
	KeyPath          string `yaml:"key_path"`
	KnownHosts       string `yaml:"known_hosts_path"`
	HostKeyChecking  string `yaml:"host_key_checking"`
	Port             int    `yaml:"port"`
	TimeoutSec       int    `yaml:"timeout_sec"`
	AliasUser        string `yaml:"alias_user"`
	SSHConfigPath    string `yaml:"ssh_config_path"`
	ShellAliasesPath string `yaml:"shell_aliases_path"`
	TargetKnownHosts string `yaml:"target_known_hosts_path"`
}

type ServerConfig struct {
	Host           string          `yaml:"host"`
	Name           string          `yaml:"name"`
	Tags           []string        `yaml:"tags"`
	SSHPort        int             `yaml:"ssh_port"`
	BastionHost    string          `yaml:"bastion_host"`
	BastionSSHPort int             `yaml:"bastion_ssh_port"`
	Bootstrap      BootstrapConfig `yaml:"bootstrap"`
	Directories    []string        `yaml:"directories"`
	ExtraFiles     []ExtraFile     `yaml:"extra_files"`
	App            AppConfig       `yaml:"app"`
	Apps           []AppConfig     `yaml:"apps"`
}

type BootstrapConfig struct {
	Packages []string       `yaml:"packages"`
	JDK      JDKConfig      `yaml:"jdk"`
	OSUpdate OSUpdateConfig `yaml:"os_update"`
}

type JDKConfig struct {
	Vendor   string `yaml:"vendor"`
	Major    int    `yaml:"major"`
	Headless *bool  `yaml:"headless"`
}

type OSUpdateConfig struct {
	Enabled *bool `yaml:"enabled"`
}

type AppConfig struct {
	Name        string       `yaml:"name"`
	Tags        []string     `yaml:"tags"`
	BaseDir     string       `yaml:"base_dir"`
	Jar         JarConfig    `yaml:"jar"`
	Jvm         JvmConfig    `yaml:"jvm"`
	Port        int          `yaml:"port"`
	ExtraOpts   StringList   `yaml:"extra_opts"`
	ConfigFiles []ConfigFile `yaml:"config_files"`
	ExtraFiles  []ExtraFile  `yaml:"extra_files"`
	Script      ScriptConfig `yaml:"script"`
}

type JarConfig struct {
	LocalPath  string `yaml:"local_path"`
	RemotePath string `yaml:"remote_path"`
}

type JvmConfig struct {
	MinHeap  string   `yaml:"min_heap"`
	MaxHeap  string   `yaml:"max_heap"`
	JavaOpts []string `yaml:"java_opts"`
}

type ConfigFile struct {
	LocalPath  string `yaml:"local_path"`
	RemotePath string `yaml:"remote_path"`
	Local      string `yaml:"local"`
	Remote     string `yaml:"remote"`
}

type ExtraFile struct {
	LocalPath  string `yaml:"local_path"`
	RemotePath string `yaml:"remote_path"`
	Chmod      string `yaml:"chmod"`
}

type ScriptConfig struct {
	Mode       string `yaml:"mode"`
	Template   string `yaml:"template"`
	LocalPath  string `yaml:"local_path"`
	ValuesFile string `yaml:"values_file"`
	RemotePath string `yaml:"remote_path"`
	RemoteDir  string `yaml:"remote_dir"`
}

// ScriptData is used to render start/stop templates.
type ScriptData struct {
	AppName       string
	BaseDir       string
	JarPath       string
	Port          int
	JvmMin        string
	JvmMax        string
	JavaOpts      []string
	ExtraOpts     []string
	ActiveProfile string
	ContextPath   string
}

func (c *AppConfig) ToScriptData() ScriptData {
	return ScriptData{
		AppName:       c.Name,
		BaseDir:       c.BaseDir,
		JarPath:       c.Jar.RemotePath,
		Port:          c.Port,
		JvmMin:        c.Jvm.MinHeap,
		JvmMax:        c.Jvm.MaxHeap,
		JavaOpts:      c.Jvm.JavaOpts,
		ExtraOpts:     c.ExtraOpts,
		ActiveProfile: "",
		ContextPath:   "",
	}
}

func (c *AppConfig) ToTemplateData() map[string]any {
	data := c.ToScriptData()

	return map[string]any{
		"AppName":       data.AppName,
		"BaseDir":       data.BaseDir,
		"JarPath":       data.JarPath,
		"Port":          data.Port,
		"JvmMin":        data.JvmMin,
		"JvmMax":        data.JvmMax,
		"JavaOpts":      data.JavaOpts,
		"ExtraOpts":     data.ExtraOpts,
		"ActiveProfile": data.ActiveProfile,
		"ContextPath":   data.ContextPath,
	}
}

func (c BastionConfig) Enabled() bool {
	return c.Host != ""
}

func (c BastionConfig) SSHSettings(base SSHConfig) SSHConfig {
	sshCfg := base

	if c.User != "" {
		sshCfg.User = c.User
	}
	if c.KeyPath != "" {
		sshCfg.KeyPath = c.KeyPath
	}
	if c.KnownHosts != "" {
		sshCfg.KnownHosts = c.KnownHosts
	}
	if c.HostKeyChecking != "" {
		sshCfg.HostKeyChecking = c.HostKeyChecking
	}
	if c.Port != 0 {
		sshCfg.Port = c.Port
	}
	if c.TimeoutSec != 0 {
		sshCfg.TimeoutSec = c.TimeoutSec
	}

	return sshCfg
}

func (c ServerConfig) SSHSettings(base SSHConfig) SSHConfig {
	sshCfg := base
	if c.SSHPort != 0 {
		sshCfg.Port = c.SSHPort
	}
	return sshCfg
}

func (c ServerConfig) BastionTargetHost() string {
	if c.BastionHost != "" {
		return c.BastionHost
	}
	return c.Host
}

func (c ServerConfig) BastionTargetPort() int {
	if c.BastionSSHPort != 0 {
		return c.BastionSSHPort
	}
	return c.SSHPort
}

func (c ServerConfig) EffectiveBootstrap(global BootstrapConfig) BootstrapConfig {
	return mergeBootstrapConfig(global, c.Bootstrap)
}

func (c ServerConfig) UsesLegacyApp() bool {
	return !c.App.IsZero()
}

func (c ServerConfig) EffectiveApps() []AppConfig {
	if len(c.Apps) > 0 {
		return c.Apps
	}
	if c.UsesLegacyApp() {
		return []AppConfig{c.App}
	}
	return nil
}

func mergeBootstrapConfig(global BootstrapConfig, override BootstrapConfig) BootstrapConfig {
	merged := BootstrapConfig{
		Packages: mergePackages(global.Packages, override.Packages),
		JDK:      global.JDK,
		OSUpdate: global.OSUpdate,
	}

	if override.JDK.Vendor != "" {
		merged.JDK.Vendor = override.JDK.Vendor
	}
	if override.JDK.Major != 0 {
		merged.JDK.Major = override.JDK.Major
	}
	if override.JDK.Headless != nil {
		merged.JDK.Headless = boolPtr(*override.JDK.Headless)
	}
	if override.OSUpdate.Enabled != nil {
		merged.OSUpdate.Enabled = boolPtr(*override.OSUpdate.Enabled)
	}

	return merged
}

func mergePackages(global []string, override []string) []string {
	if len(global) == 0 && len(override) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(global)+len(override))
	merged := make([]string, 0, len(global)+len(override))
	for _, pkg := range append(append([]string{}, global...), override...) {
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		merged = append(merged, pkg)
	}

	return merged
}

func boolPtr(value bool) *bool {
	return &value
}

func (c AppConfig) IsZero() bool {
	return c.Name == "" &&
		c.BaseDir == "" &&
		c.Jar.LocalPath == "" &&
		c.Jar.RemotePath == "" &&
		c.Port == 0 &&
		len(c.ExtraOpts) == 0 &&
		len(c.ConfigFiles) == 0 &&
		len(c.ExtraFiles) == 0 &&
		c.Script.Mode == "" &&
		c.Script.Template == "" &&
		c.Script.LocalPath == "" &&
		c.Script.ValuesFile == "" &&
		c.Script.RemotePath == "" &&
		c.Script.RemoteDir == "" &&
		c.Jvm.MinHeap == "" &&
		c.Jvm.MaxHeap == "" &&
		len(c.Jvm.JavaOpts) == 0
}

const (
	ScriptModeTemplate  = "template"
	ScriptModeLocalFile = "local-file"
)

func (c *ConfigFile) Normalize() {
	if c.LocalPath == "" {
		c.LocalPath = c.Local
	}
	if c.RemotePath == "" {
		c.RemotePath = c.Remote
	}
}

func (c *ScriptConfig) Normalize(baseDir string) {
	if c.RemotePath == "" {
		if c.RemoteDir != "" {
			c.RemotePath = filepath.ToSlash(filepath.Join(c.RemoteDir, "server.sh"))
		} else {
			c.RemotePath = filepath.ToSlash(filepath.Join(baseDir, "scripts", "server.sh"))
		}
	}
}
