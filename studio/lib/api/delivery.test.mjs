import assert from 'node:assert/strict';
import test from 'node:test';
import { deliveryAPI } from './delivery.ts';

void test('artifact listing follows project-scoped cursors', async (t) => {
  const calls = [];
  t.mock.method(globalThis, 'fetch', async (url) => {
    calls.push(String(url));
    const body = String(url).includes('cursor=next')
      ? { items: [{ id: 'two' }], nextCursor: null }
      : { items: [{ id: 'one' }], nextCursor: 'next' };
    return new Response(JSON.stringify(body));
  });
  const values = await deliveryAPI.listArtifacts('project/a');
  assert.deepEqual(
    values.map((value) => value.id),
    ['one', 'two'],
  );
  assert.match(calls[0], /projects\/project%2Fa\/artifacts/);
});

void test('generation sends an idempotency key with immutable revision', async (t) => {
  let call;
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    call = { url: String(url), options };
    return new Response(JSON.stringify({ id: 'gen-1', status: 'succeeded' }));
  });
  await deliveryAPI.createGeneration(
    'project-1',
    'qa',
    7,
    'package',
    'retry-key',
  );
  assert.equal(
    new Headers(call.options.headers).get('Idempotency-Key'),
    'retry-key',
  );
  assert.deepEqual(JSON.parse(call.options.body), {
    environment: 'qa',
    revision: 7,
    generator: 'crosswalk-vm',
    output: 'package',
  });
});

void test('Docker validation and generation send the selected generator', async (t) => {
  const bodies = [];
  t.mock.method(globalThis, 'fetch', async (_url, options) => {
    bodies.push(JSON.parse(options.body));
    return new Response(JSON.stringify({ id: 'result', valid: true }));
  });
  await deliveryAPI.validatePackage(
    'project-1',
    'dev',
    {
      apiVersion: 'studio.deployer/v1',
      nodes: [],
      placements: [],
      connections: [],
    },
    'request-1',
    'crosswalk-docker',
  );
  await deliveryAPI.createGeneration(
    'project-1',
    'dev',
    2,
    'package',
    'docker-key',
    'crosswalk-docker',
  );
  assert.equal(bodies[0].generator, 'crosswalk-docker');
  assert.equal(bodies[1].generator, 'crosswalk-docker');
});

void test('Compose validation and generation send the selected generator', async (t) => {
  const bodies = [];
  t.mock.method(globalThis, 'fetch', async (_url, options) => {
    bodies.push(JSON.parse(options.body));
    return new Response(JSON.stringify({ id: 'result', valid: true }));
  });
  await deliveryAPI.validatePackage(
    'project-1',
    'dev',
    {
      apiVersion: 'studio.deployer/v1',
      nodes: [],
      placements: [],
      connections: [],
    },
    'request-compose',
    'crosswalk-compose',
  );
  await deliveryAPI.createGeneration(
    'project-1',
    'dev',
    3,
    'package',
    'compose-key',
    'crosswalk-compose',
  );
  assert.equal(bodies[0].generator, 'crosswalk-compose');
  assert.equal(bodies[1].generator, 'crosswalk-compose');
});

void test('generation history and retry keep project and snapshot source scoped', async (t) => {
  const calls = [];
  t.mock.method(globalThis, 'fetch', async (url, options = {}) => {
    calls.push({ url: String(url), options });
    return new Response(
      JSON.stringify(
        String(url).endsWith('/retry')
          ? { id: 'retry-1', retryOfGenerationId: 'failed/1' }
          : { items: [], nextCursor: 'next' },
      ),
      { status: String(url).endsWith('/retry') ? 202 : 200 },
    );
  });

  await deliveryAPI.listGenerations('project/1', 'qa', 'cursor/1');
  await deliveryAPI.retryGeneration('project/1', 'failed/1', 'retry-key');

  assert.match(
    calls[0].url,
    /projects\/project%2F1\/generations\?limit=10&environment=qa&cursor=cursor%2F1$/,
  );
  assert.match(
    calls[1].url,
    /projects\/project%2F1\/generations\/failed%2F1\/retry$/,
  );
  assert.equal(
    new Headers(calls[1].options.headers).get('Idempotency-Key'),
    'retry-key',
  );
});

void test('artifact upload uses multipart fields expected by the backend', async (t) => {
  let call;
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    call = { url: String(url), options };
    return new Response(JSON.stringify({ id: 'artifact-1' }), { status: 201 });
  });
  const file = new File(['binary'], 'app.jar');
  await deliveryAPI.uploadArtifact('project-1', 'jar', file, 'release.jar');
  assert.equal(call.options.body.get('kind'), 'jar');
  assert.equal(call.options.body.get('file'), file);
  assert.equal(call.options.body.get('displayName'), 'release.jar');
  assert.equal(new Headers(call.options.headers).has('Content-Type'), false);
});

void test('artifact content read stays within the encoded project and artifact path', async (t) => {
  let call;
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    call = { url: String(url), options };
    return new Response(
      JSON.stringify({
        id: 'artifact/1',
        content: 'server: {}',
        encoding: 'utf-8',
      }),
    );
  });

  const result = await deliveryAPI.getArtifactContent(
    'project/1',
    'artifact/1',
  );

  assert.match(
    call.url,
    /projects\/project%2F1\/artifacts\/artifact%2F1\/content$/,
  );
  assert.equal(result.content, 'server: {}');
});
