#!/usr/bin/env bash
set -euo pipefail

target="${1:-}"
case "$target" in
  ubuntu)
    image="localhost/overpass-m2-ubuntu-ssh"
    container="overpass-m2-ubuntu-ssh"
    containerfile="tests/podman/ubuntu-ssh/Containerfile"
    context_dir="tests/podman/ubuntu-ssh"
    jdk_major="21"
    ;;
  amazonlinux)
    image="localhost/overpass-m2-amazonlinux-ssh"
    container="overpass-m2-amazonlinux-ssh"
    containerfile="tests/podman/amazonlinux-ssh/Containerfile"
    context_dir="tests/podman/amazonlinux-ssh"
    jdk_major="17"
    ;;
  *)
    echo "usage: $0 {ubuntu|amazonlinux}" >&2
    exit 2
    ;;
esac

root_dir="$(cd "$(dirname "$0")/../.." && pwd)"
temp_dir="$(mktemp -d -t overpass-m2-idempotency.XXXXXX)"

cleanup() {
  podman rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$temp_dir"
}
trap cleanup EXIT

ssh-keygen -q -t ed25519 -N "" -f "$temp_dir/id_ed25519"
podman build -t "$image" -f "$root_dir/$containerfile" "$root_dir/$context_dir"
podman run -d --name "$container" -p 127.0.0.1::22 -e "AUTHORIZED_KEY=$(cat "$temp_dir/id_ed25519.pub")" "$image" >/dev/null
sleep 1
if ! podman ps --format '{{.Names}}' | grep -Fxq "$container"; then
  podman logs "$container" >&2
  exit 1
fi

port="$(podman port "$container" 22/tcp | head -n 1 | awk -F: '{print $NF}')"
for _ in $(seq 1 20); do
  if ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "$temp_dir/id_ed25519" -p "$port" deploy@127.0.0.1 true >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i "$temp_dir/id_ed25519" -p "$port" deploy@127.0.0.1 true >/dev/null 2>&1; then
  echo "SSH target did not become ready" >&2
  exit 1
fi

go -C "$root_dir" build -o "$temp_dir/deploy" ./cmd/deploy
cat > "$temp_dir/deploy.yml" <<EOF
ssh:
  user: deploy
  key_path: $temp_dir/id_ed25519
  known_hosts_path: $temp_dir/known_hosts
  host_key_checking: accept-new
  port: $port
  timeout_sec: 5
bootstrap:
  jdk:
    vendor: corretto
    major: $jdk_major
    headless: true
servers:
  - name: m2-$target
    host: 127.0.0.1
    app:
      name: m2-app
      base_dir: /home/deploy/overpass/m2-app
      port: 18080
      jar:
        local_path: $root_dir/tests/podman/fixtures/sample.jar
        remote_path: /home/deploy/overpass/m2-app/lib/sample.jar
      config_files:
        - local_path: $root_dir/tests/podman/fixtures/application.yml
          remote_path: /home/deploy/overpass/m2-app/config/application.yml
      script:
        mode: local-file
        local_path: $root_dir/tests/podman/fixtures/server.sh
        remote_path: /home/deploy/overpass/m2-app/bin/server.sh
EOF

"$temp_dir/deploy" vm --config "$temp_dir/deploy.yml" > "$temp_dir/first.log"
"$temp_dir/deploy" vm --config "$temp_dir/deploy.yml" > "$temp_dir/second.log"

grep -Fq "done m2-$target (127.0.0.1)" "$temp_dir/first.log"
grep -Fq "skipped m2-$target (127.0.0.1)" "$temp_dir/second.log"
grep -Fq "Totals: done=0 skipped=1 failed=0" "$temp_dir/second.log"
podman exec "$container" java -version >/dev/null 2>&1

if ! podman exec "$container" sh -lc '
  for file in /home/deploy/overpass/m2-app/lib/* /home/deploy/overpass/m2-app/config/* /home/deploy/overpass/m2-app/bin/*; do
    case "$file" in
      *.deploy-upload-*) exit 1 ;;
    esac
  done
'; then
  echo "temporary deployment file was left on the remote host" >&2
  exit 1
fi

echo "$target M2 idempotency test passed"
