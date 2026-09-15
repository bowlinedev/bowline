#!/usr/bin/env bash
set -uo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

report="$(mktemp)"
filter="$(mktemp)"
trap 'rm -f "$report" "$filter"' EXIT

pnpm audit --prod --json >"$report" 2>/dev/null

cat >"$filter" <<'PY'
import json, sys

with open(sys.argv[1]) as handle:
    data = json.load(handle)

published = []
for advisory in (data.get("advisories") or {}).values():
    paths = [p for finding in advisory.get("findings", []) for p in finding.get("paths", [])]
    reachable = [p for p in paths if p.startswith("packages__")]
    if reachable:
        published.append((advisory.get("severity"), advisory.get("module_name"), reachable))

if published:
    for severity, module, paths in sorted(published):
        print(f"{severity}\t{module}\t{', '.join(paths)}")
    print(f"{len(published)} advisory path(s) reach a published package", file=sys.stderr)
    sys.exit(1)

print(f"no advisory reaches a published package ({len(data.get('advisories') or {})} reported against the example apps)")
PY

python3 "$filter" "$report"
