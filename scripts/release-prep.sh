#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"
version="${1:?usage: release-prep.sh vX.Y.Z}"
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "version must look like vX.Y.Z" >&2; exit 2 ;;
esac
new="${version#v}"

missing=""
for tool in pnpm uv cargo go python3; do
  command -v "$tool" >/dev/null || missing="$missing $tool"
done
[ -z "$missing" ] || { echo "release-prep needs:$missing" >&2; exit 1; }

old="$(sed -n 's/^const Version = "\(.*\)"$/\1/p' version.go)"
[ -n "$old" ] || { echo "cannot read the current version from version.go" >&2; exit 1; }
if [ "$old" = "$new" ]; then
  echo "already at $new" >&2
  exit 1
fi

sub() {
  local file="$1" pattern="$2" replacement="$3"
  [ -f "$file" ] || return 0
  perl -0pi -e "s/$pattern/$replacement/g" "$file"
  echo "bumped $file"
}

sub version.go "const Version = \"$old\"" "const Version = \"$new\""
sub version_test.go "Version != \"$old\"" "Version != \"$new\""
sub README.md "Version $old\." "Version $new."

for pkg in packages/*/package.json; do
  sub "$pkg" "\"version\": \"$old\"" "\"version\": \"$new\""
done
for src in packages/client/src/index.ts packages/svelte/src/index.ts packages/agent/src/index.ts; do
  sub "$src" "export const version = \"$old\"" "export const version = \"$new\""
done
sub packages/client/src/index.test.ts "\"$old\"" "\"$new\""

for changelog in packages/*/CHANGELOG.md; do
  [ -f "$changelog" ] || continue
  grep -q "^## $new" "$changelog" && continue
  perl -0pi -e "s/^(# [^\n]+\n)/\$1\n## $new\n\n- Released with bowline $new; see the root CHANGELOG for what changed.\n/" "$changelog"
  echo "noted $new in $changelog"
done

scripts/bump-clients.sh "$version"

sub examples/ledger/bowline.json "\"version\": \"$old\"" "\"version\": \"$new\""
sub examples/ledger/api/routes.go "example:\"$old\"" "example:\"$new\""
sub examples/federation/billing/api/routes.go "example:\"$old\"" "example:\"$new\""
sub examples/ledger/web/e2e/mock.spec.ts "$old" "$new"
sub examples/ledger/elixir/mix.exs "\"$old\"" "\"$new\""
sub examples/ledger/python/pyproject.toml "\"$old\"" "\"$new\""
sub examples/ledger/rust/Cargo.toml "\"$old\"" "\"$new\""

if grep -q "^## Unreleased$" CHANGELOG.md; then
  perl -0pi -e "s/^## Unreleased\$/## $new/m" CHANGELOG.md
else
  perl -0pi -e "s/^(# [^\n]+\n)/\$1\n## $new\n/" CHANGELOG.md
fi
grep -q "^## $new\$" CHANGELOG.md || { echo "release-prep: could not open $new in CHANGELOG.md" >&2; exit 1; }
echo "opened $new in CHANGELOG.md"

run() {
  printf -- '-- %s\n' "$*"
  "$@" || { echo "release-prep: failed at: $*" >&2; exit 1; }
}

run pnpm install --lockfile-only
run sh -c 'cd examples/ledger/python && uv lock'
run sh -c 'cd examples/ledger/rust && cargo update -p bowline-client'
run sh -c 'cd packages/python/bowline-client && uv lock'
run sh -c 'cd packages/rust/bowline-client && cargo update -p bowline-client'
run sh -c 'cd python/bowline-agent && uv lock'
run sh -c 'cd examples/ledger && go run ../../cmd/bowline gen'
run sh -c 'cd examples/federation/billing && go run ../../../cmd/bowline gen'
run scripts/repin-gateway.sh examples/federation/gateway/bowline.gateway.json
run sh -c 'cd examples/federation/gateway && go run ../../../cmd/bowline gateway compose -o composed.contract.json'
run sh -c 'cd examples/federation/gateway && go run ../../../cmd/bowline gen --from composed.contract.json'
run scripts/eval-ledger.sh record
run sh -c 'cd examples/ledger && go run ../../cmd/bowline check'
run sh -c 'cd examples/federation/billing && go run ../../../cmd/bowline check'
run scripts/version-check.sh

cat <<EOF

prepared $new. next, by hand:
  scripts/verify.sh
  scripts/tag-modules.sh $version
EOF
