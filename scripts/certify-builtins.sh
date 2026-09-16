#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(ts go dart python rust elixir)
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
(cd cmd/bowline && go build -buildvcs=false -o "$work/bowline" .)

status=0
for target in "${targets[@]}"; do
  printf '\n== %s\n' "$target"
  if ! "$work/bowline" certify --config "cmd/bowline/certify/$target.json" --report; then
    status=1
  fi
done

if [ "$status" -ne 0 ]; then
  echo "certify-builtins: at least one target is not certified" >&2
fi
exit "$status"
