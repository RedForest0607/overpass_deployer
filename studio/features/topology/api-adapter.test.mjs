import assert from 'node:assert/strict';
import test from 'node:test';
import { sample } from './model.ts';
import { fromAPI, toAPI } from './api-adapter.ts';

test('unsupported modes and extra ports cannot be silently rewritten',()=>{
 const doc=toAPI(sample().nodes,sample().edges,{x:0,y:0,zoom:1});
 doc.blueprint.placements[0].mode='compose';
 assert.throws(()=>fromAPI(doc.blueprint,doc.layout),/배치/);
 doc.blueprint.placements[0].mode='process';
 const ports=doc.blueprint.nodes.find(n=>n.kind==='java').spec.ports;
 ports.push({...ports[0],name:'admin',servicePort:9090});
 assert.throws(()=>fromAPI(doc.blueprint,doc.layout),/포트/);
});

test('parent dimensions contain all persisted children without resetting coordinates',()=>{
 const doc=toAPI(sample().nodes,sample().edges,{x:0,y:0,zoom:1});
 doc.layout.nodes.find(n=>n.nodeId==='redis').position.y=800;
 const restored=fromAPI(doc.blueprint,doc.layout);
 assert.equal(restored.nodes.find(n=>n.id==='redis').position.y,800);
 assert.ok(restored.nodes.find(n=>n.id==='vm').style.height>=1000);
});

test('React Flow topology round-trips through Blueprint and layout', () => {
  const original = sample();
  original.nodes.find((node) => node.id === 'app').data.artifactRef = 'artifact-01';
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
  assert.equal(blueprint.nodes.find((node) => node.id === 'vm').spec.provider, 'aws-ec2');
  assert.equal(blueprint.nodes.find((node) => node.id === 'redis').spec.location, 'external');
  assert.equal(blueprint.nodes.find((node) => node.id === 'redis').spec.endpoint, 'redis.internal');
});
