import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { parse } from 'yaml';

const directory = path.dirname(fileURLToPath(import.meta.url));
const openAPI = parse(await readFile(path.join(directory, 'openapi-v1.yaml'), 'utf8'));
const blueprint = JSON.parse(
  await readFile(path.join(directory, 'blueprint-v1.schema.json'), 'utf8'),
);

function references(value, result = []) {
  if (Array.isArray(value)) {
    value.forEach((item) => references(item, result));
  } else if (value && typeof value === 'object') {
    if (typeof value.$ref === 'string') result.push(value.$ref);
    Object.values(value).forEach((item) => references(item, result));
  }
  return result;
}

function resolvePointer(document, pointer) {
  return pointer
    .slice(2)
    .split('/')
    .map((part) => part.replaceAll('~1', '/').replaceAll('~0', '~'))
    .reduce((current, part) => current?.[part], document);
}

test('OpenAPI document and local references are internally consistent', async () => {
  assert.equal(openAPI.openapi, '3.1.0');
  assert.equal(openAPI.servers[0].url, '/api/v1');
  for (const reference of references(openAPI)) {
    if (reference.startsWith('#/')) {
      assert.notEqual(resolvePointer(openAPI, reference), undefined, reference);
      continue;
    }
    const target = path.resolve(directory, reference);
    assert.equal(target.startsWith(directory + path.sep), true, reference);
    await readFile(target);
  }
});

test('operation IDs are unique and API operations define error responses', () => {
  const operationIDs = [];
  for (const pathItem of Object.values(openAPI.paths)) {
    for (const method of ['get', 'post', 'put', 'patch', 'delete']) {
      const operation = pathItem[method];
      if (!operation) continue;
      operationIDs.push(operation.operationId);
      assert.ok(operation.responses.default, `${operation.operationId} default response`);
    }
  }
  assert.equal(new Set(operationIDs).size, operationIDs.length);
});

test('Blueprint schema has a stable version and valid local references', () => {
  assert.equal(blueprint.properties.apiVersion.const, 'studio.deployer/v1');
  for (const reference of references(blueprint)) {
    assert.ok(reference.startsWith('#/'), `unexpected external reference ${reference}`);
    assert.notEqual(resolvePointer(blueprint, reference), undefined, reference);
  }
});
