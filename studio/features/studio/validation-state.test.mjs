import test from 'node:test';
import assert from 'node:assert/strict';
import { isCurrentValidation } from './validation-state.ts';

test('only the matching response for the current graph can be shown', () => {
  assert.equal(
    isCurrentValidation('graph1', 'graph1', 'request1', 'request1'),
    true,
  );
  assert.equal(
    isCurrentValidation('graph1', 'graph2', 'request1', 'request1'),
    false,
  );
  assert.equal(
    isCurrentValidation('graph1', 'graph1', 'request1', 'request0'),
    false,
  );
});
