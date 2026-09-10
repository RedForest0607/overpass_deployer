'use client';
import { Handle, Position, type NodeProps } from '@xyflow/react';
import {
  Coffee,
  Server,
  Database,
  Layers,
  Container,
  Globe,
  Box,
  MoreHorizontal,
  Search,
} from 'lucide-react';
import { catalog, type TopologyNode, type Kind } from './model';
export const icons = {
  java: Coffee,
  vm: Server,
  database: Database,
  redis: Layers,
  kubernetes: Container,
  registry: Box,
  ingress: Globe,
  elasticsearch: Search,
};
export function ResourceIcon({
  kind,
  size = 20,
}: {
  kind: Kind;
  size?: number;
}) {
  const Icon = icons[kind];
  return <Icon size={size} />;
}
export default function ResourceNode({
  data,
  selected,
  parentId,
}: NodeProps<TopologyNode>) {
  const item = catalog.find((c) => c.kind === data.kind)!;
  return (
    <div
      className={'resource-node ' + (selected ? 'chosen' : '')}
      style={{ '--node-accent': item.color } as React.CSSProperties}
    >
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
      <div className="resource-top">
        <span className="resource-icon">
          <ResourceIcon kind={data.kind} />
        </span>
        <span className="node-kind">
          {data.kind === 'java' ? 'APPLICATION' : data.kind.toUpperCase()}
        </span>
        <MoreHorizontal size={16} />
      </div>
      <strong>{data.label}</strong>
      <p>
        {data.kind === 'java'
          ? 'Java ' + data.java + ' · Spring Boot'
          : parentId
            ? '내부 실행 서비스'
            : data.host || item.subtitle}
      </p>
      <div className="resource-bottom">
        <span className="tiny-dot" />
        <code>
          {data.port
            ? ':' + data.port + ' / TCP'
            : data.kind === 'vm'
              ? 'Linux · VM target'
              : item.subtitle}
        </code>
      </div>
    </div>
  );
}
