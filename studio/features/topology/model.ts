import type { Edge, Node } from '@xyflow/react';
import type {
  ContainerDeployment,
  JavaDeployment,
  UtilityDeployment,
  VMDeployment,
} from '../../lib/api/contracts.ts';

export type Kind =
  | 'java'
  | 'container'
  | 'vm'
  | 'database'
  | 'redis'
  | 'kubernetes'
  | 'registry'
  | 'ingress'
  | 'elasticsearch';
export type NodeData = {
  label: string;
  kind: Kind;
  port: number;
  containerPort: number;
  host: string;
  image: string;
  digest: string;
  registryUsername: string;
  tokenEnv: string;
  java: string;
  heap: string;
  artifact: string;
  artifactRef?: string | null;
  secretRef: string;
  namespace: string;
  platform: string;
  deployment?:
    | VMDeployment
    | JavaDeployment
    | ContainerDeployment
    | UtilityDeployment;
};
export type TopologyNode = Node<NodeData, 'resource' | 'targetGroup'>;
export type EdgePoint = { x: number; y: number };
export type ConnectionVisualLayout = {
  bendOffset?: EdgePoint;
  labelPosition?: number;
  /** @deprecated Read-only compatibility with the first movable-label draft. */
  labelOffset?: EdgePoint;
};
export type TopologyEdgeData = {
  protocol?: 'TCP' | 'UDP';
  port?: number;
  layout?: ConnectionVisualLayout;
  onLayoutChange?: (layout: ConnectionVisualLayout) => void;
  onSelect?: () => void;
};
export type TopologyEdge = Edge<
  TopologyEdgeData,
  'communication' | 'smoothstep'
>;
export type Issue = {
  id: string;
  nodeId?: string;
  severity: 'error' | 'warning';
  message: string;
};
export const catalog: {
  kind: Kind;
  title: string;
  subtitle: string;
  group: string;
  color: string;
}[] = [
  {
    kind: 'java',
    title: 'Java application',
    subtitle: 'JAR · Spring Boot',
    group: 'WORKLOADS',
    color: '#b77b33',
  },
  {
    kind: 'container',
    title: 'Container',
    subtitle: 'Docker · OCI image',
    group: 'WORKLOADS',
    color: '#4e87b8',
  },
  {
    kind: 'vm',
    title: 'Virtual machine',
    subtitle: 'Linux · deploy.yml',
    group: 'DEPLOYMENT TARGETS',
    color: '#397b60',
  },
  {
    kind: 'kubernetes',
    title: 'Kubernetes',
    subtitle: 'Cluster · Helm',
    group: 'DEPLOYMENT TARGETS',
    color: '#4c80b9',
  },
  {
    kind: 'database',
    title: 'PostgreSQL',
    subtitle: 'External database',
    group: 'DEPENDENCIES',
    color: '#6577b3',
  },
  {
    kind: 'redis',
    title: 'Redis',
    subtitle: 'Hosted or external cache',
    group: 'DEPENDENCIES',
    color: '#b56a70',
  },
  {
    kind: 'elasticsearch',
    title: 'Elasticsearch',
    subtitle: 'Hosted or external search',
    group: 'WORKLOADS',
    color: '#ad9450',
  },
  {
    kind: 'registry',
    title: 'Image registry',
    subtitle: 'Harbor · ECR',
    group: 'DELIVERY',
    color: '#8a73aa',
  },
  {
    kind: 'ingress',
    title: 'Ingress',
    subtitle: 'HTTP entry point',
    group: 'DELIVERY',
    color: '#589493',
  },
];
export function createNode(
  kind: Kind,
  id: string,
  x: number,
  y: number,
): TopologyNode {
  const deployment =
    kind === 'redis'
      ? { directories: ['/app/redis'], files: [], services: [] }
      : kind === 'elasticsearch'
        ? { directories: ['/app/elasticsearch'], files: [], services: [] }
        : undefined;
  return {
    id,
    type: ['vm', 'kubernetes'].includes(kind) ? 'targetGroup' : 'resource',
    style: ['vm', 'kubernetes'].includes(kind)
      ? { width: 340, height: 490 }
      : undefined,
    position: { x, y },
    data: {
      kind,
      label: kind + '-' + id.slice(-4),
      port:
        kind === 'java' || kind === 'container'
          ? 8080
          : kind === 'database'
            ? 5432
            : kind === 'redis'
              ? 6379
              : kind === 'elasticsearch'
                ? 9200
                : 0,
      containerPort: kind === 'container' ? 8080 : 0,
      host: '',
      image: '',
      digest: '',
      registryUsername: '',
      tokenEnv: '',
      java: '21',
      heap: '1g',
      artifact: '',
      secretRef: '',
      namespace: 'commerce-dev',
      platform: 'On-prem VM',
      ...(deployment ? { deployment } : {}),
    },
  };
}
export function sample(project = 'bo-pcweb'): {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
} {
  const app = createNode('java', 'app', 50, 100);
  app.data = {
    ...app.data,
    label: project,
    artifact: project + '-1.2.0.jar',
    artifactRef: 'artifact-sample',
  };
  app.parentId = 'vm';
  app.extent = 'parent';
  const vm = createNode('vm', 'vm', 90, 65);
  vm.data = { ...vm.data, label: 'dev-was-01', host: '10.20.10.10' };
  const db = createNode('database', 'db', 590, 90);
  db.data = {
    ...db.data,
    label: 'commerce-db',
    host: 'db.internal.example',
    secretRef: 'commerce-db',
  };
  const redis = createNode('redis', 'redis', 50, 290);
  redis.data = {
    ...redis.data,
    label: 'session-redis',
    host: 'redis.internal.example',
    secretRef: 'session-redis',
  };
  redis.parentId = 'vm';
  redis.extent = 'parent';
  redis.data.host = '';
  return {
    nodes: [vm, app, db, redis],
    edges: [
      {
        id: 'redis-vm',
        source: 'redis',
        target: 'vm',
        label: 'deploys_to',
        type: 'smoothstep',
      },
      {
        id: 'app-vm',
        source: 'app',
        target: 'vm',
        label: 'deploys_to',
        type: 'smoothstep',
      },
      {
        id: 'app-db',
        source: 'app',
        target: 'db',
        label: 'TCP · 5432',
        type: 'communication',
        data: { protocol: 'TCP', port: 5432 },
      },
      {
        id: 'app-redis',
        source: 'app',
        target: 'redis',
        label: 'TCP · 6379',
        type: 'communication',
        data: { protocol: 'TCP', port: 6379 },
      },
    ],
  };
}
export function connectionType(source: Kind, target: Kind): string | null {
  if (
    ['java', 'container', 'redis', 'elasticsearch'].includes(source) &&
    (source === 'container'
      ? target === 'vm'
      : ['vm', 'kubernetes'].includes(target))
  )
    return 'deploys_to';
  if (
    ['java', 'container'].includes(source) &&
    ['database', 'redis', 'elasticsearch', 'java', 'container'].includes(target)
  )
    return 'connects_to';
  if (['java', 'container'].includes(source) && target === 'registry')
    return 'publishes_to';
  if (source === 'ingress' && ['java', 'container'].includes(target))
    return 'routes_to';
  return null;
}
export function validate(
  nodes: TopologyNode[],
  edges: TopologyEdge[],
): Issue[] {
  const issues: Issue[] = [];
  const error = (id: string, message: string, nodeId?: string) =>
    issues.push({ id, message, nodeId, severity: 'error' });
  if (!nodes.length)
    error('empty', '최소 하나의 애플리케이션과 배포 대상을 추가하세요.');
  const names = new Set<string>();
  for (const n of nodes) {
    if (!/^[a-z][a-z0-9-]*$/.test(n.data.label))
      error(n.id + 'name', '이름은 소문자·숫자·하이픈으로 입력하세요.', n.id);
    if (names.has(n.data.label))
      error(n.id + 'duplicate', '중복된 노드 이름입니다.', n.id);
    names.add(n.data.label);
    if (
      ['java', 'container', 'database', 'redis', 'elasticsearch'].includes(
        n.data.kind,
      ) &&
      (!Number.isInteger(n.data.port) || n.data.port < 1 || n.data.port > 65535)
    )
      error(n.id + 'port', '포트는 1–65535 사이의 정수여야 합니다.', n.id);
    if (
      ['vm', 'database', 'redis', 'elasticsearch'].includes(n.data.kind) &&
      !n.parentId &&
      !n.data.host.trim()
    )
      error(n.id + 'host', '호스트 주소를 입력하세요.', n.id);
    if (n.data.kind === 'container') {
      if (
        !Number.isInteger(n.data.containerPort) ||
        n.data.containerPort < 1 ||
        n.data.containerPort > 65535
      )
        error(
          n.id + 'container-port',
          '컨테이너 포트는 1–65535 사이의 정수여야 합니다.',
          n.id,
        );
      if (
        !/^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\/[a-z0-9]+(?:[._-][a-z0-9]+)*)+$/.test(
          n.data.image,
        )
      )
        error(
          n.id + 'image',
          '이미지는 tag 없는 소문자 repository 경로여야 합니다.',
          n.id,
        );
      if (!/^sha256:[a-f0-9]{64}$/.test(n.data.digest))
        error(n.id + 'digest', 'sha256 이미지 digest를 입력하세요.', n.id);
    }
    if (n.data.kind === 'registry') {
      if (!n.data.host.trim())
        error(n.id + 'host', 'Registry host를 입력하세요.', n.id);
      const hasDockerCredentials =
        !!n.data.registryUsername.trim() || !!n.data.tokenEnv.trim();
      if (hasDockerCredentials) {
        if (!n.data.registryUsername.trim())
          error(n.id + 'username', 'Registry 사용자 이름을 입력하세요.', n.id);
        if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(n.data.tokenEnv))
          error(
            n.id + 'token-env',
            '비밀값 대신 token 환경변수 이름을 입력하세요.',
            n.id,
          );
      } else if (!n.data.secretRef.trim()) {
        error(
          n.id + 'credential-ref',
          'Docker 인증 정보 또는 기존 Secret 참조를 입력하세요.',
          n.id,
        );
      }
    }
    if (['java', 'container', 'redis', 'elasticsearch'].includes(n.data.kind)) {
      const targets = edges.filter(
        (e) =>
          e.source === n.id &&
          (n.data.kind === 'container'
            ? ['vm']
            : ['vm', 'kubernetes']
          ).includes(nodes.find((t) => t.id === e.target)?.data.kind ?? ''),
      );
      if (
        (['java', 'container'].includes(n.data.kind) && targets.length !== 1) ||
        targets.length > 1
      )
        error(
          n.id + 'target',
          '애플리케이션에 배포 대상 하나를 연결하세요.',
          n.id,
        );
      if (
        targets[0]?.target !== n.parentId ||
        (n.parentId &&
          !nodes.some(
            (t) =>
              t.id === n.parentId &&
              (n.data.kind === 'container'
                ? t.data.kind === 'vm'
                : ['vm', 'kubernetes'].includes(t.data.kind)),
          ))
      )
        error(
          n.id + 'placement',
          '화면의 실행 대상과 배치 관계가 일치하지 않습니다.',
          n.id,
        );
      if (n.data.kind === 'java' && !n.data.artifactRef)
        issues.push({
          id: n.id + 'artifact',
          nodeId: n.id,
          severity: 'warning',
          message: '파일 메뉴에서 JAR를 등록하고 Java 노드에 연결하세요.',
        });
    }
  }
  for (const e of edges) {
    const from = nodes.find((n) => n.id === e.source),
      to = nodes.find((n) => n.id === e.target);
    if (
      !from ||
      !to ||
      from.id === to.id ||
      !connectionType(from.data.kind, to.data.kind)
    )
      error(e.id, '허용되지 않은 연결입니다.', e.source);
    if (
      e.data?.port !== undefined &&
      (!Number.isInteger(e.data.port) || e.data.port < 1 || e.data.port > 65535)
    )
      error(e.id + 'port', '연결 포트가 유효하지 않습니다.', e.target);
    if (e.data?.port !== undefined && to && e.data.port !== to.data.port)
      error(
        e.id + 'binding',
        '연결 포트와 대상 노드의 포트가 다릅니다.',
        to.id,
      );
  }
  for (const target of nodes.filter((n) => n.data.kind === 'vm')) {
    const ports = new Set<number>();
    for (const e of edges.filter((e) => e.target === target.id)) {
      const app = nodes.find((n) => n.id === e.source);
      if (
        !app ||
        !['java', 'container', 'redis', 'elasticsearch'].includes(app.data.kind)
      )
        continue;
      if (ports.has(app.data.port))
        error(
          app.id + 'conflict',
          '같은 VM/EC2에서 실행되는 서비스의 포트가 충돌합니다.',
          app.id,
        );
      ports.add(app.data.port);
    }
  }
  return issues;
}
