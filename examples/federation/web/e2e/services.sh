#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(cd "$here/../../.." && pwd)"
work="$(mktemp -d)"

cleanup() {
  for pid in ${pids:-}; do kill "$pid" 2>/dev/null || true; done
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

(cd "$repo/examples/ledger" && go build -buildvcs=false -o "$work/ledger" ./cmd/server)
(cd "$repo/examples/federation/billing" && go build -buildvcs=false -o "$work/billing" ./cmd/server)
(cd "$repo/cmd/bowline" && go build -buildvcs=false -o "$work/bowline" .)

export LEDGER_FIXED_TIME=2026-09-15T12:00:00Z
export BILLING_FIXED_TIME=2026-09-15T12:00:00Z
export BILLING_SIGNING_KEY=billing-2026
export BILLING_SIGNING_SECRET=federation-demo-secret
export LEDGER_INBOUND_KEY=billing-2026
export LEDGER_INBOUND_SECRET=federation-demo-secret

ADDR=127.0.0.1:8080 "$work/ledger" >"$work/ledger.log" 2>&1 &
pids="$!"
ADDR=127.0.0.1:8081 LEDGER_URL=http://127.0.0.1:8080/api "$work/billing" >"$work/billing.log" 2>&1 &
pids="$pids $!"

for _ in $(seq 1 100); do
  if curl -fsS http://127.0.0.1:8080/api/health >/dev/null 2>&1 && curl -fsS http://127.0.0.1:8081/api/health >/dev/null 2>&1; then break; fi
  sleep 0.2
done

cd "$repo/examples/federation/gateway"
exec "$work/bowline" gateway
