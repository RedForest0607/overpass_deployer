import assert from 'node:assert/strict';
import test from 'node:test';
import {
  analyzeArtifactMappings,
  applyArtifactMappings,
  recommendedAssignments,
} from './artifact-mapping.ts';

const artifact = (id, kind, originalName) => ({
  id,
  projectId: 'project-1',
  kind,
  originalName,
  sizeBytes: 10,
  sha256: id.padEnd(64, '0'),
  createdAt: '2026-09-14T00:00:00Z',
});

function topology() {
  return [
    {
      id: 'vm-1',
      type: 'targetGroup',
      position: { x: 0, y: 0 },
      data: {
        kind: 'vm',
        label: 'dev-was-01',
        deployment: {
          files: [
            {
              artifactRef: 'missing-server-file',
              remotePath: '/etc/security/limits.conf',
            },
          ],
        },
      },
    },
    {
      id: 'app-1',
      type: 'resource',
      parentId: 'vm-1',
      position: { x: 20, y: 40 },
      data: {
        kind: 'java',
        label: 'bo-pcweb',
        artifact: '',
        artifactRef: 'placeholder-bo-pcweb-jar',
        deployment: {
          configFiles: [
            {
              artifactRef: 'config-1',
              remotePath: '/app/bo-pcweb/config/application.yml',
            },
          ],
          extraFiles: [
            {
              artifactRef: 'plain-file',
              remotePath: '/tmp/agent.tgz',
              extract: { enabled: true, remoteDir: '/opt/agent' },
            },
          ],
          script: {
            mode: 'template',
            valuesArtifactRef: '',
            valuesPackagePath: 'values/bo-pcweb/script-values.yml',
            remotePath: '/app/bo-pcweb/bin/server.sh',
          },
        },
      },
    },
  ];
}

const artifacts = [
  artifact('jar-1', 'jar', 'bo-pcweb.jar'),
  artifact('config-1', 'config-template', 'application.yml'),
  artifact('plain-file', 'file', 'agent.tgz'),
  artifact('archive-1', 'archive', 'agent.tgz'),
  artifact('archive-zip', 'archive', 'agent.zip'),
  artifact('values-1', 'script-values', 'script-values.yml'),
  artifact('limits-1', 'file', 'limits.conf'),
];

test('discovers typed artifact requirements and reports their status', () => {
  const requirements = analyzeArtifactMappings(topology(), artifacts);
  assert.equal(requirements.length, 5);
  assert.deepEqual(
    requirements.map(({ label, status }) => [label, status]),
    [
      ['서버 파일 · limits.conf', 'unresolved'],
      ['실행 JAR', 'unresolved'],
      ['설정 파일 · application.yml', 'resolved'],
      ['추가 파일 · agent.tgz', 'kind-mismatch'],
      ['스크립트 values', 'missing'],
    ],
  );
  assert.equal(
    requirements.find(({ label }) => label === '추가 파일 · agent.tgz')
      .suggestedArtifactId,
    'archive-1',
  );
});

test('stages suggestions and applies mappings without mutating input nodes', () => {
  const nodes = topology();
  const original = structuredClone(nodes);
  const requirements = analyzeArtifactMappings(nodes, artifacts);
  const result = applyArtifactMappings(
    nodes,
    artifacts,
    recommendedAssignments(requirements),
  );

  assert.deepEqual(nodes, original);
  assert.equal(result.applied, 4);
  const app = result.nodes.find(({ id }) => id === 'app-1');
  assert.equal(app.data.artifactRef, 'jar-1');
  assert.equal(app.data.artifact, 'bo-pcweb.jar');
  assert.equal(app.data.deployment.extraFiles[0].artifactRef, 'archive-1');
  assert.equal(app.data.deployment.script.valuesArtifactRef, 'values-1');
  const vm = result.nodes.find(({ id }) => id === 'vm-1');
  assert.equal(vm.data.deployment.files[0].artifactRef, 'limits-1');
});

test('rejects foreign, incompatible, and stale assignments', () => {
  const requirements = analyzeArtifactMappings(topology(), artifacts);
  const jarRequirement = requirements.find(({ label }) => label === '실행 JAR');

  assert.throws(
    () =>
      applyArtifactMappings(topology(), artifacts, {
        [jarRequirement.id]: 'foreign-artifact',
      }),
    /현재 프로젝트/,
  );
  assert.throws(
    () =>
      applyArtifactMappings(topology(), artifacts, {
        [jarRequirement.id]: 'config-1',
      }),
    /사용할 수 없습니다/,
  );
  const archiveRequirement = requirements.find(({ label }) =>
    label.startsWith('추가 파일'),
  );
  assert.throws(
    () =>
      applyArtifactMappings(topology(), artifacts, {
        [archiveRequirement.id]: 'archive-zip',
      }),
    /사용할 수 없습니다/,
  );
  assert.throws(
    () =>
      applyArtifactMappings(topology(), artifacts, {
        'unknown:data.artifactRef': 'jar-1',
      }),
    /알 수 없는 매핑/,
  );
});
