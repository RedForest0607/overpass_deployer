import { getSmoothStepPath, Position } from '@xyflow/react';
import type { ConnectionVisualLayout, EdgePoint } from './model.ts';

export const DEFAULT_LABEL_POSITION = 0.65;

export type PathMetrics = {
  getTotalLength: () => number;
  getPointAtLength: (length: number) => EdgePoint;
};

export function communicationEdgeGeometry(
  source: EdgePoint,
  target: EdgePoint,
  layout?: ConnectionVisualLayout,
  sourcePosition: Position = Position.Right,
  targetPosition: Position = Position.Left,
) {
  const center = midpoint(source, target);
  const horizontal = isHorizontal(sourcePosition);
  const bendCenter = add(center, layout?.bendOffset);
  const [path, pathCenterX, pathCenterY] = getSmoothStepPath({
    sourceX: source.x,
    sourceY: source.y,
    sourcePosition,
    targetX: target.x,
    targetY: target.y,
    targetPosition,
    borderRadius: 8,
    ...(layout?.bendOffset
      ? horizontal
        ? { centerX: bendCenter.x }
        : { centerY: bendCenter.y }
      : {}),
  });
  const bend = { x: pathCenterX, y: pathCenterY };
  return { bend, path };
}

export function pointOnPath(
  path: PathMetrics,
  position: number,
): EdgePoint | undefined {
  const totalLength = path.getTotalLength();
  if (!Number.isFinite(totalLength) || totalLength <= 0) return undefined;
  return path.getPointAtLength(totalLength * clampPosition(position));
}

export function nearestPathPosition(
  path: PathMetrics,
  point: EdgePoint,
): number | undefined {
  const totalLength = path.getTotalLength();
  if (!Number.isFinite(totalLength) || totalLength <= 0) return undefined;

  const samples = 40;
  let bestPosition = 0;
  let bestDistance = Number.POSITIVE_INFINITY;
  for (let index = 0; index <= samples; index++) {
    const position = index / samples;
    const distance = squaredDistance(
      path.getPointAtLength(totalLength * position),
      point,
    );
    if (distance < bestDistance) {
      bestDistance = distance;
      bestPosition = position;
    }
  }

  let start = Math.max(0, bestPosition - 1 / samples);
  let end = Math.min(1, bestPosition + 1 / samples);
  for (let index = 0; index < 8; index++) {
    const left = start + (end - start) / 3;
    const right = end - (end - start) / 3;
    const leftDistance = squaredDistance(
      path.getPointAtLength(totalLength * left),
      point,
    );
    const rightDistance = squaredDistance(
      path.getPointAtLength(totalLength * right),
      point,
    );
    if (leftDistance <= rightDistance) end = right;
    else start = left;
  }
  return (start + end) / 2;
}

export function constrainedBendOffset(
  sourcePosition: Position,
  center: EdgePoint,
  point: EdgePoint,
): EdgePoint {
  const offset = pointOffset(center, point);
  return isHorizontal(sourcePosition)
    ? { x: offset.x, y: 0 }
    : { x: 0, y: offset.y };
}

export function pointOffset(origin: EdgePoint, point: EdgePoint): EdgePoint {
  return { x: point.x - origin.x, y: point.y - origin.y };
}

export function midpoint(first: EdgePoint, second: EdgePoint): EdgePoint {
  return { x: (first.x + second.x) / 2, y: (first.y + second.y) / 2 };
}

function add(point: EdgePoint, offset?: EdgePoint): EdgePoint {
  return {
    x: point.x + (offset?.x ?? 0),
    y: point.y + (offset?.y ?? 0),
  };
}

function isHorizontal(position: Position) {
  return position === Position.Left || position === Position.Right;
}

function clampPosition(value: number) {
  if (!Number.isFinite(value)) return DEFAULT_LABEL_POSITION;
  return Math.min(1, Math.max(0, value));
}

function squaredDistance(first: EdgePoint, second: EdgePoint) {
  return (first.x - second.x) ** 2 + (first.y - second.y) ** 2;
}
