'use client';
import { Handle, Position, type NodeProps } from '@xyflow/react';
import { ResourceIcon } from './ResourceNode';
import type { TopologyNode } from './model';

export default function TargetNode({
  data,
  selected,
}: NodeProps<TopologyNode>) {
  return (
    <div className={'target-node ' + (selected ? 'chosen' : '')}>
      <Handle type="target" position={Position.Left} />
      <div className="target-heading">
        <ResourceIcon kind={data.kind} />
        <div>
          <small>{data.kind === 'vm' ? data.platform : 'KUBERNETES'}</small>
          <strong>{data.label}</strong>
        </div>
        <code>{data.host || data.namespace}</code>
      </div>
      <div className="target-hint">
        실행 서비스 · Java / Container / Redis / Elasticsearch
        <br />
        라이브러리에서 이 영역으로 드래그하세요
      </div>
    </div>
  );
}
