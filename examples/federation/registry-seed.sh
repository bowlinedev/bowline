#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../.." && pwd)"
store="${1:-$repo/examples/federation/registry-data}"
listen="${BOWLINE_REGISTRY_LISTEN:-127.0.0.1:8095}"
registry="http://$listen"
token="${BOWLINE_REGISTRY_TOKEN:-federation-demo-token}"
work="$(mktemp -d)"

cleanup() {
  if [ -n "${server:-}" ]; then kill "$server" 2>/dev/null || true; fi
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

(cd "$repo/cmd/bowline" && go build -buildvcs=false -o "$work/bowline" .)

rm -rf "$store"
mkdir -p "$store"
"$work/bowline" registry serve --store "$store" --listen "$listen" --token "$token" >"$work/registry.log" 2>&1 &
server=$!

for _ in $(seq 1 100); do
  if curl -fsS "$registry/v1/healthz" >/dev/null 2>&1; then break; fi
  sleep 0.2
done
curl -fsS "$registry/v1/healthz" >/dev/null

export BOWLINE_REGISTRY_TOKEN="$token"

(cd "$repo/examples/ledger" && "$work/bowline" publish --registry "$registry" --service ledger --tag main)
(cd "$repo/examples/federation/billing" && "$work/bowline" publish --registry "$registry" --service billing --tag main)
(cd "$repo/examples/ledger" && "$work/bowline" publish --registry "$registry" --consumer ledger-web --provider ledger --usage contracts/consumers/ledger-web.json)

echo "seeded $store from $registry"
