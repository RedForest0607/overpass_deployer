#!/usr/bin/env bash
set -euo pipefail

machine="${PODMAN_SMOKE_MACHINE:-podman-machine-default}"
compose_version="${PODMAN_SMOKE_COMPOSE_VERSION:-v2.29.7}"
registry_image="${PODMAN_SMOKE_REGISTRY_IMAGE:-docker.io/library/registry:2}"
registry_name="crosswalk-compose-smoke-registry"
registry_port="${PODMAN_SMOKE_REGISTRY_PORT:-5500}"
plugin_path="/var/home/core/.docker/cli-plugins/docker-compose"
registry_config="/etc/containers/registries.conf.d/crosswalk-compose-smoke.conf"
timestamp="$(date -u +%Y%m%d%H%M%S)"
preserve="${PODMAN_SMOKE_PRESERVE:-false}"

if [[ ! "$machine" =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]*$ || ! "$compose_version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ || ! "$registry_port" =~ ^[0-9]{1,5}$ || ( "$preserve" != "false" && "$preserve" != "true" ) ]]; then
  echo "invalid Podman machine, Compose version, Registry port or preserve setting" >&2
  exit 2
fi
if (( 10#$registry_port < 1 || 10#$registry_port > 65535 )); then
  echo "Registry port must be between 1 and 65535" >&2
  exit 2
fi

# Machine SSH and engine operations must use the same rootless connection.
machine_state="$(podman machine inspect "$machine" --format '{{.State}}')"
ssh_port="$(podman machine inspect "$machine" --format '{{.SSHConfig.Port}}')"
identity_path="$(podman machine inspect "$machine" --format '{{.SSHConfig.IdentityPath}}')"
ssh_user="$(podman machine inspect "$machine" --format '{{.SSHConfig.RemoteUsername}}')"
connection_uri="$(podman system connection list --format '{{.Name}} {{.URI}}' | awk -v name="$machine" '$1 == name {print $2}')"
if [[ ! "$ssh_port" =~ ^[0-9]+$ || ! -f "$identity_path" || "$ssh_user" != "core" || "$connection_uri" != "ssh://core@127.0.0.1:$ssh_port/run/user/"*"/podman/podman.sock" ]]; then
  echo "cannot resolve a matching core/rootless Podman connection for $machine" >&2
  exit 1
fi

engine() { podman --connection "$machine" "$@"; }
export SMOKE_PROJECT="crosswalk-smoke-podman-$timestamp"
export SMOKE_REMOTE_DIR="/var/home/core/crosswalk-smoke/$timestamp"

started_here=false
provider_owned=false
registry_owned=false
smoke_started=false
smoke_succeeded=false
cleanup_failed=false

cleanup_step() {
  local description="$1"
  shift
  if ! "$@"; then
    printf 'cleanup failed: %s\n' "$description" >&2
    cleanup_failed=true
  fi
}

cleanup() {
  local status=$?
  set +e
  if [[ "$smoke_started" == true && ( "$preserve" == true || "$smoke_succeeded" == false ) ]]; then
    printf 'Smoke environment preserved on %s at %s for inspection.\n' "$machine" "$SMOKE_REMOTE_DIR" >&2
    exit "$status"
  fi
  if [[ "$smoke_succeeded" == true ]]; then
    local containers container
    if containers="$(engine ps -aq --filter "label=com.docker.compose.project=$SMOKE_PROJECT")"; then
      while IFS= read -r container; do
        if [[ -n "$container" ]]; then
          cleanup_step "container $container" engine rm -f "$container"
        fi
      done <<< "$containers"
    else
      echo "cleanup failed: list smoke containers" >&2
      cleanup_failed=true
    fi
    cleanup_step "smoke network" engine network rm "${SMOKE_PROJECT}_default"
    if [[ "$cleanup_failed" == false ]]; then
      cleanup_step "smoke revisions" podman machine ssh "$machine" "rm -rf -- '$SMOKE_REMOTE_DIR'"
    else
      echo "Smoke revisions preserved because workload cleanup failed." >&2
      exit 1
    fi
  fi
  if [[ "$registry_owned" == true ]]; then
    cleanup_step "smoke Registry" engine rm -f "$registry_id"
  fi
  if [[ "$provider_owned" == true ]]; then
    cleanup_step "Compose provider and Registry configuration" podman machine ssh "$machine" "set -e; rm -f '$plugin_path' '$plugin_path.download' '$plugin_path.sha256'; sudo rm -f '$registry_config'"
  fi
  if [[ "$started_here" == true ]]; then
    cleanup_step "Podman machine stop" podman machine stop "$machine"
  fi
  if [[ "$status" == 0 && "$cleanup_failed" == true ]]; then
    exit 1
  fi
  exit "$status"
}
trap cleanup EXIT

if [[ "$machine_state" != running ]]; then
  podman machine start "$machine"
  started_here=true
fi
architecture="$(podman machine ssh "$machine" 'uname -m')"
case "$architecture" in
  aarch64|arm64) export SMOKE_PLATFORM=linux/arm64 ;;
  x86_64) export SMOKE_PLATFORM=linux/amd64 ;;
  *) echo "unsupported Podman machine architecture: $architecture" >&2; exit 1 ;;
esac
if engine container exists "$registry_name"; then
  echo "Podman smoke Registry container already exists; refusing to replace it" >&2
  exit 1
else
  exists_status=$?
  if [[ "$exists_status" != 1 ]]; then
    echo "cannot inspect the selected Podman engine; refusing to prepare smoke resources" >&2
    exit "$exists_status"
  fi
fi
if ! podman machine ssh "$machine" "test ! -e '$plugin_path' && test ! -e '$registry_config'"; then
  echo "Compose provider or smoke Registry config already exists; refusing to replace it" >&2
  exit 1
fi
provider_owned=true

podman machine ssh "$machine" "bash -s -- '$compose_version' '$registry_port'" < "$(dirname "$0")/prepare-podman-provider.sh"

engine pull "$registry_image"
registry_id="$(engine run -d --name "$registry_name" -p "127.0.0.1:$registry_port:5000" "$registry_image")"
registry_owned=true
for _ in $(seq 1 30); do
  if podman machine ssh "$machine" "curl -fsS http://localhost:$registry_port/v2/ >/dev/null"; then
    break
  fi
  sleep 1
done
podman machine ssh "$machine" "curl -fsS http://localhost:$registry_port/v2/ >/dev/null"

export CROSSWALK_SMOKE_TOKEN=fixture-token
export SMOKE_REGISTRY="localhost:$registry_port"
export SMOKE_REGISTRY_USERNAME=fixture
export SMOKE_REGISTRY_TOKEN_ENV=CROSSWALK_SMOKE_TOKEN
export SMOKE_REGISTRY_INSECURE=true
export SMOKE_CONTAINER_CLI=podman
export SMOKE_PODMAN_CONNECTION="$machine"
export SMOKE_SSH_HOST=127.0.0.1
export SMOKE_SSH_PORT="$ssh_port"
export SMOKE_SSH_USER="$ssh_user"
export SMOKE_SSH_KEY="$identity_path"

smoke_started=true
bash "$(dirname "$0")/run-real-smoke.sh"
smoke_succeeded=true
