#!/bin/sh
set -eu

studio_script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
studio_dir=$(CDPATH= cd -- "$studio_script_dir/.." && pwd)
studio_backend_dir="$studio_dir/backend"
studio_repository_dir=$(CDPATH= cd -- "$studio_dir/.." && pwd)

studio_backend_address=${STUDIO_LOCAL_BACKEND_ADDRESS:-127.0.0.1:8080}
studio_frontend_host=${STUDIO_LOCAL_FRONTEND_HOST:-127.0.0.1}
studio_frontend_port=${STUDIO_LOCAL_FRONTEND_PORT:-3000}
studio_data_dir=${STUDIO_LOCAL_DATA_DIR:-"$studio_repository_dir/.cache/studio-local"}
studio_api_target="http://$studio_backend_address"
studio_backend_pid=""

studio_cleanup() {
  if [ -n "$studio_backend_pid" ] && kill -0 "$studio_backend_pid" 2>/dev/null; then
    kill "$studio_backend_pid" 2>/dev/null || true
    wait "$studio_backend_pid" 2>/dev/null || true
  fi
}
trap studio_cleanup EXIT INT TERM

for studio_command in go node npm curl; do
  if ! command -v "$studio_command" >/dev/null 2>&1; then
    echo "required command not found: $studio_command" >&2
    exit 1
  fi
done

if [ ! -d "$studio_dir/node_modules" ]; then
  echo "studio/node_modules is missing; run 'cd studio && npm ci' first." >&2
  exit 1
fi

echo "Starting Studio Backend at $studio_api_target"
go -C "$studio_backend_dir" run ./cmd/studio-api \
  --address "$studio_backend_address" \
  --data-dir "$studio_data_dir" &
studio_backend_pid=$!

studio_backend_ready=false
studio_attempt=0
while [ "$studio_attempt" -lt 50 ]; do
  if curl --fail --silent --show-error "$studio_api_target/healthz" >/dev/null 2>&1; then
    studio_backend_ready=true
    break
  fi
  if ! kill -0 "$studio_backend_pid" 2>/dev/null; then
    break
  fi
  studio_attempt=$((studio_attempt + 1))
  sleep 0.1
done

if [ "$studio_backend_ready" != true ]; then
  echo "Studio Backend did not become ready." >&2
  exit 1
fi

echo "Studio Frontend: http://$studio_frontend_host:$studio_frontend_port"
echo "Studio Backend:  $studio_api_target"
echo "Press Ctrl-C to stop both processes."

cd "$studio_dir"
STUDIO_API_PROXY_TARGET="$studio_api_target" \
  npm run dev -- --host "$studio_frontend_host" --port "$studio_frontend_port"
