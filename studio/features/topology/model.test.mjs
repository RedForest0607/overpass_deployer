import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sample, createNode, validate, connectionType } from './model.ts';
import { placeWorkload, removeResource } from './placement.ts';

void test('sample topology passes validation', () => {
  const { nodes, edges } = sample();
  assert.deepEqual(validate(nodes, edges), []);
});
void test('rejects invalid ports, disconnected workloads and duplicate names', () => {
  const { nodes, edges } = sample();
  nodes.find((n) => n.id === 'app').data.port = 70000;
  nodes.find((n) => n.id === 'vm').data.label = nodes.find(
    (n) => n.id === 'app',
  ).data.label;
  const issues = validate(
    nodes,
    edges.filter((e) => e.id !== 'app-vm'),
  );
  assert.ok(issues.some((i) => i.id === 'appport'));
  assert.ok(issues.some((i) => i.id === 'apptarget'));
  assert.ok(issues.some((i) => i.id === 'appduplicate'));
});
void test('rejects forbidden connections and dangling references', () => {
  assert.equal(connectionType('database', 'vm'), null);
  const { nodes, edges } = sample();
  edges.push({ id: 'dangling', source: 'app', target: 'missing' });
  assert.ok(validate(nodes, edges).some((i) => i.id === 'dangling'));
});
void test('detects same-host application port collision and stale network binding', () => {
  const { nodes, edges } = sample();
  const second = createNode('java', 'second', 0, 0);
  nodes.push(second);
  edges.push({ id: 'second-vm', source: 'second', target: 'vm' });
  nodes.find((n) => n.id === 'db').data.port = 5433;
  const issues = validate(nodes, edges);
  assert.ok(issues.some((i) => i.id === 'secondconflict'));
  assert.ok(issues.some((i) => i.id === 'app-dbbinding'));
});
void test('multiple Java and search services share one host with independent slots', () => {
  let { nodes, edges } = sample();
  const app = createNode('java', 'second', 0, 0);
  app.data.port = 8081;
  app.data.artifact = 'second.jar';
  app.data.artifactRef = 'artifact-second';
  const search = createNode('elasticsearch', 'search', 0, 0);
  nodes.push(app, search);
  ({ nodes, edges } = placeWorkload(nodes, edges, 'second', 'vm'));
  ({ nodes, edges } = placeWorkload(nodes, edges, 'search', 'vm'));
  assert.deepEqual(validate(nodes, edges), []);
  const children = nodes.filter((n) => n.parentId === 'vm');
  assert.equal(children.length, 4);
  assert.equal(new Set(children.map((n) => n.position.y)).size, 4);
  assert.ok(
    children.every(
      (n) => nodes.indexOf(n) > nodes.findIndex((n) => n.id === 'vm'),
    ),
  );
});
void test('moving a service replaces placement and deleting the target preserves services', () => {
  let { nodes, edges } = sample();
  const host = createNode('vm', 'ec2', 800, 0);
  host.data.host = '10.0.0.2';
  host.data.platform = 'AWS EC2';
  nodes.push(host);
  ({ nodes, edges } = placeWorkload(nodes, edges, 'app', 'ec2'));
  assert.equal(nodes.find((n) => n.id === 'app').parentId, 'ec2');
  assert.equal(
    edges.filter((e) => e.source === 'app' && ['vm', 'ec2'].includes(e.target))
      .length,
    1,
  );
  assert.deepEqual(validate(nodes, edges), []);
  ({ nodes, edges } = removeResource(nodes, edges, 'ec2'));
  assert.ok(nodes.some((n) => n.id === 'app' && !n.parentId));
  assert.ok(edges.some((e) => e.id === 'app-db'));
  assert.ok(validate(nodes, edges).some((i) => i.id === 'apptarget'));
  assert.ok(nodes.every((n) => n.parentId !== 'ec2'));
});
void test('hosted Redis needs no endpoint, external Redis does; mixed service ports collide', () => {
  let { nodes, edges } = sample();
  assert.deepEqual(validate(nodes, edges), []);
  nodes.find((n) => n.id === 'redis').data.port = 8080;
  assert.ok(validate(nodes, edges).some((i) => i.id.endsWith('conflict')));
  ({ nodes, edges } = placeWorkload(nodes, edges, 'redis'));
  assert.ok(validate(nodes, edges).some((i) => i.id === 'redishost'));
});
void test('rejects inconsistent visual parent and deployment relationship', () => {
  const { nodes, edges } = sample();
  nodes.find((n) => n.id === 'app').parentId = 'missing';
  assert.ok(validate(nodes, edges).some((i) => i.id === 'appplacement'));
});

void test('Docker container requires a VM placement, pinned image, and registry token environment', () => {
  let { nodes, edges } = sample();
  const container = createNode('container', 'container', 0, 0);
  container.data.label = 'sample-container';
  container.data.port = 8082;
  container.data.containerPort = 8080;
  container.data.image = 'overpass/sample-api';
  container.data.digest = 'sha256:' + 'a'.repeat(64);
  const registry = createNode('registry', 'registry', 600, 300);
  registry.data.label = 'harbor';
  registry.data.host = 'harbor.example.com';
  registry.data.registryUsername = 'robot-studio';
  registry.data.tokenEnv = 'HARBOR_TOKEN';
  nodes.push(container, registry);
  ({ nodes, edges } = placeWorkload(nodes, edges, 'container', 'vm'));
  assert.deepEqual(validate(nodes, edges), []);
  assert.equal(connectionType('container', 'registry'), 'publishes_to');

  nodes.find((node) => node.id === 'container').data.digest = 'latest';
  nodes.find((node) => node.id === 'registry').data.tokenEnv = 'secret-value!';
  const issues = validate(nodes, edges);
  assert.ok(issues.some((issue) => issue.id === 'containerdigest'));
  assert.ok(issues.some((issue) => issue.id === 'registrytoken-env'));
});

void test('legacy registry secret references remain editable for VM blueprints', () => {
  const { nodes, edges } = sample();
  const registry = createNode('registry', 'registry', 600, 300);
  registry.data.label = 'legacy-registry';
  registry.data.host = 'harbor.example.com';
  registry.data.secretRef = 'legacy-harbor-secret';
  nodes.push(registry);
  assert.deepEqual(validate(nodes, edges), []);
});
