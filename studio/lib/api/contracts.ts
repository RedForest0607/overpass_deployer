export type Environment = 'dev' | 'qa' | 'prod';
export type NodeKind =
  | 'vm'
  | 'java'
  | 'redis'
  | 'elasticsearch'
  | 'database'
  | 'kubernetes'
  | 'registry'
  | 'ingress';

export type Port = {
  name: string;
  servicePort: number;
  protocol: 'TCP' | 'UDP';
};

export type ServiceSetting = {
  name: string;
  enabled?: boolean;
  state?: 'started' | 'stopped' | 'restarted';
};
export type ExtractProfile = {
  enabled: boolean;
  remoteDir?: string;
  stripComponents?: number;
  stagingPath?: string;
  sudo?: boolean;
  owner?: string;
  group?: string;
  chownRecursive?: boolean;
  keepStagedArchive?: boolean;
};
export type ArtifactDelivery = {
  artifactRef: string;
  packagePath?: string;
  remotePath: string;
  chmod?: string;
  extract?: ExtractProfile;
};
export type ConfigArtifactDelivery = {
  artifactRef: string;
  packagePath?: string;
  remotePath: string;
};
export type BootstrapProfile = {
  packages?: string[];
  users?: Record<string, unknown>[];
  sysctls?: Record<string, unknown>[];
  services?: ServiceSetting[];
  jdk?: {
    enabled: boolean;
    vendor?: string;
    major?: number;
    headless?: boolean;
  };
  osUpdate?: { enabled: boolean };
  timezone?: string;
  swap?: { enabled: boolean; path?: string; size?: string };
  dockerCompose?: {
    enabled: boolean;
    version?: string;
    sourceUrl?: string;
    installPath?: string;
  };
};
export type VMDeployment = {
  tags?: string[];
  sshPort?: number;
  bastionHost?: string;
  bastionSSHPort?: number;
  bootstrap?: BootstrapProfile;
  directories?: string[];
  files?: ArtifactDelivery[];
  utilities?: {
    name: string;
    directories?: string[];
    files?: ArtifactDelivery[];
    services?: ServiceSetting[];
  }[];
};
export type JavaDeployment = {
  tags?: string[];
  baseDir?: string;
  jarRemotePath?: string;
  jarPackagePath?: string;
  minHeap?: string;
  javaOptions?: string[];
  extraOptions?: string[];
  configFiles?: ConfigArtifactDelivery[];
  extraFiles?: ArtifactDelivery[];
  script?: {
    mode: 'template' | 'local-file';
    template?: string;
    artifactRef?: string;
    packagePath?: string;
    valuesArtifactRef?: string;
    valuesPackagePath?: string;
    remotePath: string;
    remoteDir?: string;
  };
};
export type UtilityDeployment = {
  directories?: string[];
  files?: ArtifactDelivery[];
  services?: ServiceSetting[];
};
export type DeploymentProfile = {
  ssh: {
    user: string;
    keyPath: string;
    knownHostsPath: string;
    hostKeyChecking: 'strict' | 'accept-new' | 'insecure';
    port?: number;
    timeoutSeconds?: number;
  };
  bastion?: Record<string, unknown> & { host: string };
  bootstrap?: BootstrapProfile;
  directoryDefaults?: { prefix: string; owner: string; group?: string }[];
};

export type BlueprintNode = {
  id: string;
  kind: NodeKind;
  name: string;
  spec: Record<string, unknown>;
};

export type Placement = {
  workloadRef: string;
  targetRef: string;
  mode: 'process' | 'compose' | 'kubernetes';
};

export type BlueprintConnection = {
  id: string;
  kind: 'connects_to' | 'routes_to' | 'publishes_to';
  sourceRef: string;
  targetRef: string;
  targetPortName?: string;
};

export type Blueprint = {
  apiVersion: 'studio.deployer/v1';
  deployment?: DeploymentProfile;
  nodes: BlueprintNode[];
  placements: Placement[];
  connections: BlueprintConnection[];
};

export type Layout = {
  nodes: { nodeId: string; position: { x: number; y: number } }[];
  connections?: {
    connectionId: string;
    bendOffset?: { x: number; y: number };
    labelPosition?: number;
    /** @deprecated Accepted while loading the first movable-label draft. */
    labelOffset?: { x: number; y: number };
  }[];
  viewport: { x: number; y: number; zoom: number };
};
