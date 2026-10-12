import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ArtifactRequests } from './artifact-requests.ts';

test('late responses stay stale across A to B to A project switches', () => {
  const scope = new ArtifactRequests();
  const a = scope.begin('A', 'list');
  scope.cancel();
  const b = scope.begin('B', 'list');
  scope.cancel();
  const nextA = scope.begin('A', 'list');
  assert.equal(scope.isCurrent(a), false);
  assert.equal(scope.isCurrent(b), false);
  assert.equal(scope.isCurrent(nextA), true);
});

test('manual refresh cancels older refresh and stale completion cannot unlock it', () => {
  const scope = new ArtifactRequests();
  const old = scope.begin('A', 'list');
  const current = scope.begin('A', 'list');
  scope.finish(old);
  assert.equal(old.controller.signal.aborted, true);
  assert.equal(scope.isCurrent(current), true);
});

test('upload is synchronously serialized and disposed responses are ignored', () => {
  const scope = new ArtifactRequests();
  const upload = scope.begin('A', 'upload');
  assert.equal(scope.begin('A', 'upload'), null);
  assert.equal(scope.begin('A', 'list'), null);
  scope.cancel();
  assert.equal(scope.isCurrent(upload), false);
  assert.equal(upload.controller.signal.aborted, true);
});
