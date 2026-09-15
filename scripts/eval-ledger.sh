#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
mode="${1:-replay}"
recording="evals/list-and-get.json"

cleanup() {
  if [ -n "${server_pid:-}" ]; then kill "$server_pid" 2>/dev/null || true; fi
  rm -rf "$work"
}
trap cleanup EXIT

(cd "$repo/cmd/bowline" && go build -buildvcs=false -o "$work/bowline" .)
(cd "$repo/examples/ledger" && go build -buildvcs=false -o "$work/ledger" ./cmd/server)

ADDR=127.0.0.1:18080 LEDGER_TOKEN=dev LEDGER_FIXED_TIME=2026-09-15T12:00:00Z "$work/ledger" >"$work/server.log" 2>&1 &
server_pid=$!

for _ in $(seq 1 50); do
  if curl -fsS "http://127.0.0.1:18080/api/health" >/dev/null 2>&1; then break; fi
  sleep 0.2
done
curl -fsS "http://127.0.0.1:18080/api/health" >/dev/null

cd "$repo/examples/ledger"
case "$mode" in
  record)
    "$work/bowline" eval record --backend url --url http://127.0.0.1:18080/api --script evals/script.json --out "$recording" --header "Authorization: Bearer dev"
    ;;
  replay)
    "$work/bowline" eval replay "$recording" --backend url --url http://127.0.0.1:18080/api --header "Authorization: Bearer dev"
    ;;
  *)
    echo "usage: $0 [record|replay]" >&2
    exit 2
    ;;
esac
