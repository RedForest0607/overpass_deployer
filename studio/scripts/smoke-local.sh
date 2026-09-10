#!/bin/sh
set -eu

studio_frontend_url=${STUDIO_LOCAL_FRONTEND_URL:-http://127.0.0.1:3000}
studio_backend_url=${STUDIO_LOCAL_BACKEND_URL:-http://127.0.0.1:8080}

for studio_command in curl node; do
  if ! command -v "$studio_command" >/dev/null 2>&1; then
    echo "required command not found: $studio_command" >&2
    exit 1
  fi
done

studio_health=$(curl --fail --silent --show-error "$studio_backend_url/healthz")
node -e '
const value = JSON.parse(process.argv[1]);
if (value.status !== "ok") process.exit(1);
' "$studio_health"

curl --fail --silent --show-error "$studio_frontend_url/" >/dev/null
studio_capabilities=$(curl --fail --silent --show-error "$studio_frontend_url/api/v1/capabilities")
node -e '
const value = JSON.parse(process.argv[1]);
if (!value.blueprintVersions?.includes("studio.deployer/v1")) process.exit(1);
if (!Array.isArray(value.generators)) process.exit(1);
' "$studio_capabilities"

echo "Local smoke test passed: frontend, backend, and /api proxy are reachable."
