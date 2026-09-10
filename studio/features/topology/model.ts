import type { Edge, Node } from '@xyflow/react';

export type Kind =
  | 'java'
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
  host: string;
  java: string;
  heap: string;
  artifact: string;
  artifactRef?: string | null;
  secretRef: string;
  namespace: string;
  platform: string;
};
export type TopologyNode = Node<NodeData, 'resource' | 'targetGroup'>;
export type TopologyEdge = Edge<{ protocol: 'TCP' | 'UDP'; port: number }>;
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
        kind === 'java'
          ? 8080
          : kind === 'database'
            ? 5432
            : kind === 'redis'
              ? 6379
              : kind === 'elasticsearch'
                ? 9200
                : 0,
      host: '',
      java: '21',
      heap: '1g',
      artifact: '',
      secretRef: '',
      namespace: 'commerce-dev',
      platform: 'On-prem VM',
    },
  };
}
export function sample(project = 'bo-pcweb'): {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
} {
  const app = createNode('java', 'app', 50, 100);
  app.data = { ...app.data, label: project, artifact: project + '-1.2.0.jar' };
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
        type: 'smoothstep',
        data: { protocol: 'TCP', port: 5432 },
      },
      {
        id: 'app-redis',
        source: 'app',
        target: 'redis',
        label: 'TCP · 6379',
        type: 'smoothstep',
        data: { protocol: 'TCP', port: 6379 },
      },
    ],
  };
}
export function connectionType(source: Kind, target: Kind): string | null {
  if (
    ['java', 'redis', 'elasticsearch'].includes(source) &&
    ['vm', 'kubernetes'].includes(target)
  )
    return 'deploys_to';
  if (
    source === 'java' &&
    ['database', 'redis', 'elasticsearch', 'java'].includes(target)
  )
    return 'connects_to';
  if (source === 'java' && target === 'registry') return 'publishes_to';
  if (source === 'ingress' && target === 'java') return 'routes_to';
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
      ['java', 'database', 'redis', 'elasticsearch'].includes(n.data.kind) &&
      (!Number.isInteger(n.data.port) || n.data.port < 1 || n.data.port > 65535)
    )
      error(n.id + 'port', '포트는 1–65535 사이의 정수여야 합니다.', n.id);
    if (
      ['vm', 'database', 'redis', 'elasticsearch'].includes(n.data.kind) &&
      !n.parentId &&
      !n.data.host.trim()
    )
      error(n.id + 'host', '호스트 주소를 입력하세요.', n.id);
    if (['java', 'redis', 'elasticsearch'].includes(n.data.kind)) {
      const targets = edges.filter(
        (e) =>
          e.source === n.id &&
          ['vm', 'kubernetes'].includes(
            nodes.find((t) => t.id === e.target)?.data.kind ?? '',
          ),
      );
      if (
        (n.data.kind === 'java' && targets.length !== 1) ||
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
              t.id === n.parentId && ['vm', 'kubernetes'].includes(t.data.kind),
          ))
      )
        error(
          n.id + 'placement',
          '화면의 실행 대상과 배치 관계가 일치하지 않습니다.',
          n.id,
        );
      if (n.data.kind !== 'java') continue;
      if (!n.data.artifact.trim())
        issues.push({
          id: n.id + 'artifact',
          nodeId: n.id,
          severity: 'warning',
          message: 'JAR 파일을 등록하세요. 지금은 파일명만 설정할 수 있습니다.',
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
      e.data &&
      (!Number.isInteger(e.data.port) || e.data.port < 1 || e.data.port > 65535)
    )
      error(e.id + 'port', '연결 포트가 유효하지 않습니다.', e.target);
    if (e.data && to && e.data.port !== to.data.port)
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
      if (!app || !['java', 'redis', 'elasticsearch'].includes(app.data.kind))
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
