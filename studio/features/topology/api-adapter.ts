import type { Viewport } from '@xyflow/react';
import type {
  Blueprint,
  BlueprintConnection,
  BlueprintNode,
  DeploymentProfile,
  Layout,
  Placement,
  Port,
} from '../../lib/api/contracts.ts';
import {
  connectionType,
  createNode,
  type Kind,
  type TopologyEdge,
  type TopologyNode,
} from './model.ts';
import { isTarget, isWorkload } from './placement.ts';

const mainPort = (node: TopologyNode): Port[] => [
  { name: 'main', servicePort: node.data.port, protocol: 'TCP' },
];

export const defaultDeploymentProfile: DeploymentProfile = {
  ssh: {
    user: 'deploy',
    keyPath: '${DEPLOY_SSH_KEY_PATH}',
    knownHostsPath: '${DEPLOY_KNOWN_HOSTS}',
    hostKeyChecking: 'strict',
  },
};

export function toAPI(
  nodes: TopologyNode[],
  edges: TopologyEdge[],
  viewport: Viewport,
  deployment?: DeploymentProfile,
): { blueprint: Blueprint; layout: Layout } {
  const byID = new Map(nodes.map((node) => [node.id, node]));
  const placements: Placement[] = nodes.flatMap((node) => {
    if (!node.parentId || !isWorkload(node)) return [];
    const target = byID.get(node.parentId);
    if (!target || !isTarget(target)) return [];
    return [
      {
        workloadRef: node.id,
        targetRef: target.id,
        mode:
          node.data.kind === 'container'
            ? 'container'
            : target.data.kind === 'kubernetes'
              ? 'kubernetes'
              : 'process',
      },
    ];
  });
  const connections: BlueprintConnection[] = edges.flatMap((edge) => {
    const source = byID.get(edge.source);
    const target = byID.get(edge.target);
    if (!source || !target || isTarget(target)) return [];
    const kind = connectionType(source.data.kind, target.data.kind);
    if (!kind || kind === 'deploys_to') return [];
    return [
      {
        id: edge.id,
        kind,
        sourceRef: source.id,
        targetRef: target.id,
        ...(kind === 'connects_to' || kind === 'routes_to'
          ? { targetPortName: 'main' }
          : {}),
      } as BlueprintConnection,
    ];
  });
  const connectionIDs = new Set(connections.map((connection) => connection.id));
  return {
    blueprint: {
      apiVersion: 'studio.deployer/v1',
      ...(deployment ? { deployment } : {}),
      nodes: nodes.map(toBlueprintNode),
      placements,
      connections,
    },
    layout: {
      nodes: nodes.map((node) => ({
        nodeId: node.id,
        position: node.position,
      })),
      connections: edges.flatMap((edge) =>
        connectionIDs.has(edge.id) && edge.data?.layout
          ? [
              {
                connectionId: edge.id,
                ...edge.data.layout,
              },
            ]
          : [],
      ),
      viewport,
    },
  };
}

function toBlueprintNode(node: TopologyNode): BlueprintNode {
  const common = { id: node.id, kind: node.data.kind, name: node.data.label };
  switch (node.data.kind) {
    case 'vm':
      return {
        ...common,
        spec: {
          provider: node.data.platform === 'AWS EC2' ? 'aws-ec2' : 'on-prem',
          host: node.data.host,
          ...(node.data.deployment ? { deployment: node.data.deployment } : {}),
        },
      };
    case 'java':
      return {
        ...common,
        spec: {
          artifactRef: node.data.artifactRef ?? null,
          javaVersion: node.data.java,
          maxHeap: node.data.heap,
          ports: mainPort(node),
          ...(node.data.deployment ? { deployment: node.data.deployment } : {}),
        },
      };
    case 'container':
      return {
        ...common,
        spec: {
          image: node.data.image,
          digest: node.data.digest,
          ports: [
            {
              name: 'main',
              servicePort: node.data.port,
              containerPort: node.data.containerPort,
              protocol: 'TCP',
            },
          ],
          ...(node.data.deployment ? { deployment: node.data.deployment } : {}),
        },
      };
    case 'redis':
    case 'elasticsearch':
      return {
        ...common,
        spec: compact({
          location: node.parentId ? 'hosted' : 'external',
          endpoint: node.parentId ? undefined : node.data.host,
          ports: mainPort(node),
          secretRef: node.data.secretRef || undefined,
          deployment: node.parentId ? node.data.deployment : undefined,
        }),
      };
    case 'database':
      return {
        ...common,
        spec: compact({
          engine: 'postgresql',
          endpoint: node.data.host,
          ports: mainPort(node),
          secretRef: node.data.secretRef || undefined,
        }),
      };
    case 'kubernetes':
      return { ...common, spec: { namespace: node.data.namespace } };
    case 'registry':
      return {
        ...common,
        spec: compact({
          endpoint: node.data.host,
          username: node.data.registryUsername || undefined,
          tokenEnv: node.data.tokenEnv || undefined,
          secretRef: node.data.secretRef || undefined,
        }),
      };
    case 'ingress':
      return { ...common, spec: { host: node.data.host } };
  }
}

export function fromAPI(
  blueprint: Blueprint,
  layout: Layout,
): {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
  viewport: Viewport;
  deployment?: DeploymentProfile;
} {
  assertEditable(blueprint);
  const positions = new Map(
    layout.nodes.map(({ nodeId, position }) => [nodeId, position]),
  );
  const placementByWorkload = new Map(
    blueprint.placements.map((item) => [item.workloadRef, item]),
  );
  const connectionLayouts = new Map(
    (layout.connections ?? []).map((item) => [item.connectionId, item]),
  );
  const nodes = blueprint.nodes.map((value, index) => {
    const position = positions.get(value.id) ?? { x: 80 + index * 40, y: 80 };
    const node = createNode(
      value.kind as Kind,
      value.id,
      position.x,
      position.y,
    );
    node.data = { ...node.data, ...fromSpec(value) };
    const placement = placementByWorkload.get(value.id);
    if (placement) {
      node.parentId = placement.targetRef;
      node.extent = 'parent';
    }
    return node;
  });
  const byID = new Map(nodes.map((node) => [node.id, node]));
  const placementEdges: TopologyEdge[] = blueprint.placements.map((item) => ({
    id: `${item.workloadRef}-placement`,
    source: item.workloadRef,
    target: item.targetRef,
    type: 'smoothstep',
    label: 'deploys_to',
  }));
  const connectionEdges: TopologyEdge[] = blueprint.connections.map((item) => {
    const target = byID.get(item.targetRef);
    const port = target?.data.port ?? 0;
    const network = item.kind === 'connects_to' || item.kind === 'routes_to';
    const savedLayout = connectionLayouts.get(item.id);
    const visualLayout = savedLayout
      ? {
          bendOffset: savedLayout.bendOffset,
          labelPosition: savedLayout.labelPosition,
        }
      : undefined;
    const hasVisualLayout =
      visualLayout?.bendOffset !== undefined ||
      visualLayout?.labelPosition !== undefined;
    return {
      id: item.id,
      source: item.sourceRef,
      target: item.targetRef,
      type: 'communication',
      label: network ? `TCP · ${port}` : item.kind,
      data:
        network || hasVisualLayout
          ? {
              ...(network ? { protocol: 'TCP' as const, port } : {}),
              ...(hasVisualLayout ? { layout: visualLayout } : {}),
            }
          : undefined,
    };
  });
  const targets = nodes.filter(isTarget);
  for (const target of targets) {
    const children = nodes.filter((node) => node.parentId === target.id);
    target.style = {
      width: 340,
      height: Math.max(300, ...children.map((node) => node.position.y + 200)),
    };
  }
  const unparented = nodes.filter((node) => !isTarget(node) && !node.parentId);
  const children = nodes.filter((node) => node.parentId);
  return {
    nodes: [...targets, ...unparented, ...children],
    edges: [...placementEdges, ...connectionEdges],
    viewport: layout.viewport,
    deployment: blueprint.deployment,
  };
}

/** Refuse unsupported shapes rather than silently rewriting a saved model. */
function assertEditable(blueprint: Blueprint) {
  const ids = new Set(blueprint.nodes.map((node) => node.id));
  if (ids.size !== blueprint.nodes.length)
    throw new Error('중복 노드 ID가 있어 편집할 수 없습니다.');
  for (const node of blueprint.nodes) {
    if (Array.isArray(node.spec.ports)) {
      const ports = node.spec.ports as Port[];
      if (
        ports.length !== 1 ||
        ports[0].name !== 'main' ||
        ports[0].protocol !== 'TCP'
      ) {
        throw new Error(
          '현재 편집기는 main TCP 포트 하나를 지원합니다. 원본 설계는 변경하지 않았습니다.',
        );
      }
    }
  }
  const placed = new Set<string>();
  for (const placement of blueprint.placements) {
    const target = blueprint.nodes.find(
      (node) => node.id === placement.targetRef,
    );
    const workload = blueprint.nodes.find(
      (node) => node.id === placement.workloadRef,
    );
    if (
      !workload ||
      !['java', 'container', 'redis', 'elasticsearch'].includes(
        workload.kind,
      ) ||
      !target ||
      !['vm', 'kubernetes'].includes(target.kind) ||
      placed.has(placement.workloadRef) ||
      placement.mode !==
        (workload.kind === 'container'
          ? 'container'
          : target.kind === 'vm'
            ? 'process'
            : 'kubernetes') ||
      (workload.kind === 'container' && target.kind !== 'vm')
    ) {
      throw new Error(
        '지원하지 않거나 잘못된 배치가 있습니다. 원본 설계는 변경하지 않았습니다.',
      );
    }
    placed.add(placement.workloadRef);
  }
  for (const node of blueprint.nodes) {
    if (
      ['redis', 'elasticsearch'].includes(node.kind) &&
      (node.spec.location === 'hosted') !== placed.has(node.id)
    ) {
      throw new Error(
        '서비스 위치와 배치가 일치하지 않습니다. 원본 설계는 변경하지 않았습니다.',
      );
    }
  }
  for (const edge of blueprint.connections) {
    if (
      !ids.has(edge.sourceRef) ||
      !ids.has(edge.targetRef) ||
      (edge.kind !== 'publishes_to' && edge.targetPortName !== 'main')
    ) {
      throw new Error(
        '지원하지 않는 연결 참조가 있습니다. 원본 설계는 변경하지 않았습니다.',
      );
    }
  }
}

function fromSpec(node: BlueprintNode) {
  const spec = node.spec;
  const firstPort = Array.isArray(spec.ports)
    ? (spec.ports[0] as Record<string, unknown> | undefined)
    : undefined;
  const port =
    typeof firstPort?.servicePort === 'number' ? firstPort.servicePort : 0;
  return {
    label: node.name,
    port,
    containerPort:
      typeof firstPort?.containerPort === 'number'
        ? firstPort.containerPort
        : port,
    host: stringValue(spec.host, stringValue(spec.endpoint)),
    image: stringValue(spec.image),
    digest: stringValue(spec.digest),
    registryUsername: stringValue(spec.username),
    tokenEnv: stringValue(spec.tokenEnv),
    java: stringValue(spec.javaVersion, '21'),
    heap: stringValue(spec.maxHeap, '1g'),
    artifact: '',
    artifactRef: typeof spec.artifactRef === 'string' ? spec.artifactRef : null,
    secretRef: stringValue(spec.secretRef),
    namespace: stringValue(spec.namespace, 'commerce-dev'),
    platform: spec.provider === 'aws-ec2' ? 'AWS EC2' : 'On-prem VM',
    deployment: ['vm', 'java', 'container', 'redis', 'elasticsearch'].includes(
      node.kind,
    )
      ? typeof spec.deployment === 'object' && spec.deployment !== null
        ? (spec.deployment as TopologyNode['data']['deployment'])
        : undefined
      : undefined,
  };
}

function stringValue(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback;
}

function compact(value: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(value).filter(([, item]) => item !== undefined),
  );
}
