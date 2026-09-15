#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../.." && pwd)"
store="$repo/examples/federation/registry-data"
listen="${BOWLINE_REGISTRY_LISTEN:-127.0.0.1:8096}"
registry="http://$listen"
export BOWLINE_REGISTRY_TOKEN="${BOWLINE_REGISTRY_TOKEN:-federation-demo-token}"
work="$(mktemp -d)"

cleanup() {
  if [ -n "${server:-}" ]; then kill "$server" 2>/dev/null || true; fi
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

(cd "$repo/cmd/bowline" && go build -buildvcs=false -o "$work/bowline" .)
"$work/bowline" registry serve --store "$store" --listen "$listen" --token "$BOWLINE_REGISTRY_TOKEN" >"$work/registry.log" 2>&1 &
server=$!
for _ in $(seq 1 100); do
  if curl -fsS "$registry/v1/healthz" >/dev/null 2>&1; then break; fi
  sleep 0.2
done
curl -fsS "$registry/v1/healthz" >/dev/null

cp -R "$repo/examples/ledger" "$work/ledger"
rm -rf "$work/ledger/web" "$work/ledger/dart" "$work/ledger/rust" "$work/ledger/elixir" "$work/ledger/python" "$work/ledger/tmp"
python3 - "$work/ledger/go.mod" "$repo" <<'PY'
import sys
path, repo = sys.argv[1], sys.argv[2]
s = open(path).read()
s = s.replace("=> ../..", "=> " + repo).replace("=> ../../", "=> " + repo + "/")
open(path, "w").write(s)
PY

cd "$work/ledger"
if ! "$work/bowline" check --registry "$registry" --service ledger; then
  echo "impact-check: the unchanged contract should not break any consumer" >&2
  exit 1
fi

python3 - ledger/types.go <<'PY'
import sys
path = sys.argv[1]
s = open(path).read()
before = s
s = s.replace('`json:"total" example:"USD 1500.00"`', '`json:"amount" example:"USD 1500.00"`')
if s == before:
    raise SystemExit("impact-check: the total field was not found to rename")
open(path, "w").write(s)
PY

report="$work/report.txt"
if "$work/bowline" check --registry "$registry" --service ledger >"$report" 2>&1; then
  cat "$report"
  echo "impact-check: removing a field the consumer reads should have failed" >&2
  exit 1
fi
cat "$report"
grep -q "breaks    ledger-web" "$report" || { echo "impact-check: the affected consumer was not named" >&2; exit 1; }
grep -q "reads total" "$report" || { echo "impact-check: the reason was not reported" >&2; exit 1; }
echo "impact-check: the registry named the consumer that reads the removed field"
