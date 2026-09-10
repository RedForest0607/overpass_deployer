#!/usr/bin/env bash
set -euo pipefail

case "${1:-}" in
  start|stop|restart)
    exit 0
    ;;
  *)
    echo "usage: $0 {start|stop|restart}" >&2
    exit 2
    ;;
esac
