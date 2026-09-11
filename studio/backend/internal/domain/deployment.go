package domain

// DeploymentProfile contains Crosswalk-wide settings that are not owned by a
// single topology node. Secret values are intentionally excluded; paths may
// point at runtime-provided SSH files or environment placeholders.
type DeploymentProfile struct {
	SSH               SSHProfile               `json:"ssh"`
	Bastion           *BastionProfile          `json:"bastion,omitempty"`
	DirectoryDefaults []DirectoryOwnershipRule `json:"directoryDefaults,omitempty"`
}

type SSHProfile struct {
	User            string `json:"user"`
	KeyPath         string `json:"keyPath"`
	KnownHostsPath  string `json:"knownHostsPath"`
	HostKeyChecking string `json:"hostKeyChecking"`
	Port            int    `json:"port,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds,omitempty"`
}

type BastionProfile struct {
	Host                 string `json:"host"`
	User                 string `json:"user,omitempty"`
	KeyPath              string `json:"keyPath,omitempty"`
	KnownHostsPath       string `json:"knownHostsPath,omitempty"`
	HostKeyChecking      string `json:"hostKeyChecking,omitempty"`
	Port                 int    `json:"port,omitempty"`
	TimeoutSeconds       int    `json:"timeoutSeconds,omitempty"`
	AliasUser            string `json:"aliasUser,omitempty"`
	SSHConfigPath        string `json:"sshConfigPath,omitempty"`
	ShellAliasesPath     string `json:"shellAliasesPath,omitempty"`
	TargetKnownHostsPath string `json:"targetKnownHostsPath,omitempty"`
	TargetKeyPath        string `json:"targetKeyPath,omitempty"`
}

type DirectoryOwnershipRule struct {
	Prefix string `json:"prefix"`
	Owner  string `json:"owner"`
	Group  string `json:"group,omitempty"`
}

type BootstrapProfile struct {
	Packages      []string              `json:"packages,omitempty"`
	Users         []BootstrapUser       `json:"users,omitempty"`
	Sysctls       []SysctlSetting       `json:"sysctls,omitempty"`
	Services      []ServiceSetting      `json:"services,omitempty"`
	JDK           *JDKBootstrap         `json:"jdk,omitempty"`
	OSUpdate      *Toggle               `json:"osUpdate,omitempty"`
	Timezone      string                `json:"timezone,omitempty"`
	Swap          *SwapBootstrap        `json:"swap,omitempty"`
	DockerCompose *DockerComposeInstall `json:"dockerCompose,omitempty"`
}

type BootstrapUser struct {
	Name       string `json:"name"`
	Group      string `json:"group,omitempty"`
	UID        int    `json:"uid,omitempty"`
	GID        int    `json:"gid,omitempty"`
	Home       string `json:"home,omitempty"`
	Shell      string `json:"shell,omitempty"`
	System     *bool  `json:"system,omitempty"`
	CreateHome *bool  `json:"createHome,omitempty"`
}

type SysctlSetting struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	PersistPath string `json:"persistPath,omitempty"`
}

type ServiceSetting struct {
	Name    string `json:"name"`
	Enabled *bool  `json:"enabled,omitempty"`
	State   string `json:"state,omitempty"`
}

type Toggle struct {
	Enabled bool `json:"enabled"`
}

type JDKBootstrap struct {
	Enabled  bool   `json:"enabled"`
	Vendor   string `json:"vendor,omitempty"`
	Major    int    `json:"major,omitempty"`
	Headless *bool  `json:"headless,omitempty"`
}

type SwapBootstrap struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path,omitempty"`
	Size    string `json:"size,omitempty"`
}

type DockerComposeInstall struct {
	Enabled     bool   `json:"enabled"`
	Version     string `json:"version,omitempty"`
	SourceURL   string `json:"sourceUrl,omitempty"`
	InstallPath string `json:"installPath,omitempty"`
}

// ArtifactDelivery describes one immutable Studio artifact and its placement
// in the generated package and on the target VM.
type ArtifactDelivery struct {
	ArtifactRef string          `json:"artifactRef"`
	PackagePath string          `json:"packagePath,omitempty"`
	RemotePath  string          `json:"remotePath"`
	Chmod       string          `json:"chmod,omitempty"`
	Extract     *ExtractProfile `json:"extract,omitempty"`
}

type ExtractProfile struct {
	Enabled           bool   `json:"enabled"`
	RemoteDir         string `json:"remoteDir,omitempty"`
	StripComponents   int    `json:"stripComponents,omitempty"`
	StagingPath       string `json:"stagingPath,omitempty"`
	Sudo              bool   `json:"sudo,omitempty"`
	Owner             string `json:"owner,omitempty"`
	Group             string `json:"group,omitempty"`
	ChownRecursive    bool   `json:"chownRecursive,omitempty"`
	KeepStagedArchive bool   `json:"keepStagedArchive,omitempty"`
}

type UtilityProfile struct {
	Name        string             `json:"name"`
	Directories []string           `json:"directories,omitempty"`
	Files       []ArtifactDelivery `json:"files,omitempty"`
	Services    []ServiceSetting   `json:"services,omitempty"`
}

type VMDeployment struct {
	Tags           []string           `json:"tags,omitempty"`
	SSHPort        int                `json:"sshPort,omitempty"`
	BastionHost    string             `json:"bastionHost,omitempty"`
	BastionSSHPort int                `json:"bastionSSHPort,omitempty"`
	Bootstrap      *BootstrapProfile  `json:"bootstrap,omitempty"`
	Directories    []string           `json:"directories,omitempty"`
	Files          []ArtifactDelivery `json:"files,omitempty"`
	Utilities      []UtilityProfile   `json:"utilities,omitempty"`
}

type ScriptProfile struct {
	Mode              string `json:"mode"`
	Template          string `json:"template,omitempty"`
	ArtifactRef       string `json:"artifactRef,omitempty"`
	PackagePath       string `json:"packagePath,omitempty"`
	ValuesArtifactRef string `json:"valuesArtifactRef,omitempty"`
	ValuesPackagePath string `json:"valuesPackagePath,omitempty"`
	RemotePath        string `json:"remotePath"`
	RemoteDir         string `json:"remoteDir,omitempty"`
}

type JavaDeployment struct {
	BaseDir        string             `json:"baseDir,omitempty"`
	JarRemotePath  string             `json:"jarRemotePath,omitempty"`
	JarPackagePath string             `json:"jarPackagePath,omitempty"`
	MinHeap        string             `json:"minHeap,omitempty"`
	JavaOptions    []string           `json:"javaOptions,omitempty"`
	ExtraOptions   []string           `json:"extraOptions,omitempty"`
	ConfigFiles    []ArtifactDelivery `json:"configFiles,omitempty"`
	ExtraFiles     []ArtifactDelivery `json:"extraFiles,omitempty"`
	Script         *ScriptProfile     `json:"script,omitempty"`
}
