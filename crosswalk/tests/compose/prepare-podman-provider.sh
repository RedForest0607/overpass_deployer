#!/usr/bin/env bash
# Runs inside the selected Podman machine; lifecycle orchestration stays on the host.
set -euo pipefail

compose_version="$1"
registry_port="$2"
case "$(uname -m)" in
  aarch64|arm64) architecture=aarch64 ;;
  x86_64) architecture=x86_64 ;;
  *) echo "unsupported Compose provider architecture" >&2; exit 1 ;;
esac

plugin_path=/var/home/core/.docker/cli-plugins/docker-compose
registry_config=/etc/containers/registries.conf.d/crosswalk-compose-smoke.conf
plugin_url="https://github.com/docker/compose/releases/download/$compose_version/docker-compose-linux-$architecture"

install -d -m 0700 /var/home/core/.docker/cli-plugins
curl -fsSL "$plugin_url" -o "$plugin_path.download"
curl -fsSL "$plugin_url.sha256" -o "$plugin_path.sha256"
expected="$(awk '{print $1}' "$plugin_path.sha256")"
actual="$(sha256sum "$plugin_path.download" | awk '{print $1}')"
if [[ ! "$expected" =~ ^[a-f0-9]{64}$ || "$expected" != "$actual" ]]; then
  echo "Compose provider checksum mismatch" >&2
  exit 1
fi
mv "$plugin_path.download" "$plugin_path"
chmod 0700 "$plugin_path"
rm -f "$plugin_path.sha256"
printf '[[registry]]\nlocation = "localhost:%s"\ninsecure = true\n' "$registry_port" | sudo tee "$registry_config" >/dev/null
docker compose version --short
