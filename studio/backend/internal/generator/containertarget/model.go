// Package containertarget defines the validated deployment model shared by
// Docker and Compose renderers. Neither renderer reparses another output format.
package containertarget

type Chart struct {
	Format  string   `yaml:"format"`
	SSH     SSH      `yaml:"ssh"`
	Bastion *Bastion `yaml:"bastion,omitempty"`
	Docker  Docker   `yaml:"docker"`
	Servers []Server `yaml:"servers"`
}

type SSH struct {
	User            string `yaml:"user"`
	KeyPath         string `yaml:"key_path"`
	KnownHostsPath  string `yaml:"known_hosts_path"`
	HostKeyChecking string `yaml:"host_key_checking"`
	Port            int    `yaml:"port,omitempty"`
	TimeoutSeconds  int    `yaml:"timeout_sec,omitempty"`
}

type Bastion struct {
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

type Docker struct {
	Registry Registry `yaml:"registry"`
}

type Registry struct {
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	TokenEnv string `yaml:"token_env"`
}

type Server struct {
	Host           string      `yaml:"host"`
	Name           string      `yaml:"name"`
	Tags           []string    `yaml:"tags,omitempty"`
	SSHPort        int         `yaml:"ssh_port,omitempty"`
	BastionHost    string      `yaml:"bastion_host,omitempty"`
	BastionSSHPort int         `yaml:"bastion_ssh_port,omitempty"`
	Containers     []Container `yaml:"containers"`
}

type Container struct {
	Name                 string            `yaml:"name"`
	Tags                 []string          `yaml:"tags,omitempty"`
	Image                string            `yaml:"image"`
	Digest               string            `yaml:"digest"`
	Ports                []Port            `yaml:"ports,omitempty"`
	Environment          map[string]string `yaml:"environment,omitempty"`
	RestartPolicy        string            `yaml:"restart,omitempty"`
	HealthTimeoutSeconds int               `yaml:"health_timeout_sec,omitempty"`
}

type Port struct {
	Host      int    `yaml:"host"`
	Container int    `yaml:"container"`
	Protocol  string `yaml:"protocol,omitempty"`
}
