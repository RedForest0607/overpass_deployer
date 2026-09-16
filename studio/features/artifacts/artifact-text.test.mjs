import assert from 'node:assert/strict';
import test from 'node:test';
import {
  artifactMediaType,
  isEditableArtifact,
  validateArtifactText,
} from './artifact-text.ts';

const artifact = (originalName, kind = 'file') => ({ originalName, kind });

test('only YAML and XML text artifacts are editable', () => {
  assert.equal(isEditableArtifact(artifact('application-prod.yml')), true);
  assert.equal(isEditableArtifact(artifact('values.YAML')), true);
  assert.equal(isEditableArtifact(artifact('logback.xml')), true);
  assert.equal(isEditableArtifact(artifact('application.yml', 'jar')), false);
  assert.equal(isEditableArtifact(artifact('bundle.tar.gz', 'archive')), false);
  assert.equal(isEditableArtifact(artifact('notes.txt')), false);
});

test('YAML validation rejects malformed content', () => {
  assert.equal(
    validateArtifactText('values.yml', 'server:\n  port: 8080'),
    undefined,
  );
  assert.equal(
    validateArtifactText(
      'resources.yml',
      'kind: Service\n---\nkind: Deployment\n',
    ),
    undefined,
  );
  assert.match(
    validateArtifactText('values.yml', 'server: [') ?? '',
    /end|flow/i,
  );
  assert.equal(
    validateArtifactText('values.yml', '  '),
    '파일 내용을 입력하세요.',
  );
});

test('artifact media type follows the edited format', () => {
  assert.equal(artifactMediaType('logback.xml'), 'application/xml');
  assert.equal(artifactMediaType('values.yaml'), 'application/yaml');
});
