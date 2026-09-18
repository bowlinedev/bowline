#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

if ! command -v golangci-lint >/dev/null; then
  echo "golangci-lint not installed; skipping"
  exit 0
fi

for m in . cmd/bowline transport/websocket mcp otel agent playground contracttest conformance gateway registry adapters/fiber examples/ledger examples/federation/billing examples/nethttp-minimal examples/go-client examples/routers/*; do
  [ -f "$m/go.mod" ] || continue
  printf -- '-- %s\n' "$m"
  (cd "$m" && golangci-lint run -c "$repo/.golangci.yml" --timeout 5m ./...)
done
