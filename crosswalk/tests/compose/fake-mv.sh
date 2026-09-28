#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "-Tf" ]]; then
  shift
  [[ "$#" == 2 ]]
  exec /usr/bin/perl -e 'rename($ARGV[0], $ARGV[1]) or die "rename: $!\n"' "$1" "$2"
fi
exec /bin/mv "$@"
