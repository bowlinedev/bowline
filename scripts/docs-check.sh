#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"

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

export PATH="$work:$PATH"
cd "$repo/examples/ledger"
status=0
for guide in "$repo"/docs/guides/*.md; do
  awk '
    /^```bash runnable$/ { inside = 1; block = ""; next }
    /^```$/ && inside { inside = 0; print block; print "\x1e"; next }
    inside { block = block $0 "\n" }
  ' "$guide" | while IFS= read -r -d $'\x1e' block; do
    block="${block#"${block%%[![:space:]]*}"}"
    [ -z "$block" ] && continue
    echo "==> $(basename "$guide"): $block"
    if ! bash -euo pipefail -c "$block" >"$work/out.txt" 2>&1; then
      cat "$work/out.txt"
      echo "docs-check: block in $(basename "$guide") failed" >&2
      exit 1
    fi
  done || status=1
done
exit $status
