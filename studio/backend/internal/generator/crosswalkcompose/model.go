package crosswalkcompose

import "studio-backend/internal/generator/containertarget"

type chartSSH = containertarget.SSH
type chartBastion = containertarget.Bastion
type chartDocker = containertarget.Docker

type composeChart struct {
	Format  string          `yaml:"format"`
	SSH     chartSSH        `yaml:"ssh"`
	Bastion *chartBastion   `yaml:"bastion,omitempty"`
	Docker  chartDocker     `yaml:"docker"`
	Servers []composeServer `yaml:"servers"`
}

type composeServer struct {
	Host            string           `yaml:"host"`
	Name            string           `yaml:"name"`
	Tags            []string         `yaml:"tags,omitempty"`
	SSHPort         int              `yaml:"ssh_port,omitempty"`
	BastionHost     string           `yaml:"bastion_host,omitempty"`
	BastionSSHPort  int              `yaml:"bastion_ssh_port,omitempty"`
	ComposeProjects []composeProject `yaml:"compose_projects"`
}

type composeProject struct {
	Name      string `yaml:"name"`
	LocalPath string `yaml:"local_path"`
	RemoteDir string `yaml:"remote_dir"`
}

type composeDocument struct {
	Name     string                    `yaml:"name"`
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image       string            `yaml:"image"`
	Ports       []string          `yaml:"ports,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Restart     string            `yaml:"restart,omitempty"`
}
