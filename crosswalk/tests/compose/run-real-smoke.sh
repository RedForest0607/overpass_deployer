#!/usr/bin/env bash
set -euo pipefail

required=(
  SMOKE_REGISTRY
  SMOKE_REGISTRY_USERNAME
  SMOKE_REGISTRY_TOKEN_ENV
  SMOKE_SSH_HOST
  SMOKE_SSH_USER
  SMOKE_SSH_KEY
  SMOKE_PROJECT
  SMOKE_REMOTE_DIR
)
for name in "${required[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    printf 'required environment variable is missing: %s\n' "$name" >&2
    exit 2
  fi
done

if [[ ! "$SMOKE_REGISTRY" =~ ^[a-zA-Z0-9][a-zA-Z0-9.-]*(:[0-9]+)?$ ]]; then
  echo "SMOKE_REGISTRY must be a hostname with an optional port" >&2
  exit 2
fi
if [[ ! "$SMOKE_REGISTRY_USERNAME" =~ ^[a-zA-Z0-9_.@+$-]+$ ]]; then
  echo "SMOKE_REGISTRY_USERNAME contains unsupported characters" >&2
  exit 2
fi
if [[ ! "$SMOKE_REGISTRY_TOKEN_ENV" =~ ^[A-Z_][A-Z0-9_]*$ ]]; then
  echo "SMOKE_REGISTRY_TOKEN_ENV must be an environment variable name" >&2
  exit 2
fi
if [[ -z "${!SMOKE_REGISTRY_TOKEN_ENV:-}" ]]; then
  echo "the registry token environment variable is empty" >&2
  exit 2
fi
if [[ ! "$SMOKE_PROJECT" =~ ^crosswalk-smoke-[a-z0-9-]+$ ]]; then
  echo "SMOKE_PROJECT must start with crosswalk-smoke- and use lowercase letters, numbers, or hyphens" >&2
  exit 2
fi
if [[ ! "$SMOKE_SSH_HOST" =~ ^[a-zA-Z0-9][a-zA-Z0-9.-]*$ || ! "$SMOKE_SSH_USER" =~ ^[a-z_][a-z0-9_-]*$ ]]; then
  echo "SMOKE_SSH_HOST or SMOKE_SSH_USER contains unsupported characters" >&2
  exit 2
fi
if [[ ! "$SMOKE_REMOTE_DIR" =~ ^/[a-zA-Z0-9_./-]+$ || "$SMOKE_REMOTE_DIR" == "/" ]]; then
  echo "SMOKE_REMOTE_DIR must be a non-root absolute path without spaces" >&2
  exit 2
fi
if [[ "$SMOKE_REMOTE_DIR" == *"//"* || "$SMOKE_REMOTE_DIR" == *"/./"* || "$SMOKE_REMOTE_DIR" == *"/../"* || "$SMOKE_REMOTE_DIR" == */. || "$SMOKE_REMOTE_DIR" == */.. ]]; then
  echo "SMOKE_REMOTE_DIR must be a clean absolute path" >&2
  exit 2
fi
if [[ ! "$SMOKE_SSH_KEY" =~ ^/[a-zA-Z0-9_./-]+$ || ! -f "$SMOKE_SSH_KEY" ]]; then
  echo "SMOKE_SSH_KEY must be an existing absolute path without spaces" >&2
  exit 2
fi

ssh_port="${SMOKE_SSH_PORT:-22}"
parallel="${SMOKE_PARALLEL:-1}"
platform="${SMOKE_PLATFORM:-linux/amd64}"
container_cli="${SMOKE_CONTAINER_CLI:-docker}"
podman_connection="${SMOKE_PODMAN_CONNECTION:-}"
registry_insecure="${SMOKE_REGISTRY_INSECURE:-false}"
case "$platform" in
  linux/amd64) goarch=amd64 ;;
  linux/arm64) goarch=arm64 ;;
  *) echo "SMOKE_PLATFORM must be linux/amd64 or linux/arm64" >&2; exit 2 ;;
esac
if [[ ! "$ssh_port" =~ ^[0-9]+$ || ! "$parallel" =~ ^[1-9][0-9]*$ ]]; then
  echo "SMOKE_SSH_PORT and SMOKE_PARALLEL must be positive integers" >&2
  exit 2
fi
if [[ "$container_cli" != "docker" && "$container_cli" != "podman" ]]; then
  echo "SMOKE_CONTAINER_CLI must be docker or podman" >&2
  exit 2
fi
container_command=("$container_cli")
if [[ "$container_cli" == "podman" && -n "$podman_connection" ]]; then
  container_command+=(--connection "$podman_connection")
fi
if [[ "$registry_insecure" != "false" && "$registry_insecure" != "true" ]]; then
  echo "SMOKE_REGISTRY_INSECURE must be true or false" >&2
  exit 2
fi
if [[ "$container_cli" == "docker" && "$registry_insecure" == "true" ]]; then
  echo "insecure Docker registries must be configured on the daemon; the smoke script will not modify daemon settings" >&2
  exit 2
fi

root_dir="$(cd "$(dirname "$0")/../.." && pwd)"
work_dir="$(mktemp -d -t crosswalk-compose-real-smoke.XXXXXX)"
docker_config="$work_dir/.docker"
registry_auth_file="$docker_config/auth.json"
build_context="$work_dir/build"
mkdir -m 0700 "$docker_config"
mkdir -m 0700 "$build_context"
cleanup_credentials() {
  rm -rf -- "$docker_config"
}
trap cleanup_credentials EXIT
export DOCKER_CONFIG="$docker_config"

repository="${SMOKE_IMAGE_REPOSITORY:-$SMOKE_REGISTRY/crosswalk/compose-smoke}"
if [[ ! "$repository" =~ ^[a-z0-9.-]+(:[0-9]+)?/[a-z0-9._/-]+$ || "$repository" != "$SMOKE_REGISTRY/"* ]]; then
  echo "SMOKE_IMAGE_REPOSITORY must be a lowercase repository under SMOKE_REGISTRY" >&2
  exit 2
fi
tag="$repository:$(date -u +%Y%m%d%H%M%S)"
token="${!SMOKE_REGISTRY_TOKEN_ENV}"
compose_file="$work_dir/compose.yml"
config_file="$work_dir/deploy.yml"
known_hosts="$work_dir/known_hosts"
precheck_known_hosts="$work_dir/precheck_known_hosts"
deploy_binary="$work_dir/deploy"
ssh_options=(-i "$SMOKE_SSH_KEY" -p "$ssh_port" -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile="$precheck_known_hosts")

printf 'Smoke artifacts: %s\n' "$work_dir"
if ! ssh "${ssh_options[@]}" "$SMOKE_SSH_USER@$SMOKE_SSH_HOST" "test ! -e '$SMOKE_REMOTE_DIR'"; then
  echo "SMOKE_REMOTE_DIR already exists; refusing to reuse or delete it" >&2
  exit 1
fi
registry_security_args=()
if [[ "$registry_insecure" == "true" ]]; then
  registry_security_args+=(--tls-verify=false)
fi
if [[ "$container_cli" == "podman" ]]; then
  printf '%s\n' "$token" | "${container_command[@]}" login "${registry_security_args[@]}" --authfile "$registry_auth_file" "$SMOKE_REGISTRY" --username "$SMOKE_REGISTRY_USERNAME" --password-stdin >/dev/null
else
  printf '%s\n' "$token" | docker login "$SMOKE_REGISTRY" --username "$SMOKE_REGISTRY_USERNAME" --password-stdin >/dev/null
fi

CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go -C "$root_dir" build -o "$build_context/compose-fixture" ./tests/compose/fixtureapp
if [[ "$container_cli" == "podman" ]]; then
  "${container_command[@]}" build --format docker --platform "$platform" -f "$root_dir/tests/compose/fixtureapp/Containerfile" -t "$tag" "$build_context"
  "${container_command[@]}" push "${registry_security_args[@]}" --authfile "$registry_auth_file" "$tag" 2>&1 | tee "$work_dir/push.log"
else
  docker build --platform "$platform" -f "$root_dir/tests/compose/fixtureapp/Containerfile" -t "$tag" "$build_context"
  docker push "$tag" 2>&1 | tee "$work_dir/push.log"
fi
digest="$(awk '/digest: sha256:/ {for (i = 1; i <= NF; i++) if (index($i, "sha256:") == 1) value=$i} END {print value}' "$work_dir/push.log")"
if [[ -z "$digest" && "$container_cli" == "podman" ]]; then
  digest="$("${container_command[@]}" image inspect --format '{{.Digest}}' "$tag")"
fi
if [[ ! "$digest" =~ ^sha256:[a-f0-9]{64}$ ]]; then
  echo "cannot resolve the pushed manifest digest" >&2
  exit 1
fi
image_ref="$repository@$digest"

go -C "$root_dir" build -o "$deploy_binary" ./cmd/deploy
cat > "$config_file" <<EOF
format: crosswalk/v1
ssh:
  user: "$SMOKE_SSH_USER"
  key_path: "$SMOKE_SSH_KEY"
  known_hosts_path: "$known_hosts"
  host_key_checking: accept-new
  port: $ssh_port
  timeout_sec: 10
docker:
  registry:
    host: "$SMOKE_REGISTRY"
    username: "$SMOKE_REGISTRY_USERNAME"
    token_env: "$SMOKE_REGISTRY_TOKEN_ENV"
servers:
  - name: compose-real-smoke
    host: "$SMOKE_SSH_HOST"
    tags: [smoke]
    compose_projects:
      - name: "$SMOKE_PROJECT"
        local_path: "$compose_file"
        remote_dir: "$SMOKE_REMOTE_DIR"
EOF

write_compose() {
  local release="$1"
  local fail_health="$2"
  cat > "$compose_file" <<EOF
name: $SMOKE_PROJECT
services:
  api:
    image: $image_ref
    environment:
      RELEASE: "$release"
      FAIL_HEALTH: "$fail_health"
    restart: unless-stopped
EOF
}

write_compose v1 false
"$deploy_binary" compose --dry-run --config "$config_file" | tee "$work_dir/dry-run.log"
"$deploy_binary" compose --preflight --config "$config_file" | tee "$work_dir/preflight.log"
"$deploy_binary" compose --apply --config "$config_file" --parallel "$parallel" | tee "$work_dir/apply-v1.log"

"$deploy_binary" compose --apply --config "$config_file" --parallel "$parallel" | tee "$work_dir/reapply-v1.log"
grep -Fq "skipped compose-real-smoke" "$work_dir/reapply-v1.log"

write_compose v2 false
v2_revision="$(sha256sum "$compose_file" | awk '{print $1}')"
"$deploy_binary" compose --apply --config "$config_file" --parallel "$parallel" | tee "$work_dir/apply-v2.log"

write_compose broken true
if "$deploy_binary" compose --apply --config "$config_file" --parallel "$parallel" >"$work_dir/apply-broken.log" 2>&1; then
  echo "unhealthy revision unexpectedly succeeded" >&2
  exit 1
fi
grep -Fq "previous revision restored" "$work_dir/apply-broken.log"

active="$(ssh "${ssh_options[@]}" "$SMOKE_SSH_USER@$SMOKE_SSH_HOST" "readlink '$SMOKE_REMOTE_DIR/active'")"
if [[ "$active" != ".revisions/$v2_revision" ]]; then
  echo "rollback active revision mismatch: $active" >&2
  exit 1
fi
ssh "${ssh_options[@]}" "$SMOKE_SSH_USER@$SMOKE_SSH_HOST" \
  "docker compose --project-name '$SMOKE_PROJECT' --file '$SMOKE_REMOTE_DIR/active/compose.yml' ps --all"

printf 'M7 real Compose smoke passed. Remote revision data was preserved at %s.\n' "$SMOKE_REMOTE_DIR"
printf 'Review logs and non-secret artifacts at %s.\n' "$work_dir"
