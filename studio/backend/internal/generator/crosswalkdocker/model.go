package crosswalkdocker

type chart struct {
	Format  string        `yaml:"format"`
	SSH     chartSSH      `yaml:"ssh"`
	Bastion *chartBastion `yaml:"bastion,omitempty"`
	Docker  chartDocker   `yaml:"docker"`
	Servers []chartServer `yaml:"servers"`
}

type chartSSH struct {
	User            string `yaml:"user"`
	KeyPath         string `yaml:"key_path"`
	KnownHostsPath  string `yaml:"known_hosts_path"`
	HostKeyChecking string `yaml:"host_key_checking"`
	Port            int    `yaml:"port,omitempty"`
	TimeoutSeconds  int    `yaml:"timeout_sec,omitempty"`
}

type chartBastion struct {
	Host                 string `yaml:"host"`
	User                 string `yaml:"user,omitempty"`
	KeyPath              string `yaml:"key_path,omitempty"`
	KnownHostsPath       string `yaml:"known_hosts_path,omitempty"`
	HostKeyChecking      string `yaml:"host_key_checking,omitempty"`
	Port                 int    `yaml:"port,omitempty"`
	TimeoutSeconds       int    `yaml:"timeout_sec,omitempty"`
	AliasUser            string `yaml:"alias_user,omitempty"`
	SSHConfigPath        string `yaml:"ssh_config_path,omitempty"`
	ShellAliasesPath     string `yaml:"shell_aliases_path,omitempty"`
	TargetKnownHostsPath string `yaml:"target_known_hosts_path,omitempty"`
	TargetKeyPath        string `yaml:"target_key_path,omitempty"`
}

type chartDocker struct {
	Registry chartRegistry `yaml:"registry"`
}

type chartRegistry struct {
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	TokenEnv string `yaml:"token_env"`
}

type chartServer struct {
	Host           string           `yaml:"host"`
	Name           string           `yaml:"name"`
	Tags           []string         `yaml:"tags,omitempty"`
	SSHPort        int              `yaml:"ssh_port,omitempty"`
	BastionHost    string           `yaml:"bastion_host,omitempty"`
	BastionSSHPort int              `yaml:"bastion_ssh_port,omitempty"`
	Containers     []chartContainer `yaml:"containers"`
}

type chartContainer struct {
	Name                 string            `yaml:"name"`
	Tags                 []string          `yaml:"tags,omitempty"`
	Image                string            `yaml:"image"`
	Digest               string            `yaml:"digest"`
	Ports                []chartPort       `yaml:"ports,omitempty"`
	Environment          map[string]string `yaml:"environment,omitempty"`
	RestartPolicy        string            `yaml:"restart,omitempty"`
	HealthTimeoutSeconds int               `yaml:"health_timeout_sec,omitempty"`
}

type chartPort struct {
	Host      int    `yaml:"host"`
	Container int    `yaml:"container"`
	Protocol  string `yaml:"protocol,omitempty"`
}
