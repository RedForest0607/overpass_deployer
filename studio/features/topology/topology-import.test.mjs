import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  MAX_TOPOLOGY_IMPORT_BYTES,
  parseTopologyImport,
} from './topology-import.ts';
import { validate } from './model.ts';

void test('imports the exported iSTOCKMALL topology envelope', () => {
  const value = parseTopologyImport(
    readFileSync('examples/istockmall-dev-topology.json', 'utf8'),
  );
  assert.equal(value.environment, 'dev');
  assert.equal(value.graph.nodes.length, 15);
  assert.equal(value.graph.edges.length, 19);
  assert.equal(value.graph.deployment.ssh.user, 'ec2-user');
});

void test('imports the Docker commerce example without losing deployment fields', () => {
  const value = parseTopologyImport(
    readFileSync('examples/docker-commerce-dev-topology.json', 'utf8'),
  );
  assert.equal(value.environment, 'dev');
  assert.equal(value.graph.nodes.length, 5);
  assert.equal(value.graph.edges.length, 6);
  const api = value.graph.nodes.find((node) => node.id === 'commerce-api');
  assert.equal(api.parentId, 'docker-host');
  assert.equal(api.data.image, 'demo/commerce-api');
  assert.equal(api.data.containerPort, 8080);
  const registry = value.graph.nodes.find((node) => node.id === 'harbor');
  assert.equal(registry.data.tokenEnv, 'HARBOR_TOKEN');
  assert.equal(registry.data.secretRef, '');
  assert.deepEqual(validate(value.graph.nodes, value.graph.edges), []);
});

void test('imports a raw Blueprint and creates an editable default layout', () => {
  const value = parseTopologyImport(
    JSON.stringify({
      apiVersion: 'studio.deployer/v1',
      nodes: [
        {
          id: 'vm',
          kind: 'vm',
          name: 'vm',
          spec: { provider: 'on-prem', host: 'host' },
        },
        {
          id: 'app',
          kind: 'java',
          name: 'app',
          spec: {
            artifactRef: null,
            javaVersion: '21',
            maxHeap: '1g',
            ports: [{ name: 'main', servicePort: 8080, protocol: 'TCP' }],
          },
        },
      ],
      placements: [{ workloadRef: 'app', targetRef: 'vm', mode: 'process' }],
      connections: [],
    }),
  );

  assert.equal(value.environment, undefined);
  assert.equal(
    value.graph.nodes.find((node) => node.id === 'app').parentId,
    'vm',
  );
  assert.equal(value.graph.viewport.zoom, 0.8);
});

void test('rejects malformed, incompatible, and oversized input', () => {
  assert.throws(() => parseTopologyImport('{'), /JSON 문법/);
  assert.throws(
    () => parseTopologyImport(JSON.stringify({ apiVersion: 'legacy' })),
    /apiVersion/,
  );
  assert.throws(
    () => parseTopologyImport('x'.repeat(MAX_TOPOLOGY_IMPORT_BYTES + 1)),
    /2 MiB/,
  );
  const sample = JSON.parse(
    readFileSync('examples/istockmall-dev-topology.json', 'utf8'),
  );
  sample.blueprint.nodes[0].spec.unsupported = true;
  assert.throws(
    () => parseTopologyImport(JSON.stringify(sample)),
    /지원하지 않는 필드/,
  );
  const invalidLayout = JSON.parse(
    readFileSync('examples/istockmall-dev-topology.json', 'utf8'),
  );
  invalidLayout.layout.nodes.pop();
  assert.throws(
    () => parseTopologyImport(JSON.stringify(invalidLayout)),
    /모든 blueprint 노드/,
  );
});

void test('imports connection layout and rejects unknown connection references', () => {
  const sample = JSON.parse(
    readFileSync('examples/istockmall-dev-topology.json', 'utf8'),
  );
  const connectionId = sample.blueprint.connections[0].id;
  sample.layout.connections = [
    {
      connectionId,
      bendOffset: { x: 80, y: -30 },
      labelPosition: 0.78,
    },
  ];

  const imported = parseTopologyImport(JSON.stringify(sample));
  assert.deepEqual(
    imported.graph.edges.find((edge) => edge.id === connectionId).data.layout,
    {
      bendOffset: sample.layout.connections[0].bendOffset,
      labelPosition: sample.layout.connections[0].labelPosition,
    },
  );

  sample.layout.connections[0].connectionId = 'missing-connection';
  assert.throws(
    () => parseTopologyImport(JSON.stringify(sample)),
    /없거나 중복된 연결/,
  );

  sample.layout.connections[0].connectionId = connectionId;
  sample.layout.connections[0].labelPosition = 1.1;
  assert.throws(
    () => parseTopologyImport(JSON.stringify(sample)),
    /0 이상 1 이하/,
  );
});
