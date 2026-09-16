import assert from 'node:assert/strict';
import test from 'node:test';
import {
  communicationEdgeGeometry,
  constrainedBendOffset,
  midpoint,
  nearestPathPosition,
  pointOnPath,
  pointOffset,
} from './communication-edge-layout.ts';
import { Position } from '@xyflow/react';

test('keeps a rounded smooth-step line while moving its bend', () => {
  const geometry = communicationEdgeGeometry(
    { x: 0, y: 20 },
    { x: 100, y: 60 },
    { bendOffset: { x: 10, y: -5 } },
    Position.Right,
    Position.Left,
  );

  assert.deepEqual(geometry.bend, { x: 60, y: 40 });
  assert.match(geometry.path, /^M0 20/);
  assert.match(geometry.path, /Q/);
  assert.match(geometry.path, /L100 60$/);
});

test('constrains bend movement to the smooth-step routing axis', () => {
  assert.deepEqual(
    constrainedBendOffset(Position.Right, { x: 50, y: 50 }, { x: 80, y: 10 }),
    { x: 30, y: 0 },
  );
  assert.deepEqual(
    constrainedBendOffset(Position.Bottom, { x: 50, y: 50 }, { x: 80, y: 10 }),
    { x: 0, y: -40 },
  );
});

test('calculates stable offsets for persistence', () => {
  assert.deepEqual(midpoint({ x: 10, y: 20 }, { x: 50, y: 100 }), {
    x: 30,
    y: 60,
  });
  assert.deepEqual(pointOffset({ x: 30, y: 60 }, { x: 45, y: 55 }), {
    x: 15,
    y: -5,
  });
});

test('projects a dragged label onto the communication path', () => {
  const horizontalPath = {
    getTotalLength: () => 100,
    getPointAtLength: (length) => ({ x: length, y: 20 }),
  };

  assert.deepEqual(pointOnPath(horizontalPath, 0.7), { x: 70, y: 20 });
  const position = nearestPathPosition(horizontalPath, { x: 32, y: 80 });
  assert.ok(Math.abs(position - 0.32) < 0.01);
});
