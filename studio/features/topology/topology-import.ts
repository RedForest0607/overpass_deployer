import type {
  Blueprint,
  DeploymentProfile,
  Environment,
  Layout,
} from '../../lib/api/contracts.ts';
import { fromAPI, toAPI } from './api-adapter.ts';
import type { TopologyEdge, TopologyNode } from './model.ts';

export const MAX_TOPOLOGY_IMPORT_BYTES = 2 * 1024 * 1024;

export type ImportedTopology = {
  environment?: Environment;
  graph: {
    nodes: TopologyNode[];
    edges: TopologyEdge[];
    viewport: Layout['viewport'];
    deployment?: DeploymentProfile;
  };
};

export function parseTopologyImport(source: string): ImportedTopology {
  if (!source.trim()) throw new Error('가져올 JSON을 입력하세요.');
  if (new TextEncoder().encode(source).byteLength > MAX_TOPOLOGY_IMPORT_BYTES) {
    throw new Error('토폴로지 JSON은 2 MiB 이하여야 합니다.');
  }

  let value: unknown;
  try {
    value = JSON.parse(source);
  } catch (cause) {
    const detail = cause instanceof Error ? `: ${cause.message}` : '';
    throw new Error(`JSON 문법을 확인하세요${detail}`);
  }
  if (!isRecord(value)) throw new Error('최상위 값은 JSON 객체여야 합니다.');

  const environment = parseEnvironment(value.environment);
  const wrapped = isRecord(value.blueprint);
  if (wrapped)
    assertAllowedKeys(
      value,
      ['environment', 'blueprint', 'layout'],
      '최상위 객체',
    );
  const blueprintValue = wrapped
    ? (value.blueprint as Record<string, unknown>)
    : value;
  assertBlueprintShape(blueprintValue);
  const blueprint = blueprintValue as Blueprint;
  const layout =
    value.layout === undefined
      ? createDefaultLayout(blueprint)
      : parseLayout(value.layout, blueprint);

  try {
    const graph = fromAPI(blueprint, layout);
    assertRoundTripPreservesBlueprint(blueprint, graph);
    return { environment, graph };
  } catch (cause) {
    const detail =
      cause instanceof Error ? cause.message : '지원하지 않는 구조입니다.';
    throw new Error(`현재 에디터에서 열 수 없는 토폴로지입니다: ${detail}`);
  }
}

function assertRoundTripPreservesBlueprint(
  original: Blueprint,
  graph: ImportedTopology['graph'],
): void {
  const encoded = toAPI(
    graph.nodes,
    graph.edges,
    graph.viewport,
    graph.deployment,
  ).blueprint;
  if (
    stableJSON(normalizeBlueprint(encoded)) !==
    stableJSON(normalizeBlueprint(original))
  ) {
    throw new Error(
      '지원하지 않는 필드가 있거나 가져오는 과정에서 보존할 수 없는 구조입니다.',
    );
  }
}

function normalizeBlueprint(value: Blueprint): Blueprint {
  return {
    ...value,
    nodes: [...value.nodes].sort((a, b) => a.id.localeCompare(b.id)),
    placements: [...value.placements].sort((a, b) =>
      `${a.workloadRef}/${a.targetRef}`.localeCompare(
        `${b.workloadRef}/${b.targetRef}`,
      ),
    ),
    connections: [...value.connections].sort((a, b) =>
      a.id.localeCompare(b.id),
    ),
  };
}

function stableJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stableJSON).join(',')}]`;
  if (isRecord(value)) {
    return `{${Object.keys(value)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${stableJSON(value[key])}`)
      .join(',')}}`;
  }
  return JSON.stringify(value);
}

function parseEnvironment(value: unknown): Environment | undefined {
  if (value === undefined) return undefined;
  if (value === 'dev' || value === 'qa' || value === 'prod') return value;
  throw new Error('environment는 dev, qa, prod 중 하나여야 합니다.');
}

function assertBlueprintShape(value: Record<string, unknown>): void {
  if (value.apiVersion !== 'studio.deployer/v1') {
    throw new Error('지원하는 apiVersion은 studio.deployer/v1입니다.');
  }
  for (const field of ['nodes', 'placements', 'connections'] as const) {
    if (!Array.isArray(value[field])) {
      throw new Error(`blueprint.${field} 배열이 필요합니다.`);
    }
  }
}

function parseLayout(value: unknown, blueprint: Blueprint): Layout {
  if (
    !isRecord(value) ||
    !Array.isArray(value.nodes) ||
    !isViewport(value.viewport)
  ) {
    throw new Error('layout에는 nodes 배열과 올바른 viewport가 필요합니다.');
  }
  assertAllowedKeys(value, ['nodes', 'connections', 'viewport'], 'layout');
  assertAllowedKeys(value.viewport, ['x', 'y', 'zoom'], 'layout.viewport');
  const expected = new Set(blueprint.nodes.map((node) => node.id));
  const actual = new Set<string>();
  for (const [index, item] of value.nodes.entries()) {
    if (
      !isRecord(item) ||
      typeof item.nodeId !== 'string' ||
      !isPosition(item.position)
    ) {
      throw new Error(
        `layout.nodes[${index}]의 nodeId와 position이 올바르지 않습니다.`,
      );
    }
    assertAllowedKeys(item, ['nodeId', 'position'], `layout.nodes[${index}]`);
    assertAllowedKeys(
      item.position,
      ['x', 'y'],
      `layout.nodes[${index}].position`,
    );
    if (!expected.has(item.nodeId) || actual.has(item.nodeId)) {
      throw new Error(
        `layout.nodes[${index}].nodeId가 없거나 중복된 노드를 참조합니다.`,
      );
    }
    actual.add(item.nodeId);
  }
  if (actual.size !== expected.size) {
    throw new Error(
      'layout에는 모든 blueprint 노드의 위치가 한 번씩 있어야 합니다.',
    );
  }
  if (value.connections !== undefined) {
    if (!Array.isArray(value.connections)) {
      throw new Error('layout.connections 배열이 필요합니다.');
    }
    const expectedConnections = new Set(
      blueprint.connections.map((connection) => connection.id),
    );
    const actualConnections = new Set<string>();
    for (const [index, item] of value.connections.entries()) {
      if (!isRecord(item) || typeof item.connectionId !== 'string') {
        throw new Error(
          `layout.connections[${index}].connectionId가 올바르지 않습니다.`,
        );
      }
      assertAllowedKeys(
        item,
        ['connectionId', 'bendOffset', 'labelPosition', 'labelOffset'],
        `layout.connections[${index}]`,
      );
      for (const field of ['bendOffset', 'labelOffset'] as const) {
        if (item[field] !== undefined && !isPosition(item[field])) {
          throw new Error(
            `layout.connections[${index}].${field}가 올바르지 않습니다.`,
          );
        }
      }
      if (
        item.labelPosition !== undefined &&
        (typeof item.labelPosition !== 'number' ||
          !Number.isFinite(item.labelPosition) ||
          item.labelPosition < 0 ||
          item.labelPosition > 1)
      ) {
        throw new Error(
          `layout.connections[${index}].labelPosition은 0 이상 1 이하여야 합니다.`,
        );
      }
      if (
        (!item.bendOffset &&
          item.labelPosition === undefined &&
          !item.labelOffset) ||
        !expectedConnections.has(item.connectionId) ||
        actualConnections.has(item.connectionId)
      ) {
        throw new Error(
          `layout.connections[${index}]가 없거나 중복된 연결을 참조합니다.`,
        );
      }
      actualConnections.add(item.connectionId);
    }
  }
  return value as Layout;
}

function createDefaultLayout(blueprint: Blueprint): Layout {
  const placements = new Map(
    blueprint.placements.map((placement) => [placement.workloadRef, placement]),
  );
  const targets = blueprint.nodes.filter(
    (node) => node.kind === 'vm' || node.kind === 'kubernetes',
  );
  const targetIndex = new Map(targets.map((node, index) => [node.id, index]));
  const childCounts = new Map<string, number>();
  let standaloneIndex = 0;

  return {
    nodes: blueprint.nodes.map((node) => {
      const placement = placements.get(node.id);
      if (placement) {
        const index = childCounts.get(placement.targetRef) ?? 0;
        childCounts.set(placement.targetRef, index + 1);
        return {
          nodeId: node.id,
          position: { x: 24, y: 80 + index * 170 },
        };
      }
      const index = targetIndex.get(node.id);
      if (index !== undefined) {
        return {
          nodeId: node.id,
          position: {
            x: 40 + (index % 3) * 400,
            y: 60 + Math.floor(index / 3) * 520,
          },
        };
      }
      const current = standaloneIndex++;
      return {
        nodeId: node.id,
        position: {
          x: 80 + (current % 3) * 300,
          y: 700 + Math.floor(current / 3) * 200,
        },
      };
    }),
    viewport: { x: 0, y: 0, zoom: 0.8 },
  };
}

function isViewport(value: unknown): value is Layout['viewport'] {
  return (
    isRecord(value) &&
    typeof value.x === 'number' &&
    Number.isFinite(value.x) &&
    typeof value.y === 'number' &&
    Number.isFinite(value.y) &&
    typeof value.zoom === 'number' &&
    Number.isFinite(value.zoom) &&
    value.zoom > 0
  );
}

function isPosition(value: unknown): value is { x: number; y: number } {
  return (
    isRecord(value) &&
    typeof value.x === 'number' &&
    Number.isFinite(value.x) &&
    typeof value.y === 'number' &&
    Number.isFinite(value.y)
  );
}

function assertAllowedKeys(
  value: Record<string, unknown>,
  allowed: string[],
  label: string,
): void {
  const allowedSet = new Set(allowed);
  const unknown = Object.keys(value).find((key) => !allowedSet.has(key));
  if (unknown) throw new Error(`${label}.${unknown} 필드는 지원하지 않습니다.`);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
