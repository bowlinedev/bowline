#!/usr/bin/env bash
set -uo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

want="$(sed -n 's/^const Version = "\(.*\)"$/\1/p' version.go)"
[ -n "$want" ] || { echo "version-check: cannot read the version from version.go" >&2; exit 1; }

status=0
bad() { echo "version-check: $*" >&2; status=1; }

check() {
  local file="$1" expr="$2" got
  [ -f "$file" ] || { bad "$file is missing"; return; }
  got="$(sed -n "$expr" "$file" | head -1)"
  [ -n "$got" ] || { bad "$file has no version to read"; return; }
  [ "$got" = "$want" ] || bad "$file says $got, want $want"
}

noted() {
  local file="$1"
  [ -f "$file" ] || { bad "$file is missing"; return; }
  grep -q "^## $want\$" "$file" || bad "$file has no '## $want' entry"
}

check version_test.go                              's/.*Version != "\([^"]*\)".*/\1/p'
check README.md                                    's/^Version \([0-9][^.]*\.[^.]*\.[0-9A-Za-z.-]*\)\..*/\1/p'

for pkg in packages/*/package.json; do
  check "$pkg"                                     's/^  "version": "\([^"]*\)",$/\1/p'
done
for src in packages/agent/src/index.ts packages/client/src/index.ts packages/svelte/src/index.ts; do
  check "$src"                                     's/^export const version = "\([^"]*\)";$/\1/p'
done
check packages/client/src/index.test.ts            's/.*expect(version).toBe("\([^"]*\)").*/\1/p'

check packages/dart/bowline/pubspec.yaml           's/^version: \(.*\)$/\1/p'
check packages/python/bowline-client/pyproject.toml 's/^version = "\([^"]*\)"$/\1/p'
check packages/rust/bowline-client/Cargo.toml      's/^version = "\([^"]*\)"$/\1/p'
check packages/elixir/bowline_client/mix.exs       's/.*version: "\([^"]*\)".*/\1/p'
check python/bowline-agent/pyproject.toml          's/^version = "\([^"]*\)"$/\1/p'

check examples/ledger/bowline.json                 's/.*"version": "\([^"]*\)".*/\1/p'
check examples/ledger/api/openapi.json             's/.*"version": "\([^"]*\)".*/\1/p'
check examples/ledger/api/routes.go                's/.*Version string.*example:"\([^"]*\)".*/\1/p'
check examples/federation/billing/api/routes.go    's/.*Version string.*example:"\([^"]*\)".*/\1/p'
check examples/ledger/web/e2e/mock.spec.ts         's/.*bowline \([0-9][^"]*\)".*/\1/p'
check examples/ledger/elixir/mix.exs               's/.*version: "\([^"]*\)".*/\1/p'
check examples/ledger/python/pyproject.toml        's/^version = "\([^"]*\)"$/\1/p'
check examples/ledger/rust/Cargo.toml              's/^version = "\([^"]*\)"$/\1/p'

noted CHANGELOG.md
for changelog in packages/*/CHANGELOG.md packages/dart/bowline/CHANGELOG.md packages/python/bowline-client/CHANGELOG.md packages/rust/bowline-client/CHANGELOG.md packages/elixir/bowline_client/CHANGELOG.md; do
  [ -f "$changelog" ] || continue
  noted "$changelog"
done

if [ "$status" -eq 0 ]; then
  echo "version-check: every version site agrees on $want"
fi
exit "$status"
