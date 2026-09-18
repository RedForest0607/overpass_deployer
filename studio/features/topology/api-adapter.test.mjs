import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { createNode, sample } from './model.ts';
import { fromAPI, toAPI } from './api-adapter.ts';
import { placeWorkload } from './placement.ts';

test('iSTOCKMALL DEV example is editable without dropping topology data', () => {
  const sample = JSON.parse(
    readFileSync('examples/istockmall-dev-topology.json', 'utf8'),
  );
  const restored = fromAPI(sample.blueprint, sample.layout);
  const encoded = toAPI(
    restored.nodes,
    restored.edges,
    restored.viewport,
    restored.deployment,
  );

  assert.equal(encoded.blueprint.nodes.length, 15);
  assert.equal(encoded.blueprint.placements.length, 12);
  assert.equal(encoded.blueprint.connections.length, 7);
  assert.equal(
    encoded.blueprint.nodes.find((node) => node.id === 'service-redis-dev').spec
      .deployment.services[0].name,
    'istock-redis',
  );
});

test('unsupported modes and extra ports cannot be silently rewritten', () => {
  const doc = toAPI(sample().nodes, sample().edges, { x: 0, y: 0, zoom: 1 });
  doc.blueprint.placements[0].mode = 'compose';
  assert.throws(() => fromAPI(doc.blueprint, doc.layout), /배치/);
  doc.blueprint.placements[0].mode = 'process';
  const ports = doc.blueprint.nodes.find((n) => n.kind === 'java').spec.ports;
  ports.push({ ...ports[0], name: 'admin', servicePort: 9090 });
  assert.throws(() => fromAPI(doc.blueprint, doc.layout), /포트/);
});

test('parent dimensions contain all persisted children without resetting coordinates', () => {
  const doc = toAPI(sample().nodes, sample().edges, { x: 0, y: 0, zoom: 1 });
  doc.layout.nodes.find((n) => n.nodeId === 'redis').position.y = 800;
  const restored = fromAPI(doc.blueprint, doc.layout);
  assert.equal(restored.nodes.find((n) => n.id === 'redis').position.y, 800);
  assert.ok(restored.nodes.find((n) => n.id === 'vm').style.height >= 1000);
});

test('React Flow topology round-trips through Blueprint and layout', () => {
  const original = sample();
  original.nodes.find((node) => node.id === 'app').data.artifactRef =
    'artifact-01';
  const viewport = { x: 12, y: -8, zoom: 1.25 };
  const encoded = toAPI(original.nodes, original.edges, viewport);

  assert.equal(encoded.blueprint.apiVersion, 'studio.deployer/v1');
  assert.deepEqual(encoded.blueprint.placements, [
    { workloadRef: 'app', targetRef: 'vm', mode: 'process' },
    { workloadRef: 'redis', targetRef: 'vm', mode: 'process' },
  ]);
  assert.ok(
    encoded.blueprint.nodes.every(
      (node) => !('position' in node) && !('parentId' in node),
    ),
  );

  const restored = fromAPI(encoded.blueprint, encoded.layout);
  assert.deepEqual(restored.viewport, viewport);
  assert.equal(restored.nodes.find((node) => node.id === 'app').parentId, 'vm');
  assert.equal(
    restored.nodes.find((node) => node.id === 'app').data.artifactRef,
    'artifact-01',
  );
  assert.deepEqual(
    restored.nodes.find((node) => node.id === 'app').position,
    original.nodes.find((node) => node.id === 'app').position,
  );
  assert.equal(
    restored.edges.filter((edge) => edge.label === 'deploys_to').length,
    2,
  );
});

test('communication bend and on-line label position round-trip only through layout', () => {
  const original = sample();
  original.edges.find((edge) => edge.id === 'app-db').data.layout = {
    bendOffset: { x: 120, y: -35 },
    labelPosition: 0.72,
  };

  const encoded = toAPI(original.nodes, original.edges, {
    x: 0,
    y: 0,
    zoom: 1,
  });

  assert.deepEqual(encoded.layout.connections, [
    {
      connectionId: 'app-db',
      bendOffset: { x: 120, y: -35 },
      labelPosition: 0.72,
    },
  ]);
  assert.equal('layout' in encoded.blueprint.connections[0], false);
  const restored = fromAPI(encoded.blueprint, encoded.layout);
  assert.deepEqual(
    restored.edges.find((edge) => edge.id === 'app-db').data.layout,
    original.edges.find((edge) => edge.id === 'app-db').data.layout,
  );
});

test('external dependencies and AWS target map to explicit API enums', () => {
  const original = sample();
  const vm = original.nodes.find((node) => node.id === 'vm');
  vm.data.platform = 'AWS EC2';
  const redis = original.nodes.find((node) => node.id === 'redis');
  redis.parentId = undefined;
  redis.extent = undefined;
  redis.data.host = 'redis.internal';
  original.edges = original.edges.filter((edge) => edge.id !== 'redis-vm');

  const { blueprint } = toAPI(original.nodes, original.edges, {
    x: 0,
    y: 0,
    zoom: 1,
  });
  assert.equal(
    blueprint.nodes.find((node) => node.id === 'vm').spec.provider,
    'aws-ec2',
  );
  assert.equal(
    blueprint.nodes.find((node) => node.id === 'redis').spec.location,
    'external',
  );
  assert.equal(
    blueprint.nodes.find((node) => node.id === 'redis').spec.endpoint,
    'redis.internal',
  );
});

test('Crosswalk deployment profiles survive an API round trip', () => {
  const original = sample();
  const vm = original.nodes.find((node) => node.id === 'vm');
  const app = original.nodes.find((node) => node.id === 'app');
  const redis = original.nodes.find((node) => node.id === 'redis');
  vm.data.deployment = {
    bootstrap: { packages: ['curl'], timezone: 'Asia/Seoul' },
    files: [
      { artifactRef: 'common-01', remotePath: '/etc/app.conf', chmod: '0644' },
    ],
  };
  app.data.deployment = {
    tags: ['api', 'commerce'],
    javaOptions: ['-Duser.timezone=Asia/Seoul'],
    extraFiles: [
      {
        artifactRef: 'archive-01',
        remotePath: '/tmp/agent.zip',
        extract: { enabled: true, remoteDir: '/opt/agent' },
      },
    ],
    script: {
      mode: 'local-file',
      artifactRef: 'script-01',
      remotePath: '/app/app/bin/server.sh',
    },
  };
  redis.data.deployment = {
    directories: ['/app/redis'],
    services: [{ name: 'redis-server', enabled: true, state: 'started' }],
  };
  app.data.artifactRef = 'jar-01';
  const deployment = {
    ssh: {
      user: 'deploy',
      keyPath: '${DEPLOY_SSH_KEY_PATH}',
      knownHostsPath: '${DEPLOY_KNOWN_HOSTS}',
      hostKeyChecking: 'strict',
    },
    bastion: { host: 'bastion.example' },
    bootstrap: { packages: ['nmap-ncat'], timezone: 'Asia/Seoul' },
  };
  const encoded = toAPI(
    original.nodes,
    original.edges,
    { x: 0, y: 0, zoom: 1 },
    deployment,
  );
  const restored = fromAPI(encoded.blueprint, encoded.layout);
  assert.deepEqual(restored.deployment, deployment);
  assert.deepEqual(
    restored.nodes.find((node) => node.id === 'vm').data.deployment,
    vm.data.deployment,
  );
  assert.deepEqual(
    restored.nodes.find((node) => node.id === 'app').data.deployment,
    app.data.deployment,
  );
  assert.deepEqual(
    restored.nodes.find((node) => node.id === 'redis').data.deployment,
    redis.data.deployment,
  );
});

test('Docker container and registry round-trip without storing token values', () => {
  let { nodes, edges } = sample();
  const container = createNode('container', 'container', 0, 0);
  container.data.label = 'sample-container';
  container.data.image = 'overpass/sample-api';
  container.data.digest = 'sha256:' + 'a'.repeat(64);
  container.data.port = 18080;
  container.data.containerPort = 8080;
  container.data.deployment = {
    environment: { SPRING_PROFILES_ACTIVE: 'dev' },
    restartPolicy: 'unless-stopped',
    healthTimeoutSeconds: 60,
  };
  const registry = createNode('registry', 'registry', 600, 200);
  registry.data.label = 'harbor';
  registry.data.host = 'harbor.example.com';
  registry.data.registryUsername = 'robot-studio';
  registry.data.tokenEnv = 'HARBOR_TOKEN';
  nodes.push(container, registry);
  ({ nodes, edges } = placeWorkload(nodes, edges, 'container', 'vm'));

  const encoded = toAPI(nodes, edges, { x: 0, y: 0, zoom: 1 });
  assert.ok(
    encoded.blueprint.placements.some(
      (placement) =>
        placement.workloadRef === 'container' && placement.mode === 'container',
    ),
  );
  const containerSpec = encoded.blueprint.nodes.find(
    (node) => node.id === 'container',
  ).spec;
  assert.equal(containerSpec.ports[0].servicePort, 18080);
  assert.equal(containerSpec.ports[0].containerPort, 8080);
  const registrySpec = encoded.blueprint.nodes.find(
    (node) => node.id === 'registry',
  ).spec;
  assert.equal(registrySpec.tokenEnv, 'HARBOR_TOKEN');
  assert.equal('token' in registrySpec, false);

  const restored = fromAPI(encoded.blueprint, encoded.layout);
  assert.equal(
    restored.nodes.find((node) => node.id === 'container').parentId,
    'vm',
  );
  assert.equal(
    restored.nodes.find((node) => node.id === 'container').data.containerPort,
    8080,
  );
  assert.equal(
    restored.nodes.find((node) => node.id === 'registry').data.registryUsername,
    'robot-studio',
  );
});
