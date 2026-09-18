import type { TopologyNode, TopologyEdge } from './model';

export const TARGET_WIDTH = 340;
export const SLOT_HEIGHT = 190;
export const isTarget = (node: TopologyNode) =>
  ['vm', 'kubernetes'].includes(node.data.kind);
export const isWorkload = (node: TopologyNode) =>
  ['java', 'container', 'redis', 'elasticsearch'].includes(node.data.kind);

export const canPlaceOn = (node: TopologyNode, target: TopologyNode) =>
  isTarget(target) &&
  (node.data.kind !== 'container' || target.data.kind === 'vm');

/** Parents precede children. Each service gets a separate slot inside its host. */
export function layoutPlacement(nodes: TopologyNode[]): TopologyNode[] {
  const targets = nodes.filter(isTarget);
  const children = nodes.filter((n) => n.parentId);
  return [
    ...targets.map((target) => ({
      ...target,
      style: {
        width: TARGET_WIDTH,
        height:
          110 +
          Math.max(1, children.filter((n) => n.parentId === target.id).length) *
            SLOT_HEIGHT,
      },
    })),
    ...nodes.filter((n) => !isTarget(n) && !n.parentId),
    ...children.map((node) => ({
      ...node,
      extent: 'parent' as const,
      position: {
        x: 50,
        y:
          100 +
          children
            .filter((n) => n.parentId === node.parentId)
            .findIndex((n) => n.id === node.id) *
            SLOT_HEIGHT,
      },
    })),
  ];
}

/** Placement is stored as parentId and a deployment relation, never a network edge. */
export function placeWorkload(
  nodes: TopologyNode[],
  edges: TopologyEdge[],
  id: string,
  targetId?: string,
) {
  const node = nodes.find((n) => n.id === id);
  const target = nodes.find((n) => n.id === targetId);
  if (
    !node ||
    !isWorkload(node) ||
    (targetId && (!target || !canPlaceOn(node, target)))
  )
    return { nodes, edges };
  const previous = nodes.find((n) => n.id === node.parentId);
  const nextNodes = nodes.map((n) =>
    n.id !== id
      ? n
      : {
          ...n,
          parentId: targetId,
          extent: targetId ? ('parent' as const) : undefined,
          position: targetId
            ? { x: 50, y: 100 }
            : {
                x: n.position.x + (previous?.position.x ?? 0),
                y: n.position.y + (previous?.position.y ?? 0),
              },
        },
  );
  const nextEdges = edges.filter(
    (e) =>
      !(e.source === id && nodes.some((n) => n.id === e.target && isTarget(n))),
  );
  if (targetId)
    nextEdges.push({
      id: id + '-placement',
      source: id,
      target: targetId,
      label: 'deploys_to',
      type: 'smoothstep',
    });
  return { nodes: layoutPlacement(nextNodes), edges: nextEdges };
}

/** Removing a host preserves its services and their absolute canvas positions. */
export function removeResource(
  nodes: TopologyNode[],
  edges: TopologyEdge[],
  id: string,
) {
  const removed = nodes.find((n) => n.id === id);
  return {
    nodes: layoutPlacement(
      nodes
        .filter((n) => n.id !== id)
        .map((n) =>
          n.parentId !== id
            ? n
            : {
                ...n,
                parentId: undefined,
                extent: undefined,
                position: {
                  x: n.position.x + (removed?.position.x ?? 0),
                  y: n.position.y + (removed?.position.y ?? 0),
                },
              },
        ),
    ),
    edges: edges.filter((e) => e.source !== id && e.target !== id),
  };
}
