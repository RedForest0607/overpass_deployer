import assert from 'node:assert/strict';
import { getEventListeners } from 'node:events';
import { test } from 'node:test';
import { abortableDelay, resolveGeneratedPackage } from './package-polling.ts';

const result = {
  id: 'pkg-1',
  generator: 'crosswalk-compose',
  generatorVersion: '1.0.0',
  chartFormat: 'crosswalk/v1',
};
const done = { id: 'gen-1', status: 'succeeded', packageId: result.id };

void test('delay rejects an already-aborted signal and removes completed listeners', async () => {
  const aborted = new AbortController();
  aborted.abort();
  await assert.rejects(abortableDelay(10000, aborted.signal), {
    name: 'AbortError',
  });
  const current = new AbortController();
  for (let i = 0; i < 3; i++) await abortableDelay(1, current.signal);
  assert.equal(getEventListeners(current.signal, 'abort').length, 0);
});

void test('polling waits for success and checks the expected generator contract', async () => {
  const statuses = [{ id: 'gen-1', status: 'running' }, done];
  const progress = [],
    delays = [];
  const api = {
    async getGeneration(project, id) {
      assert.equal(project, 'project-1');
      assert.equal(id, 'gen-1');
      return statuses.shift();
    },
    async getPackage(project, id) {
      assert.equal(project, 'project-1');
      assert.equal(id, result.id);
      return result;
    },
  };
  const value = await resolveGeneratedPackage(
    api,
    'project-1',
    { id: 'gen-1', status: 'queued' },
    new AbortController().signal,
    'crosswalk-compose',
    (value) => progress.push(value.status),
    async (ms) => {
      delays.push(ms);
    },
  );
  assert.equal(value, result);
  assert.deepEqual(progress, ['running', 'succeeded']);
  assert.deepEqual(delays, [1000, 2000]);
});

void test('late generation responses cannot publish progress after cancellation', async () => {
  const controller = new AbortController();
  let published = false,
    packageRead = false;
  const api = {
    async getGeneration() {
      controller.abort();
      return done;
    },
    async getPackage() {
      packageRead = true;
      return result;
    },
  };
  await assert.rejects(
    resolveGeneratedPackage(
      api,
      'project-1',
      { id: 'gen-1', status: 'queued' },
      controller.signal,
      'crosswalk-compose',
      () => {
        published = true;
      },
      async () => {},
    ),
    { name: 'AbortError' },
  );
  assert.equal(published, false);
  assert.equal(packageRead, false);
});

void test('late package responses cannot return a cancelled result', async () => {
  const controller = new AbortController();
  const api = {
    async getPackage() {
      controller.abort();
      return result;
    },
  };
  await assert.rejects(
    resolveGeneratedPackage(
      api,
      'project-1',
      done,
      controller.signal,
      'crosswalk-compose',
      () => {},
    ),
    { name: 'AbortError' },
  );
});

void test('failed jobs, missing package IDs and incompatible packages stay errors', async () => {
  const api = {
    async getPackage() {
      return { ...result, generator: 'crosswalk-vm' };
    },
  };
  for (const [generation, message] of [
    [
      { ...done, status: 'failed', error: { message: 'build failed' } },
      /build failed/,
    ],
    [{ ...done, packageId: undefined }, /package ID/],
    [done, /crosswalk-compose/],
  ]) {
    await assert.rejects(
      resolveGeneratedPackage(
        api,
        'project-1',
        generation,
        new AbortController().signal,
        'crosswalk-compose',
        () => {},
      ),
      message,
    );
  }
});
