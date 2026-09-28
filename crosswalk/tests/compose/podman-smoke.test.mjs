import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';

const script = fileURLToPath(new URL('./run-podman-smoke.sh', import.meta.url));

// Shell functions intercept every engine/VM command; no daemon, download or cleanup runs.
const harness = `
podman() {
  printf 'podman %s\\n' "$*" >&2
  case "$*" in
    'machine inspect review-machine --format {{.State}}') printf 'stopped\\n' ;;
    'machine inspect review-machine --format {{.SSHConfig.Port}}') printf '2222\\n' ;;
    'machine inspect review-machine --format {{.SSHConfig.IdentityPath}}') printf '%s\\n' "$0" ;;
    'machine inspect review-machine --format {{.SSHConfig.RemoteUsername}}') printf 'core\\n' ;;
    'system connection list --format {{.Name}} {{.URI}}')
      printf 'review-machine ssh://core@127.0.0.1:%s/run/user/501/podman/podman.sock\\n' "\${TEST_CONNECTION_PORT:-2222}" ;;
    'machine ssh review-machine uname -m') printf 'x86_64\\n' ;;
    '--connection review-machine container exists '*) return "\${TEST_ENGINE_STATUS:-1}" ;;
    '--connection review-machine run '*) printf 'registry-owned-id\\n' ;;
    '--connection review-machine ps '*) printf 'workload-owned-id\\n' ;;
    '--connection review-machine rm -f workload-owned-id') return "\${TEST_CLEANUP_STATUS:-0}" ;;
  esac
}
bash() {
  printf 'lifecycle connection=%s platform=%s\\n' "$SMOKE_PODMAN_CONNECTION" "$SMOKE_PLATFORM" >&2
  return "\${TEST_SMOKE_STATUS:-0}"
}
source "$0"
`;

function run(overrides = {}) {
  return spawnSync('/bin/bash', ['-c', harness, script], {
    encoding: 'utf8',
    env: {
      PATH: process.env.PATH,
      PODMAN_SMOKE_MACHINE: 'review-machine',
      // A conflicting inherited default must not affect any engine command.
      CONTAINER_CONNECTION: 'other-machine',
      CONTAINER_HOST: 'ssh://other-machine/podman.sock',
      ...overrides,
    },
  });
}

test('pins all engine operations and lifecycle to the selected machine', () => {
  const result = run();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stderr, /lifecycle connection=review-machine platform=linux\/amd64/);
  const engineCalls = result.stderr.split('\n').filter((line) =>
    /^podman (?:--connection|pull|run|rm|ps|network|container) /.test(line),
  );
  assert.ok(engineCalls.length > 0);
  assert.ok(engineCalls.every((line) => line.startsWith('podman --connection review-machine ')));
  assert.match(result.stderr, /podman machine stop review-machine/);
});

test('rejects a connection that does not match machine SSH before starting the VM', () => {
  const result = run({ TEST_CONNECTION_PORT: '3333' });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /cannot resolve a matching/);
  assert.doesNotMatch(result.stderr, /podman machine start|podman --connection/);
});

test('reports cleanup failure and preserves revisions and environment', () => {
  const result = run({ TEST_CLEANUP_STATUS: '1' });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /cleanup failed: container workload-owned-id/);
  assert.doesNotMatch(result.stderr, /rm -rf|rm -f registry-owned-id|machine stop/);
});

test('does not mistake an engine connection failure for a missing Registry', () => {
  const result = run({ TEST_ENGINE_STATUS: '125' });
  assert.equal(result.status, 125);
  assert.match(result.stderr, /cannot inspect the selected Podman engine/);
  assert.doesNotMatch(result.stderr, /bash -s|podman --connection review-machine (?:pull|run|rm)/);
});

test('preserves dependencies and VM after lifecycle failure or explicit preserve', () => {
  for (const overrides of [
    { TEST_SMOKE_STATUS: '7' },
    { PODMAN_SMOKE_PRESERVE: 'true' },
  ]) {
    const result = run(overrides);
    assert.equal(result.status, Number(overrides.TEST_SMOKE_STATUS ?? 0));
    assert.match(result.stderr, /Smoke environment preserved/);
    assert.doesNotMatch(result.stderr, /podman --connection review-machine rm|machine stop|rm -rf/);
  }
});
