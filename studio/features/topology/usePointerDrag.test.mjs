import assert from 'node:assert/strict';
import test from 'node:test';
import { exceedsDragThreshold } from './usePointerDrag.ts';

test('distinguishes a click from a pointer drag', () => {
  assert.equal(
    exceedsDragThreshold({ x: 100, y: 100 }, { x: 103, y: 103 }, 5),
    false,
  );
  assert.equal(
    exceedsDragThreshold({ x: 100, y: 100 }, { x: 106, y: 100 }, 5),
    true,
  );
});
