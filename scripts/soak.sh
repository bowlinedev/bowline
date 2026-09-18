#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"
seconds="${1:-120}"

case "$seconds" in
  ''|*[!0-9]*) echo "usage: soak.sh [seconds]" >&2; exit 2 ;;
esac

echo "== soak for ${seconds}s per case"
BOWLINE_SOAK="$seconds" go test -run TestSoak -v -timeout "$((seconds * 4 + 120))s" .
