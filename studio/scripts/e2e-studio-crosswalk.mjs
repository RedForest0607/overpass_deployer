import assert from 'node:assert/strict';
import { once } from 'node:events';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';
import { gzipSync } from 'node:zlib';
import JSZip from 'jszip';

const studioDir = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..',
);
const repositoryDir = path.resolve(studioDir, '..');
const backendDir = path.join(studioDir, 'backend');
const crosswalkDir = path.join(repositoryDir, 'crosswalk');
const temporaryDir = await mkdtemp(
  path.join(os.tmpdir(), 'overpass-studio-e2e-'),
);
let backend;

async function run(command, args, options = {}) {
  const child = spawn(command, args, {
    cwd: options.cwd,
    env: { ...process.env, ...options.env },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let stdout = '';
  let stderr = '';
  child.stdout.on('data', (chunk) => (stdout += chunk));
  child.stderr.on('data', (chunk) => (stderr += chunk));
  const [code] = await once(child, 'exit');
  if (code !== 0) {
    throw new Error(
      `${command} ${args.join(' ')} failed (${code})\n${stdout}\n${stderr}`,
    );
  }
  return { stdout, stderr };
}

async function freePort() {
  const server = net.createServer();
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  assert.equal(typeof address, 'object');
  server.close();
  await once(server, 'close');
  return address.port;
}

async function jsonRequest(url, options = {}) {
  const response = await fetch(url, options);
  const body = await response.text();
  if (!response.ok) {
    throw new Error(
      `${options.method ?? 'GET'} ${url} -> ${response.status}\n${body}`,
    );
  }
  return { response, value: body ? JSON.parse(body) : undefined };
}

async function upload(baseURL, projectID, kind, name, bytes) {
  const form = new FormData();
  form.set('kind', kind);
  form.set('file', new Blob([bytes]), name);
  const { value } = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/artifacts`,
    { method: 'POST', body: form },
  );
  return value;
}

async function waitUntilReady(baseURL, child) {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    if (child.exitCode !== null)
      throw new Error('Studio Backend exited before readiness');
    try {
      const response = await fetch(`${baseURL}/healthz`);
      if (response.ok) return;
    } catch {
      // The listener is still starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  throw new Error('Studio Backend readiness timed out');
}

async function waitForGeneration(baseURL, projectID, generationID) {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    const { value } = await jsonRequest(
      `${baseURL}/api/v1/projects/${projectID}/generations/${generationID}`,
    );
    if (value.status === 'succeeded' || value.status === 'failed') return value;
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  throw new Error('generation completion timed out');
}

async function extractZIP(bytes, destination) {
  const archive = await JSZip.loadAsync(bytes);
  for (const [entryPath, entry] of Object.entries(archive.files)) {
    const normalized = path.posix.normalize(entryPath);
    if (
      path.posix.isAbsolute(normalized) ||
      normalized === '..' ||
      normalized.startsWith('../')
    ) {
      throw new Error(`unsafe generated ZIP entry: ${entryPath}`);
    }
    const target = path.join(destination, ...normalized.split('/'));
    if (entry.dir) {
      await mkdir(target, { recursive: true });
      continue;
    }
    await mkdir(path.dirname(target), { recursive: true });
    await writeFile(target, await entry.async('nodebuffer'));
  }
  return archive;
}

function tarGZIP(name, content) {
  const body = Buffer.from(content);
  const header = Buffer.alloc(512);
  const write = (value, offset, length) =>
    header.write(
      value,
      offset,
      Math.min(Buffer.byteLength(value), length),
      'ascii',
    );
  const octal = (value, length) =>
    `${value.toString(8).padStart(length - 1, '0')}\0`;
  write(name, 0, 100);
  write(octal(0o644, 8), 100, 8);
  write(octal(0, 8), 108, 8);
  write(octal(0, 8), 116, 8);
  write(octal(body.length, 12), 124, 12);
  write(octal(0, 12), 136, 12);
  header.fill(0x20, 148, 156);
  header[156] = '0'.charCodeAt(0);
  write('ustar\0', 257, 6);
  write('00', 263, 2);
  const checksum = header.reduce((total, value) => total + value, 0);
  write(`${checksum.toString(8).padStart(6, '0')}\0 `, 148, 8);
  const padding = Buffer.alloc((512 - (body.length % 512)) % 512);
  return gzipSync(Buffer.concat([header, body, padding, Buffer.alloc(1024)]));
}

async function verifyDockerPackage(baseURL, crosswalkBinary, keyPath) {
  const created = await jsonRequest(`${baseURL}/api/v1/projects`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: 'studio-crosswalk-docker-e2e',
      description: '',
    }),
  });
  const location = created.response.headers.get('location');
  assert.ok(location);
  const projectID = location.split('/').at(-1);
  const blueprint = {
    apiVersion: 'studio.deployer/v1',
    deployment: {
      ssh: {
        user: 'deploy',
        keyPath: '${DEPLOY_SSH_KEY_PATH}',
        knownHostsPath: '',
        hostKeyChecking: 'insecure',
      },
    },
    nodes: [
      {
        id: 'registry',
        kind: 'registry',
        name: 'harbor',
        spec: {
          endpoint: 'harbor.example.com',
          username: 'robot-studio',
          tokenEnv: 'HARBOR_TOKEN',
        },
      },
      {
        id: 'vm',
        kind: 'vm',
        name: 'docker-dev-01',
        spec: { provider: 'on-prem', host: '192.0.2.20' },
      },
      {
        id: 'api',
        kind: 'container',
        name: 'commerce-api',
        spec: {
          image: 'team/commerce-api',
          digest: `sha256:${'a'.repeat(64)}`,
          ports: [
            {
              name: 'http',
              servicePort: 18080,
              containerPort: 8080,
              protocol: 'TCP',
            },
          ],
          deployment: {
            tags: ['api'],
            environment: { SPRING_PROFILES_ACTIVE: 'dev' },
            restartPolicy: 'unless-stopped',
            healthTimeoutSeconds: 60,
          },
        },
      },
    ],
    placements: [{ workloadRef: 'api', targetRef: 'vm', mode: 'container' }],
    connections: [],
  };
  const layout = {
    nodes: blueprint.nodes.map((node, index) => ({
      nodeId: node.id,
      position: { x: index * 200, y: 0 },
    })),
    viewport: { x: 0, y: 0, zoom: 1 },
  };

  await jsonRequest(`${baseURL}/api/v1/projects/${projectID}/topologies/dev`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ expectedRevision: 0, blueprint, layout }),
  });
  const validation = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/validations`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        environment: 'dev',
        clientRequestId: 'studio-crosswalk-docker-e2e',
        purpose: 'package',
        generator: 'crosswalk-docker',
        blueprint,
      }),
    },
  );
  assert.equal(validation.value.valid, true);

  const generation = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/generations`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': 'studio-crosswalk-docker-e2e-package',
      },
      body: JSON.stringify({
        environment: 'dev',
        revision: 1,
        generator: 'crosswalk-docker',
        output: 'package',
      }),
    },
  );
  const completed = await waitForGeneration(
    baseURL,
    projectID,
    generation.value.id,
  );
  assert.equal(completed.status, 'succeeded');
  assert.ok(completed.packageId);

  const metadata = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/packages/${completed.packageId}`,
  );
  assert.equal(metadata.value.generator, 'crosswalk-docker');
  assert.equal(metadata.value.chartFormat, 'crosswalk/v1');

  const packageResponse = await fetch(
    `${baseURL}/api/v1/projects/${projectID}/packages/${completed.packageId}/download`,
  );
  assert.equal(packageResponse.status, 200);
  const packageDir = path.join(temporaryDir, 'docker-package');
  await mkdir(packageDir, { recursive: true });
  const archive = await extractZIP(
    Buffer.from(await packageResponse.arrayBuffer()),
    packageDir,
  );
  assert.deepEqual(Object.keys(archive.files), ['deploy.yml']);
  const chart = await readFile(path.join(packageDir, 'deploy.yml'), 'utf8');
  assert.match(chart, /token_env: HARBOR_TOKEN/);
  assert.doesNotMatch(chart, /token:|password:|secret:/i);

  const dryRun = await run(
    crosswalkBinary,
    ['docker', '--dry-run', '--config', 'deploy.yml'],
    { cwd: packageDir, env: { DEPLOY_SSH_KEY_PATH: keyPath } },
  );
  assert.match(dryRun.stdout, /DRY-RUN|dry-run|docker-dev-01|commerce-api/i);

  const composeValidation = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/validations`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        environment: 'dev',
        clientRequestId: 'studio-crosswalk-compose-e2e',
        purpose: 'package',
        generator: 'crosswalk-compose',
        blueprint,
      }),
    },
  );
  assert.equal(composeValidation.value.valid, true);

  const composeGeneration = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/generations`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': 'studio-crosswalk-compose-e2e-package',
      },
      body: JSON.stringify({
        environment: 'dev',
        revision: 1,
        generator: 'crosswalk-compose',
        output: 'package',
      }),
    },
  );
  const completedCompose = await waitForGeneration(
    baseURL,
    projectID,
    composeGeneration.value.id,
  );
  assert.equal(completedCompose.status, 'succeeded');
  assert.ok(completedCompose.packageId);

  const composeMetadata = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/packages/${completedCompose.packageId}`,
  );
  assert.equal(composeMetadata.value.generator, 'crosswalk-compose');
  assert.equal(composeMetadata.value.chartFormat, 'crosswalk/v1');

  const composePackageResponse = await fetch(
    `${baseURL}/api/v1/projects/${projectID}/packages/${completedCompose.packageId}/download`,
  );
  assert.equal(composePackageResponse.status, 200);
  const composePackageDir = path.join(temporaryDir, 'compose-package');
  await mkdir(composePackageDir, { recursive: true });
  const composeArchive = await extractZIP(
    Buffer.from(await composePackageResponse.arrayBuffer()),
    composePackageDir,
  );
  assert.deepEqual(Object.keys(composeArchive.files), [
    'deploy.yml',
    'compose/docker-dev-01/compose.yml',
  ]);
  const composeChart = await readFile(
    path.join(composePackageDir, 'deploy.yml'),
    'utf8',
  );
  assert.match(composeChart, /compose_projects:/);
  assert.match(composeChart, /token_env: HARBOR_TOKEN/);
  assert.doesNotMatch(composeChart, /token:|password:|secret:/i);
  const composeDocument = await readFile(
    path.join(composePackageDir, 'compose/docker-dev-01/compose.yml'),
    'utf8',
  );
  assert.match(
    composeDocument,
    new RegExp(
      `harbor\\.example\\.com/team/commerce-api@sha256:${'a'.repeat(64)}`,
    ),
  );
  assert.match(composeDocument, /18080:8080\/tcp/);

  const composeDryRun = await run(
    crosswalkBinary,
    ['compose', '--dry-run', '--config', 'deploy.yml'],
    { cwd: composePackageDir, env: { DEPLOY_SSH_KEY_PATH: keyPath } },
  );
  assert.match(
    composeDryRun.stdout,
    /DRY-RUN|dry-run|docker-dev-01|verify-compose|validate-compose/i,
  );
}

try {
  const backendBinary = path.join(temporaryDir, 'studio-api');
  const crosswalkBinary = path.join(temporaryDir, 'deploy');
  await run('go', ['build', '-o', backendBinary, './cmd/studio-api'], {
    cwd: backendDir,
  });
  await run('go', ['build', '-o', crosswalkBinary, './cmd/deploy'], {
    cwd: crosswalkDir,
  });

  const port = await freePort();
  const baseURL = `http://127.0.0.1:${port}`;
  backend = spawn(
    backendBinary,
    [
      '--address',
      `127.0.0.1:${port}`,
      '--data-dir',
      path.join(temporaryDir, 'data'),
    ],
    { stdio: ['ignore', 'pipe', 'pipe'] },
  );
  await waitUntilReady(baseURL, backend);

  const created = await jsonRequest(`${baseURL}/api/v1/projects`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: 'studio-crosswalk-e2e', description: '' }),
  });
  const location = created.response.headers.get('location');
  assert.ok(location);
  const projectID = location.split('/').at(-1);

  const jarZIP = new JSZip();
  jarZIP.file('META-INF/MANIFEST.MF', 'Manifest-Version: 1.0\n');
  const [jar, config, values, redis] = await Promise.all([
    upload(
      baseURL,
      projectID,
      'jar',
      'commerce.jar',
      await jarZIP.generateAsync({ type: 'nodebuffer' }),
    ),
    upload(
      baseURL,
      projectID,
      'config-template',
      'application.yml',
      Buffer.from('server:\n  port: 8080\n'),
    ),
    upload(
      baseURL,
      projectID,
      'script-values',
      'script-values.yml',
      Buffer.from('ActiveProfile: dev\n'),
    ),
    upload(
      baseURL,
      projectID,
      'archive',
      'redis.tgz',
      tarGZIP('redis/redis.conf', 'bind 127.0.0.1\n'),
    ),
  ]);

  const blueprint = {
    apiVersion: 'studio.deployer/v1',
    deployment: {
      ssh: {
        user: 'deploy',
        keyPath: '${DEPLOY_SSH_KEY_PATH}',
        knownHostsPath: '',
        hostKeyChecking: 'insecure',
      },
    },
    nodes: [
      {
        id: 'vm',
        kind: 'vm',
        name: 'dev-was-01',
        spec: { provider: 'on-prem', host: '192.0.2.10' },
      },
      {
        id: 'app',
        kind: 'java',
        name: 'commerce',
        spec: {
          artifactRef: jar.id,
          javaVersion: '21',
          maxHeap: '1g',
          ports: [{ name: 'main', servicePort: 8080, protocol: 'TCP' }],
          deployment: {
            configFiles: [
              {
                artifactRef: config.id,
                remotePath: '/app/overpass/commerce/config/application.yml',
              },
            ],
            script: {
              mode: 'template',
              template: 'embedded:server.sh.tmpl',
              valuesArtifactRef: values.id,
              remotePath: '/app/overpass/commerce/bin/server.sh',
            },
          },
        },
      },
      {
        id: 'redis',
        kind: 'redis',
        name: 'session-redis',
        spec: {
          location: 'hosted',
          ports: [{ name: 'main', servicePort: 6379, protocol: 'TCP' }],
          deployment: {
            directories: ['/app/redis'],
            files: [
              {
                artifactRef: redis.id,
                remotePath: '/tmp/redis.tgz',
                extract: { enabled: true, remoteDir: '/app/redis' },
              },
            ],
          },
        },
      },
    ],
    placements: [
      { workloadRef: 'app', targetRef: 'vm', mode: 'process' },
      { workloadRef: 'redis', targetRef: 'vm', mode: 'process' },
    ],
    connections: [
      {
        id: 'app-redis',
        kind: 'connects_to',
        sourceRef: 'app',
        targetRef: 'redis',
        targetPortName: 'main',
      },
    ],
  };
  const layout = {
    nodes: blueprint.nodes.map((node, index) => ({
      nodeId: node.id,
      position: { x: index * 200, y: 0 },
    })),
    viewport: { x: 0, y: 0, zoom: 1 },
  };

  await jsonRequest(`${baseURL}/api/v1/projects/${projectID}/topologies/dev`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ expectedRevision: 0, blueprint, layout }),
  });
  const validation = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/validations`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        environment: 'dev',
        clientRequestId: 'studio-crosswalk-e2e',
        purpose: 'package',
        generator: 'crosswalk-vm',
        blueprint,
      }),
    },
  );
  assert.equal(validation.value.valid, true);

  const generationBody = JSON.stringify({
    environment: 'dev',
    revision: 1,
    generator: 'crosswalk-vm',
    output: 'package',
  });
  const generation = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/generations`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': 'studio-crosswalk-e2e-package',
      },
      body: generationBody,
    },
  );
  assert.equal(generation.value.status, 'queued');
  const completedGeneration = await waitForGeneration(
    baseURL,
    projectID,
    generation.value.id,
  );
  assert.equal(completedGeneration.status, 'succeeded');
  assert.ok(completedGeneration.packageId);

  const replay = await jsonRequest(
    `${baseURL}/api/v1/projects/${projectID}/generations`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': 'studio-crosswalk-e2e-package',
      },
      body: generationBody,
    },
  );
  assert.equal(replay.value.id, generation.value.id);

  const packageResponse = await fetch(
    `${baseURL}/api/v1/projects/${projectID}/packages/${completedGeneration.packageId}/download`,
  );
  assert.equal(packageResponse.status, 200);
  const packageBytes = Buffer.from(await packageResponse.arrayBuffer());
  const packageDir = path.join(temporaryDir, 'package');
  await mkdir(packageDir, { recursive: true });
  const archive = await extractZIP(packageBytes, packageDir);
  for (const expected of [
    'deploy.yml',
    'assets/commerce/commerce.jar',
    'config/commerce/application.yml',
    'values/commerce/script-values.yml',
    'files/utility/dev-was-01/session-redis/redis.tgz',
  ]) {
    assert.ok(archive.files[expected], `missing package entry: ${expected}`);
  }

  const keyPath = path.join(temporaryDir, 'test-key');
  await writeFile(keyPath, 'not-used-by-dry-run\n', { mode: 0o600 });
  const dryRun = await run(
    crosswalkBinary,
    ['vm', '--dry-run', '--config', 'deploy.yml'],
    { cwd: packageDir, env: { DEPLOY_SSH_KEY_PATH: keyPath } },
  );
  assert.match(dryRun.stdout, /DRY-RUN|dry-run|dev-was-01/i);
  await verifyDockerPackage(baseURL, crosswalkBinary, keyPath);
  console.log(
    'Studio → generated VM/Docker/Compose ZIP → Crosswalk dry-run E2E passed.',
  );
} finally {
  if (backend && backend.exitCode === null) {
    backend.kill('SIGTERM');
    await Promise.race([
      once(backend, 'exit'),
      new Promise((resolve) => setTimeout(resolve, 2_000)),
    ]);
  }
  await rm(temporaryDir, { recursive: true, force: true });
}
