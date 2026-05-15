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
	Timezone TimezoneConfig `yaml:"timezone"`
	Swap     SwapConfig     `yaml:"swap"`
}

type JDKConfig struct {
	Vendor   string `yaml:"vendor"`
	Major    int    `yaml:"major"`
	Headless *bool  `yaml:"headless"`
}

type OSUpdateConfig struct {
	Enabled *bool `yaml:"enabled"`
}

type TimezoneConfig struct {
	Name string `yaml:"name"`
}

type SwapConfig struct {
	Enabled *bool  `yaml:"enabled"`
	Path    string `yaml:"path"`
	Size    string `yaml:"size"`
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
	LocalPath  string        `yaml:"local_path"`
	RemotePath string        `yaml:"remote_path"`
	Chmod      string        `yaml:"chmod"`
	Extract    ExtractConfig `yaml:"extract"`
}

type ExtractConfig struct {
	Enabled         bool   `yaml:"enabled"`
	RemoteDir       string `yaml:"remote_dir"`
	StripComponents int    `yaml:"strip_components"`
}

type ScriptConfig struct {
	Mode       string `yaml:"mode"`
	Template   string `yaml:"template"`
	LocalPath  string `yaml:"local_path"`
	ValuesFile string `yaml:"values_file"`
	RemotePath string `yaml:"remote_path"`
	RemoteDir  string `yaml:"remote_dir"`
}

// ScriptData는 시작/중지 스크립트 템플릿 렌더링에 필요한 앱 실행 값을 담는다.
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

// ToScriptData는 앱 설정을 기본 서버 스크립트 템플릿에서 쓰는 구조체로 변환한다.
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

// ToTemplateData는 템플릿 함수와 사용자 override가 다루기 쉬운 map 형태로 앱 값을 변환한다.
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

// Enabled는 bastion.host가 설정되어 실제 배스천 동기화가 필요한지 판단한다.
func (c BastionConfig) Enabled() bool {
	return c.Host != ""
}

// SSHSettings는 전역 SSH 설정 위에 배스천 전용 설정을 덮어쓴 최종 접속값을 만든다.
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

// SSHSettings는 서버별 SSH 포트 설정을 전역 SSH 설정에 반영한다.
func (c ServerConfig) SSHSettings(base SSHConfig) SSHConfig {
	sshCfg := base
	if c.SSHPort != 0 {
		sshCfg.Port = c.SSHPort
	}
	return sshCfg
}

// BastionTargetHost는 배스천에서 접근할 내부 호스트명을 명시값 또는 서버 호스트로 결정한다.
func (c ServerConfig) BastionTargetHost() string {
	if c.BastionHost != "" {
		return c.BastionHost
	}
	return c.Host
}

// BastionTargetPort는 배스천에서 사용할 대상 SSH 포트를 서버별 설정 기준으로 결정한다.
func (c ServerConfig) BastionTargetPort() int {
	if c.BastionSSHPort != 0 {
		return c.BastionSSHPort
	}
	return c.SSHPort
}

// EffectiveBootstrap은 전역 bootstrap 설정과 서버별 override를 병합한 실행 설정을 반환한다.
func (c ServerConfig) EffectiveBootstrap(global BootstrapConfig) BootstrapConfig {
	return mergeBootstrapConfig(global, c.Bootstrap)
}

// UsesLegacyApp은 단일 app 필드를 사용하는 이전 설정 형식인지 판별한다.
func (c ServerConfig) UsesLegacyApp() bool {
	return !c.App.IsZero()
}

// EffectiveApps는 apps 목록과 legacy app 형식을 동일한 앱 목록으로 정규화한다.
func (c ServerConfig) EffectiveApps() []AppConfig {
	if len(c.Apps) > 0 {
		return c.Apps
	}
	if c.UsesLegacyApp() {
		return []AppConfig{c.App}
	}
	return nil
}

// mergeBootstrapConfig는 전역 bootstrap 값에 서버별 값이 있을 때만 덮어써 최종 설정을 만든다.
func mergeBootstrapConfig(global BootstrapConfig, override BootstrapConfig) BootstrapConfig {
	merged := BootstrapConfig{
		Packages: mergePackages(global.Packages, override.Packages),
		JDK:      global.JDK,
		OSUpdate: global.OSUpdate,
		Timezone: global.Timezone,
		Swap:     global.Swap,
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
	if override.Timezone.Name != "" {
		merged.Timezone.Name = override.Timezone.Name
	}
	if override.Swap.Enabled != nil {
		merged.Swap.Enabled = boolPtr(*override.Swap.Enabled)
	}
	if override.Swap.Path != "" {
		merged.Swap.Path = override.Swap.Path
	}
	if override.Swap.Size != "" {
		merged.Swap.Size = override.Swap.Size
	}

	return merged
}

// mergePackages는 전역/서버 패키지 목록을 순서 보존과 중복 제거 규칙으로 합친다.
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

// boolPtr는 bool 값을 선택형 설정 포인터로 다룰 때 사용한다.
func boolPtr(value bool) *bool {
	return &value
}

// IsZero는 legacy app 블록이 실질적으로 비어 있는지 확인한다.
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

// Normalize는 이전 local/remote 필드를 현재 local_path/remote_path 필드로 보정한다.
func (c *ConfigFile) Normalize() {
	if c.LocalPath == "" {
		c.LocalPath = c.Local
	}
	if c.RemotePath == "" {
		c.RemotePath = c.Remote
	}
}

// Normalize는 스크립트 배포 경로가 비어 있을 때 앱 기본 디렉터리 기준 기본값을 채운다.
func (c *ScriptConfig) Normalize(baseDir string) {
	if c.RemotePath == "" {
		if c.RemoteDir != "" {
			c.RemotePath = filepath.ToSlash(filepath.Join(c.RemoteDir, "server.sh"))
		} else {
			c.RemotePath = filepath.ToSlash(filepath.Join(baseDir, "bin", "server.sh"))
		}
	}
}
