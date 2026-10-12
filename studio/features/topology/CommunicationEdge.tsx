'use client';

import {
  BaseEdge,
  EdgeLabelRenderer,
  useReactFlow,
  type EdgeProps,
} from '@xyflow/react';
import { useLayoutEffect, useRef, useState } from 'react';
import {
  communicationEdgeGeometry,
  constrainedBendOffset,
  DEFAULT_LABEL_POSITION,
  midpoint,
  nearestPathPosition,
  pointOnPath,
} from './communication-edge-layout';
import type {
  ConnectionVisualLayout,
  EdgePoint,
  TopologyEdge,
  TopologyNode,
} from './model';
import { usePointerDrag } from './usePointerDrag';

export default function CommunicationEdge(props: EdgeProps<TopologyEdge>) {
  const {
    id,
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
    markerEnd,
    label,
    data,
    selected,
  } = props;
  const { screenToFlowPosition } = useReactFlow<TopologyNode, TopologyEdge>();
  const source = { x: sourceX, y: sourceY };
  const target = { x: targetX, y: targetY };
  const center = midpoint(source, target);
  const geometry = communicationEdgeGeometry(
    source,
    target,
    data?.layout,
    sourcePosition,
    targetPosition,
  );
  const pathRef = useRef<SVGPathElement>(null);
  const labelPosition = data?.layout?.labelPosition ?? DEFAULT_LABEL_POSITION;
  const [labelPoint, setLabelPoint] = useState<EdgePoint>(geometry.bend);

  useLayoutEffect(() => {
    if (!pathRef.current) return;
    const next = pointOnPath(pathRef.current, labelPosition);
    if (!next) return;
    setLabelPoint((current) =>
      current.x === next.x && current.y === next.y ? current : next,
    );
  }, [geometry.path, labelPosition]);

  function updateLayout(change: Partial<ConnectionVisualLayout>) {
    data?.onLayoutChange?.({ ...data.layout, ...change });
  }

  const bendInteraction = usePointerDrag({
    onDrag: (screenPoint) => {
      const point = screenToFlowPosition(screenPoint);
      updateLayout({
        bendOffset: constrainedBendOffset(sourcePosition, center, point),
      });
    },
    onClick: data?.onSelect,
  });
  const labelInteraction = usePointerDrag({
    onDrag: (screenPoint) => {
      if (!pathRef.current) return;
      const position = nearestPathPosition(
        pathRef.current,
        screenToFlowPosition(screenPoint),
      );
      if (position !== undefined) {
        data?.onLayoutChange?.({
          bendOffset: data.layout?.bendOffset,
          labelPosition: position,
        });
      }
    },
    onClick: data?.onSelect,
  });
  const displayLabel =
    label ?? (data?.port ? `${data.protocol ?? 'TCP'} · ${data.port}` : '');
  const accessibleLabel =
    typeof displayLabel === 'string' || typeof displayLabel === 'number'
      ? `${displayLabel} 라벨 위치 이동`
      : '통신 라벨 위치 이동';

  return (
    <>
      <BaseEdge
        id={id}
        path={geometry.path}
        markerEnd={markerEnd}
        style={{
          stroke: selected ? '#256754' : '#a5bcad',
          strokeWidth: selected ? 2 : 1.5,
        }}
      />
      <path
        ref={pathRef}
        d={geometry.path}
        fill="none"
        stroke="transparent"
        pointerEvents="none"
        aria-hidden="true"
      />
      <EdgeLabelRenderer>
        <button
          type="button"
          aria-label="통신선 굴곡점 이동"
          title="드래그: 통신선 이동 · 더블클릭: 초기화"
          className={`nodrag nopan pointer-events-auto absolute size-12 -translate-x-1/2 -translate-y-1/2 cursor-move rounded-full transition-opacity ${selected ? 'opacity-100' : 'opacity-60 hover:opacity-100'}`}
          style={{
            left: geometry.bend.x,
            top: geometry.bend.y,
            zIndex: 10,
            background:
              'radial-gradient(circle, #6f9b84 0 6px, white 6px 8px, transparent 8px)',
          }}
          onDoubleClick={(event) => {
            event.stopPropagation();
            updateLayout({ bendOffset: undefined });
          }}
          {...bendInteraction}
        />
        {displayLabel && (
          <button
            type="button"
            aria-label={accessibleLabel}
            title="드래그: 선을 따라 포트 라벨 이동 · 더블클릭: 초기화"
            className="nodrag nopan pointer-events-auto absolute -translate-x-1/2 -translate-y-1/2 cursor-move rounded-md border border-[#dce5df] bg-white/95 px-2 py-1 font-mono text-[10px] whitespace-nowrap text-[#667c70] shadow-sm"
            style={{
              left: labelPoint?.x ?? geometry.bend.x,
              top: labelPoint?.y ?? geometry.bend.y,
              zIndex: 10,
            }}
            onDoubleClick={(event) => {
              event.stopPropagation();
              data?.onLayoutChange?.({ bendOffset: data.layout?.bendOffset });
            }}
            {...labelInteraction}
          >
            {displayLabel}
          </button>
        )}
      </EdgeLabelRenderer>
    </>
  );
}
