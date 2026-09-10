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
  nodes: BlueprintNode[];
  placements: Placement[];
  connections: BlueprintConnection[];
};

export type Layout = {
  nodes: { nodeId: string; position: { x: number; y: number } }[];
  viewport: { x: number; y: number; zoom: number };
};
