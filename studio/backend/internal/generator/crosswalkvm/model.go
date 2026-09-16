package crosswalkvm

type chart struct {
	SSH               chartSSH               `yaml:"ssh"`
	Bastion           *chartBastion          `yaml:"bastion,omitempty"`
	Bootstrap         *chartBootstrap        `yaml:"bootstrap,omitempty"`
	DirectoryDefaults chartDirectoryDefaults `yaml:"directory_defaults,omitempty"`
	Servers           []chartServer          `yaml:"servers"`
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

type chartDirectoryDefaults struct {
	Rules []chartDirectoryRule `yaml:"rules,omitempty"`
}

type chartDirectoryRule struct {
	Prefix string `yaml:"prefix"`
	Owner  string `yaml:"owner"`
	Group  string `yaml:"group,omitempty"`
}

type chartServer struct {
	Host           string          `yaml:"host"`
	Name           string          `yaml:"name"`
	Tags           []string        `yaml:"tags,omitempty"`
	SSHPort        int             `yaml:"ssh_port,omitempty"`
	BastionHost    string          `yaml:"bastion_host,omitempty"`
	BastionSSHPort int             `yaml:"bastion_ssh_port,omitempty"`
	Bootstrap      *chartBootstrap `yaml:"bootstrap,omitempty"`
	Directories    []string        `yaml:"directories,omitempty"`
	ExtraFiles     []chartFile     `yaml:"extra_files,omitempty"`
	Utils          []chartUtility  `yaml:"utils,omitempty"`
	Apps           []chartApp      `yaml:"apps,omitempty"`
}

type chartBootstrap struct {
	Packages      []string            `yaml:"packages,omitempty"`
	Users         []chartUser         `yaml:"users,omitempty"`
	Sysctls       []chartSysctl       `yaml:"sysctl,omitempty"`
	Services      []chartService      `yaml:"services,omitempty"`
	JDK           *chartJDK           `yaml:"jdk,omitempty"`
	OSUpdate      *chartToggle        `yaml:"os_update,omitempty"`
	Timezone      *chartTimezone      `yaml:"timezone,omitempty"`
	Swap          *chartSwap          `yaml:"swap,omitempty"`
	DockerCompose *chartDockerCompose `yaml:"docker_compose,omitempty"`
}

type chartUser struct {
	Name       string `yaml:"name"`
	Group      string `yaml:"group,omitempty"`
	UID        int    `yaml:"uid,omitempty"`
	GID        int    `yaml:"gid,omitempty"`
	Home       string `yaml:"home,omitempty"`
	Shell      string `yaml:"shell,omitempty"`
	System     *bool  `yaml:"system,omitempty"`
	CreateHome *bool  `yaml:"create_home,omitempty"`
}

type chartSysctl struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value"`
	PersistPath string `yaml:"persist_path,omitempty"`
}
type chartService struct {
	Name    string `yaml:"name"`
	Enabled *bool  `yaml:"enabled,omitempty"`
	State   string `yaml:"state,omitempty"`
}
type chartToggle struct {
	Enabled bool `yaml:"enabled"`
}
type chartTimezone struct {
	Name string `yaml:"name"`
}
type chartJDK struct {
	Enabled  bool   `yaml:"enabled"`
	Vendor   string `yaml:"vendor,omitempty"`
	Major    int    `yaml:"major,omitempty"`
	Headless *bool  `yaml:"headless,omitempty"`
}
type chartSwap struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path,omitempty"`
	Size    string `yaml:"size,omitempty"`
}
type chartDockerCompose struct {
	Enabled     bool   `yaml:"enabled"`
	Version     string `yaml:"version,omitempty"`
	SourceURL   string `yaml:"source_url,omitempty"`
	InstallPath string `yaml:"install_path,omitempty"`
}

type chartUtility struct {
	Name        string         `yaml:"name"`
	Directories []string       `yaml:"directories,omitempty"`
	Files       []chartFile    `yaml:"files,omitempty"`
	Services    []chartService `yaml:"services,omitempty"`
}

type chartFile struct {
	LocalPath  string        `yaml:"local_path"`
	RemotePath string        `yaml:"remote_path"`
	Chmod      string        `yaml:"chmod,omitempty"`
	Extract    *chartExtract `yaml:"extract,omitempty"`
}

type chartExtract struct {
	Enabled           bool   `yaml:"enabled"`
	RemoteDir         string `yaml:"remote_dir,omitempty"`
	StripComponents   int    `yaml:"strip_components,omitempty"`
	StagingPath       string `yaml:"staging_path,omitempty"`
	Sudo              bool   `yaml:"sudo,omitempty"`
	Owner             string `yaml:"owner,omitempty"`
	Group             string `yaml:"group,omitempty"`
	ChownRecursive    bool   `yaml:"chown_recursive,omitempty"`
	KeepStagedArchive bool   `yaml:"keep_staged_archive,omitempty"`
}

type chartApp struct {
	Name        string            `yaml:"name"`
	Tags        []string          `yaml:"tags,omitempty"`
	BaseDir     string            `yaml:"base_dir"`
	Jar         chartJar          `yaml:"jar"`
	JVM         chartJVM          `yaml:"jvm"`
	Port        int               `yaml:"port"`
	ExtraOpts   []string          `yaml:"extra_opts,omitempty"`
	ConfigFiles []chartConfigFile `yaml:"config_files,omitempty"`
	ExtraFiles  []chartFile       `yaml:"extra_files,omitempty"`
	Script      chartScript       `yaml:"script"`
}

type chartConfigFile struct {
	LocalPath  string `yaml:"local_path"`
	RemotePath string `yaml:"remote_path"`
}

type chartJar struct {
	LocalPath  string `yaml:"local_path"`
	RemotePath string `yaml:"remote_path"`
}
type chartJVM struct {
	MinHeap  string   `yaml:"min_heap,omitempty"`
	MaxHeap  string   `yaml:"max_heap"`
	JavaOpts []string `yaml:"java_opts,omitempty"`
}
type chartScript struct {
	Mode       string `yaml:"mode"`
	Template   string `yaml:"template,omitempty"`
	LocalPath  string `yaml:"local_path,omitempty"`
	ValuesFile string `yaml:"values_file,omitempty"`
	RemotePath string `yaml:"remote_path"`
	RemoteDir  string `yaml:"remote_dir,omitempty"`
}
