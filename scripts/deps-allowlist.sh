#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
allowlist="$repo/scripts/deps-allowlist.txt"

check_module() {
  local dir="$1" allowed="$2"
  python3 - "$dir" "$allowed" <<'PY'
import json, subprocess, sys

directory, allowed = sys.argv[1], sys.argv[2].split()
raw = subprocess.run(["go", "mod", "edit", "-json"], cwd=directory, capture_output=True, text=True, check=True).stdout
mod = json.loads(raw)
own = "github.com/bowlinedev/bowline"
bad = []
for req in mod.get("Require") or []:
    if req.get("Indirect"):
        continue
    path = req["Path"]
    if path == own or path.startswith(own + "/"):
        continue
    if path in allowed:
        continue
    bad.append(path)
if bad:
    print(f"{directory}: not on the allowlist: {', '.join(sorted(bad))}", file=sys.stderr)
    sys.exit(1)
PY
}

run() {
  local status=0 line module allowed
  while IFS= read -r line; do
    case "$line" in ''|'#'*) continue ;; esac
    module="${line%%=*}"
    allowed="${line#*=}"
    module="$(printf '%s' "$module" | tr -d '[:space:]')"
    [ -f "$repo/$module/go.mod" ] || { echo "$module has no go.mod" >&2; status=1; continue; }
    check_module "$repo/$module" "$allowed" || status=1
  done < "$allowlist"
  return "$status"
}

self_test() {
  local tmp
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  cp "$repo/go.mod" "$tmp/go.mod"
  (cd "$tmp" && go mod edit -require=example.com/forbidden@v1.0.0)
  if check_module "$tmp" "" 2>/dev/null; then
    echo "self-test: an unlisted dependency was accepted" >&2
    return 1
  fi
  echo "self-test: an unlisted dependency is rejected"
}

case "${1:-check}" in
  check) run ;;
  self-test) self_test ;;
  *) echo "usage: deps-allowlist.sh [check|self-test]" >&2; exit 2 ;;
esac
