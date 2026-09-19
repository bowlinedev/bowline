#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

version="${1:?usage: repin-modules.sh vX.Y.Z [module ...]}"
shift
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "version must look like vX.Y.Z" >&2; exit 2 ;;
esac

modules=("$@")
if [ "${#modules[@]}" -eq 0 ]; then
  modules=(transport/websocket mcp agent contracttest registry adapters/fiber gateway otel stores/sql stores/redis cmd/bowline)
fi

for module in "${modules[@]}"; do
  [ -f "$module/go.mod" ] || { echo "no module at $module" >&2; exit 1; }
  own="$(cd "$module" && GOWORK=off go mod edit -json | jq -r .Module.Path)"
  deps="$(cd "$module" && GOWORK=off go mod edit -json |
    jq -r --arg own "$own" '.Require[]? | select(.Path | startswith("github.com/bowlinedev/bowline")) | select(.Path != $own) | .Path')"
  [ -n "$deps" ] || { echo "-- $module has no bowline requires"; continue; }
  while IFS= read -r dep; do
    [ -n "$dep" ] || continue
    (cd "$module" && GOWORK=off go mod edit -require="$dep@$version")
    echo "-- $module now requires $dep@$version"
  done <<< "$deps"
  (cd "$module" && GOWORK=off go mod tidy)
done

echo "repinned ${#modules[@]} module(s) to $version"
