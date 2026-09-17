import { spawnSync } from 'node:child_process';
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parse } from 'yaml';

const studioDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const schemaPath = path.join(studioDirectory, 'schemas/openapi-v1.yaml');
const typescriptPath = path.join(studioDirectory, 'lib/api/generated.ts');
const goPath = path.join(studioDirectory, 'backend/internal/api/generated_types.go');
const checkOnly = process.argv.includes('--check');
const requestSchemas = new Set([
  'CreateProject',
  'UpdateProject',
  'SaveTopology',
  'ValidationRequest',
  'CreateGeneration',
]);
const goSchemas = requestSchemas;
const externalTypeOverrides = new Map([
  ['./blueprint-v1.schema.json', { typescript: 'Blueprint', go: 'json.RawMessage' }],
]);
const goComponentOverrides = new Map([
  ['Layout', 'json.RawMessage'],
]);

const document = parse(await readFile(schemaPath, 'utf8'));
const schemas = document?.components?.schemas;
if (!schemas || typeof schemas !== 'object') {
  throw new Error('OpenAPI components.schemas is required');
}

const typescript = renderTypescript(schemas);
const go = formatGo(renderGo(schemas));
await emit(typescriptPath, typescript);
await emit(goPath, go);

async function emit(target, content) {
  if (checkOnly) {
    const current = await readFile(target, 'utf8').catch(() => '');
    if (current !== content) {
      throw new Error(`${path.relative(studioDirectory, target)} is stale; run npm run generate:api`);
    }
    return;
  }
  await writeFile(target, content);
}

function renderTypescript(values) {
  const lines = [
    '// Code generated from schemas/openapi-v1.yaml by scripts/generate-api-contracts.mjs. DO NOT EDIT.',
    "import type { Blueprint, Layout } from './contracts.ts';",
    'export type { Blueprint, Layout };',
    '',
  ];
  for (const [name, schema] of Object.entries(values)) {
    if (name === 'Blueprint' || name === 'Layout') continue;
    lines.push(`export type ${name} = ${typescriptType(schema)};`, '');
  }
  return lines.join('\n');
}

function typescriptType(schema) {
  if (!schema || typeof schema !== 'object') return 'unknown';
  if (schema.$ref) return typescriptReference(schema.$ref);
  if (schema.const !== undefined) return JSON.stringify(schema.const);
  if (schema.enum) return schema.enum.map((value) => JSON.stringify(value)).join(' | ');
  if (schema.oneOf) return schema.oneOf.map(typescriptType).join(' | ');
  if (schema.allOf) return schema.allOf.map(typescriptType).join(' & ');

  const nullable = Array.isArray(schema.type) && schema.type.includes('null');
  const type = Array.isArray(schema.type)
    ? schema.type.find((value) => value !== 'null')
    : schema.type;
  let result;
  switch (type) {
    case 'string':
      result = 'string';
      break;
    case 'integer':
    case 'number':
      result = 'number';
      break;
    case 'boolean':
      result = 'boolean';
      break;
    case 'array':
      result = `Array<${typescriptType(schema.items)}>`;
      break;
    case 'null':
      result = 'null';
      break;
    case 'object': {
      if (!schema.properties && schema.additionalProperties) {
        result = `Record<string, ${schema.additionalProperties === true ? 'unknown' : typescriptType(schema.additionalProperties)}>`;
        break;
      }
      const required = new Set(schema.required ?? []);
      const fields = Object.entries(schema.properties ?? {}).map(
        ([name, property]) => `${JSON.stringify(name)}${required.has(name) ? '' : '?'}: ${typescriptType(property)};`,
      );
      if (schema.additionalProperties) {
        fields.push(`[key: string]: ${schema.additionalProperties === true ? 'unknown' : typescriptType(schema.additionalProperties)};`);
      }
      result = `{ ${fields.join(' ')} }`;
      break;
    }
    default:
      result = schema.properties ? typescriptType({ ...schema, type: 'object' }) : 'unknown';
  }
  return nullable && result !== 'null' ? `${result} | null` : result;
}

function typescriptReference(reference) {
  if (reference.startsWith('#/components/schemas/')) return reference.split('/').at(-1);
  const override = externalTypeOverrides.get(reference);
  if (!override) throw new Error(`unsupported external TypeScript reference: ${reference}`);
  return override.typescript;
}

function renderGo(values) {
  const declarations = [];
  for (const [name, schema] of Object.entries(values)) {
    if (!goSchemas.has(name)) continue;
    declarations.push(...goDeclaration(name, schema), '');
  }
  const imports = [];
  if (declarations.some((line) => line.includes('json.RawMessage'))) imports.push('\t"encoding/json"');
  if (declarations.some((line) => line.includes('time.Time'))) imports.push('\t"time"');
  const lines = [
    '// Code generated from schemas/openapi-v1.yaml by scripts/generate-api-contracts.mjs. DO NOT EDIT.',
    'package api',
    '',
  ];
  if (imports.length) {
    lines.push('import (', ...imports, ')', '');
  }
  lines.push(...declarations);
  return lines.join('\n');
}

function goDeclaration(name, schema) {
  if (schema.$ref) return [`type ${name} = ${goType(schema)}`];
  if (schema.type === 'object' || schema.properties) {
    return [`type ${name} ${goStruct(schema, requestSchemas.has(name))}`];
  }
  return [`type ${name} ${goType(schema)}`];
}

function goStruct(schema, request) {
  const required = new Set(schema.required ?? []);
  const lines = ['struct {'];
  for (const [jsonName, property] of Object.entries(schema.properties ?? {})) {
    const isRequired = required.has(jsonName);
    let fieldType = goType(property);
    if ((request && isRequired) || (!isRequired && goNeedsPointer(fieldType))) {
      fieldType = `*${fieldType}`;
    }
    const omitEmpty = isRequired ? '' : ',omitempty';
    lines.push(`\t${goName(jsonName)} ${fieldType} \`json:"${jsonName}${omitEmpty}"\``);
  }
  lines.push('}');
  return lines.join('\n');
}

function goType(schema) {
  if (!schema || typeof schema !== 'object') return 'any';
  if (schema.$ref) return goReference(schema.$ref);
  if (schema.const !== undefined) return typeof schema.const === 'string' ? 'string' : 'any';
  if (schema.enum) return schema.enum.every((value) => typeof value === 'string') ? 'string' : 'any';
  if (schema.oneOf) {
    const nonNull = schema.oneOf.filter((value) => value.type !== 'null');
    if (nonNull.length === 1 && nonNull.length !== schema.oneOf.length) return `*${goType(nonNull[0])}`;
    return 'any';
  }
  if (schema.allOf) {
    if (schema.allOf.length === 1) return goType(schema.allOf[0]);
    return 'any';
  }
  const nullable = Array.isArray(schema.type) && schema.type.includes('null');
  const type = Array.isArray(schema.type)
    ? schema.type.find((value) => value !== 'null')
    : schema.type;
  let result;
  switch (type) {
    case 'string':
      result = schema.format === 'date-time' ? 'time.Time' : 'string';
      break;
    case 'integer':
      result = 'int64';
      break;
    case 'number':
      result = 'float64';
      break;
    case 'boolean':
      result = 'bool';
      break;
    case 'array':
      result = `[]${goType(schema.items)}`;
      break;
    case 'object':
      if (!schema.properties && schema.additionalProperties) {
        result = `map[string]${schema.additionalProperties === true ? 'any' : goType(schema.additionalProperties)}`;
      } else {
        result = goStruct(schema, false);
      }
      break;
    default:
      result = schema.properties ? goStruct(schema, false) : 'any';
  }
  return nullable && goNeedsPointer(result) ? `*${result}` : result;
}

function goReference(reference) {
  if (reference.startsWith('#/components/schemas/')) {
    const name = reference.split('/').at(-1);
    const override = goComponentOverrides.get(name);
    if (override) return override;
    return goSchemas.has(name) ? name : goType(schemas[name]);
  }
  const override = externalTypeOverrides.get(reference);
  if (!override) throw new Error(`unsupported external Go reference: ${reference}`);
  return override.go;
}

function goNeedsPointer(type) {
  return !type.startsWith('*') && !type.startsWith('[]') && !type.startsWith('map[') && type !== 'any';
}

function goName(value) {
  const words = value.replace(/([a-z0-9])([A-Z])/g, '$1 $2').split(/[^a-zA-Z0-9]+/);
  return words
    .filter(Boolean)
    .map((word) => {
      const upper = word.toUpperCase();
      if (['ID', 'URL', 'URI', 'API', 'JAR', 'SHA'].includes(upper)) return upper;
      return word[0].toUpperCase() + word.slice(1);
    })
    .join('')
    .replace(/Ids$/, 'IDs')
    .replace(/Id$/, 'ID')
    .replace(/^Sha(?=\d|$)/, 'SHA');
}

function formatGo(source) {
  const result = spawnSync('gofmt', { input: source, encoding: 'utf8' });
  if (result.status !== 0) throw new Error(`gofmt generated_types.go: ${result.stderr}`);
  return result.stdout;
}
