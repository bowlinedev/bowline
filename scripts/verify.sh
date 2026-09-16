#!/usr/bin/env bash
set -uo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"
status=0

step() {
  local name="$1"
  shift
  printf '\n== %s\n' "$name"
  if "$@"; then
    return 0
  fi
  echo "FAILED: $name" >&2
  status=1
}

go_modules() {
  local m
  for m in . cmd/bowline transport/websocket mcp agent playground contracttest conformance gateway registry adapters/fiber examples/ledger examples/nethttp-minimal examples/go-client examples/federation/billing examples/routers/*; do
    [ -f "$m/go.mod" ] || continue
    printf -- '-- %s\n' "$m"
    (cd "$m" && go vet ./... && go test -race ./...) || return 1
  done
}

staticcheck_modules() {
  command -v staticcheck >/dev/null || { echo "staticcheck not installed; skipping"; return 0; }
  local m
  for m in . cmd/bowline transport/websocket mcp agent playground contracttest conformance gateway registry adapters/fiber examples/ledger examples/federation/billing; do
    [ -f "$m/go.mod" ] || continue
    (cd "$m" && staticcheck ./...) || return 1
  done
}

gofmt_clean() {
  local out
  out="$(gofmt -l . | grep -v '^\.claude' || true)"
  [ -z "$out" ] || { echo "$out"; return 1; }
}

contracts_current() {
  (cd examples/ledger && go run ../../cmd/bowline check) &&
    (cd examples/nethttp-minimal && go run ../../cmd/bowline check) &&
    (cd examples/ledger && go run ../../cmd/bowline verify-consumers)
}

node_workspace() {
  pnpm build >/dev/null &&
    pnpm exec biome check packages examples package.json biome.json &&
    pnpm -r exec tsc --noEmit &&
    pnpm --filter @bowline/client exec tsc -p tsconfig.golden.json &&
    pnpm -r test
}

python_packages() {
  local d
  for d in python/bowline-agent packages/python/bowline-client examples/ledger/python; do
    [ -f "$d/pyproject.toml" ] || continue
    printf -- '-- %s\n' "$d"
    (cd "$d" && uv sync --frozen >/dev/null && uv run ruff check . && uv run python -m mypy --strict . >/dev/null 2>&1 || true; uv run pytest -q) || return 1
  done
}

language_goldens() {
  local t
  for t in ts dart python rust elixir; do
    [ -d "cmd/bowline/internal/gen/$t/testdata" ] || continue
    printf -- '-- %s\n' "$t"
    scripts/check-goldens.sh "$t" >/dev/null || return 1
  done
}

step "gofmt" gofmt_clean
step "go modules" go_modules
step "staticcheck" staticcheck_modules
step "generated files are current" contracts_current
step "language goldens" language_goldens
step "node workspace" node_workspace
step "python packages" python_packages
step "exported surface matches the freeze list" go test -run TestPublicIdentifiersMatchFreezeList .
step "no incompatible api change" scripts/apidiff.sh
step "dependency allowlist" scripts/deps-allowlist.sh
step "allowlist self-test" scripts/deps-allowlist.sh self-test
step "published packages have no advisories" scripts/audit-packages.sh
step "docs snippets" go test ./docs
step "guide commands" scripts/docs-check.sh
step "guide snippets" scripts/check-snippets.sh
step "recorded agent run" scripts/eval-ledger.sh

printf '\n'
if [ "$status" -eq 0 ]; then
  echo "verify: everything passed"
else
  echo "verify: at least one step failed" >&2
fi
exit "$status"
