'use client';

import { useCallback, type Dispatch, type SetStateAction } from 'react';
import { addEdge, useReactFlow, type Connection } from '@xyflow/react';
import {
  createNode,
  connectionType,
  type Kind,
  type TopologyNode,
  type TopologyEdge,
} from '../topology/model';
import {
  isTarget,
  isWorkload,
  placeWorkload,
  removeResource,
  layoutPlacement,
} from '../topology/placement';

type Selection = { type: 'node' | 'edge'; id: string } | null;
type Graph = { nodes: TopologyNode[]; edges: TopologyEdge[] };
type Setter<T> = Dispatch<SetStateAction<T>>;

export function useTopologyEditor({
  nodes,
  edges,
  selection,
  setNodes,
  setEdges,
  setSelection,
  setMessage,
  setLastRemoval,
  setPaletteOpen,
}: Graph & {
  selection: Selection;
  setNodes: Setter<TopologyNode[]>;
  setEdges: Setter<TopologyEdge[]>;
  setSelection: Setter<Selection>;
  setMessage: Setter<string>;
  setLastRemoval: Setter<Graph | null>;
  setPaletteOpen: Setter<boolean>;
}) {
  const flow = useReactFlow();
  function place(id: string, targetId?: string) {
    const next = placeWorkload(nodes, edges, id, targetId);
    setNodes(next.nodes);
    setEdges(next.edges);
  }
  const valid = useCallback(
    (c: Connection | TopologyEdge) => {
      const from = nodes.find((n) => n.id === c.source),
        to = nodes.find((n) => n.id === c.target);
      return (
        !!from &&
        !!to &&
        from.id !== to.id &&
        !!connectionType(from.data.kind, to.data.kind) &&
        !edges.some((e) => e.source === c.source && e.target === c.target)
      );
    },
    [nodes, edges],
  );
  function connect(c: Connection) {
    if (!valid(c)) {
      setMessage('이 노드 조합은 연결할 수 없습니다.');
      return;
    }
    const from = nodes.find((n) => n.id === c.source)!,
      to = nodes.find((n) => n.id === c.target)!;
    const relation = connectionType(from.data.kind, to.data.kind)!;
    if (relation === 'deploys_to') {
      place(from.id, to.id);
      return;
    }
    const network = ['connects_to', 'routes_to'].includes(relation);
    setEdges((es) =>
      addEdge(
        {
          ...c,
          type: 'communication',
          label: network ? 'TCP · ' + to.data.port : relation,
          data: network ? { protocol: 'TCP', port: to.data.port } : undefined,
        },
        es,
      ),
    );
  }
  function add(kind: Kind, position?: { x: number; y: number }) {
    const id = crypto.randomUUID();
    const node = createNode(
      kind,
      id,
      position?.x ?? 550,
      position?.y ?? 120 + (nodes.length % 3) * 190,
    );
    const targets = nodes.filter(isTarget);
    const target = position
      ? targets.find(
          (n) =>
            position.x >= n.position.x &&
            position.x <= n.position.x + Number(n.style?.width ?? 340) &&
            position.y >= n.position.y &&
            position.y <= n.position.y + Number(n.style?.height ?? 300),
        )
      : targets.length === 1
        ? targets[0]
        : undefined;
    const next =
      isWorkload(node) && target
        ? placeWorkload([...nodes, node], edges, id, target.id)
        : { nodes: layoutPlacement([...nodes, node]), edges };
    setNodes(next.nodes);
    setEdges(next.edges);
    setMessage('노드를 추가했습니다. 더블클릭해 설정하세요.');
    setPaletteOpen(false);
    setTimeout(() => flow.fitView({ padding: 0.25, duration: 250 }), 50);
  }
  function remove() {
    if (!selection) return;
    setLastRemoval({ nodes, edges });
    if (selection.type === 'node') {
      const next = removeResource(nodes, edges, selection.id);
      setNodes(next.nodes);
      setEdges(next.edges);
    } else setEdges((es) => es.filter((e) => e.id !== selection.id));
    setSelection(null);
    setMessage('삭제했습니다. 실행 취소로 복구할 수 있습니다.');
  }

  function updateNode(data: Partial<TopologyNode['data']>) {
    setNodes((current) =>
      current.map((node) =>
        node.id === selection?.id
          ? { ...node, data: { ...node.data, ...data } }
          : node,
      ),
    );
    if (typeof data.port === 'number') {
      const port = data.port;
      setEdges((current) =>
        current.map((edge) =>
          edge.target === selection?.id && edge.data?.port !== undefined
            ? { ...edge, data: { ...edge.data, port }, label: 'TCP · ' + port }
            : edge,
        ),
      );
    }
  }

  return { place, valid, connect, add, remove, updateNode };
}
