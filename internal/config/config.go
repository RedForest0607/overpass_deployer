package config

type Config struct {
	SSH     SSHConfig      `yaml:"ssh"`
	Bastion BastionConfig  `yaml:"bastion"`
	Servers []ServerConfig `yaml:"servers"`
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
	TargetKnownHosts string `yaml:"target_known_hosts_path"`
}

type ServerConfig struct {
	Host string    `yaml:"host"`
	Name string    `yaml:"name"`
	App  AppConfig `yaml:"app"`
}

type AppConfig struct {
	Name        string       `yaml:"name"`
	BaseDir     string       `yaml:"base_dir"`
	Jar         JarConfig    `yaml:"jar"`
	Jvm         JvmConfig    `yaml:"jvm"`
	Port        int          `yaml:"port"`
	ExtraOpts   StringList   `yaml:"extra_opts"`
	ConfigFiles []ConfigFile `yaml:"config_files"`
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
	Local  string `yaml:"local"`
	Remote string `yaml:"remote"`
}

type ScriptConfig struct {
	Template  string `yaml:"template"`
	RemoteDir string `yaml:"remote_dir"`
}

// ScriptData is used to render start/stop templates.
type ScriptData struct {
	AppName   string
	BaseDir   string
	JarPath   string
	Port      int
	JvmMin    string
	JvmMax    string
	JavaOpts  []string
	ExtraOpts []string
}

func (c *AppConfig) ToScriptData() ScriptData {
	return ScriptData{
		AppName:   c.Name,
		BaseDir:   c.BaseDir,
		JarPath:   c.Jar.RemotePath,
		Port:      c.Port,
		JvmMin:    c.Jvm.MinHeap,
		JvmMax:    c.Jvm.MaxHeap,
		JavaOpts:  c.Jvm.JavaOpts,
		ExtraOpts: c.ExtraOpts,
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
