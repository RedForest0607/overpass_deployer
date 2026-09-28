#!/usr/bin/env bash
set -euo pipefail

state_dir="${COMPOSE_FIXTURE_STATE:?COMPOSE_FIXTURE_STATE is required}"
mkdir -p "$state_dir"

increment() {
  local target="$state_dir/$1"
  local value=0
  if [[ -f "$target" ]]; then
    value="$(<"$target")"
  fi
  printf '%s\n' "$((value + 1))" > "$target"
}

compose_file=""
compose_action=""
for ((index = 1; index <= $#; index++)); do
  argument="${!index}"
  if [[ "$argument" == "--file" ]]; then
    next=$((index + 1))
    compose_file="${!next}"
  fi
  case "$argument" in
    config|up|ps|down) compose_action="$argument" ;;
  esac
done

case "${1:-}" in
  version)
    printf '28.4.0\n'
    ;;
  login)
    read -r token
    [[ "$token" == "${COMPOSE_FIXTURE_TOKEN:?}" ]]
    printf 'logged-in\n' > "$state_dir/login"
    ;;
  ps)
    # Initial ownership check. The fixture starts without unmanaged containers.
    ;;
  pull)
    printf '%s\n' "${2:?image is required}" >> "$state_dir/images"
    ;;
  image)
    case "${2:-}" in
      ls)
        [[ ! -f "$state_dir/images" ]] || sort -u "$state_dir/images"
        ;;
      inspect)
        image="${@: -1}"
        grep -Fxq "$image" "$state_dir/images"
        printf '["%s"]\n' "$image"
        ;;
      *) exit 2 ;;
    esac
    ;;
  compose)
    if [[ "${2:-}" == "version" ]]; then
      printf '2.29.7\n'
      exit 0
    fi
    [[ -n "$compose_file" && -f "$compose_file" ]]
    case "$compose_action" in
      config)
        if [[ " $* " == *" --services "* ]]; then
          awk '/^  [A-Za-z0-9_.-]+:$/ {name=$1; sub(/:$/, "", name); print name}' "$compose_file"
        elif [[ " $* " == *" --images "* ]]; then
          awk '$1 == "image:" {print $2}' "$compose_file"
        else
          grep -Fq 'services:' "$compose_file"
          grep -Fq 'image:' "$compose_file"
        fi
        ;;
      up)
        increment up-count
        release="$(awk '$1 == "RELEASE:" {gsub(/\"/, "", $2); print $2}' "$compose_file")"
        printf '%s\n' "$release" > "$state_dir/last-release"
        if grep -Fq 'FAIL_HEALTH: "true"' "$compose_file"; then
          printf 'unhealthy\n' > "$state_dir/health"
        else
          printf 'healthy\n' > "$state_dir/health"
        fi
        ;;
      ps)
        health="$(<"$state_dir/health")"
        printf '{"Service":"api","State":"running","Health":"%s"}\n' "$health"
        ;;
      down)
        increment down-count
        printf 'stopped\n' > "$state_dir/health"
        ;;
      *) exit 2 ;;
    esac
    ;;
  *)
    exit 2
    ;;
esac
