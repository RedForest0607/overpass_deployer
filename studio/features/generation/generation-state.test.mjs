import assert from 'node:assert/strict';
import test from 'node:test';
import { idempotencyAttempt, nextPollDelay } from './generation-state.ts';

void test('the same generation retry reuses its idempotency key', () => {
  let sequence = 0;
  const create = () => 'key-' + ++sequence;
  const first = idempotencyAttempt(null, 'project:dev:3:package', create);
  const retry = idempotencyAttempt(first, 'project:dev:3:package', create);
  const changed = idempotencyAttempt(first, 'project:dev:4:package', create);
  assert.equal(retry.key, first.key);
  assert.notEqual(changed.key, first.key);
});

void test('generation polling backs off and stays bounded', () => {
  assert.equal(nextPollDelay(1), 1000);
  assert.equal(nextPollDelay(3), 3000);
  assert.equal(nextPollDelay(20), 5000);
});
