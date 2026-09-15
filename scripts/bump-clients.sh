#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
version="${1:?usage: bump-clients.sh vX.Y.Z}"
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "version must look like vX.Y.Z" >&2; exit 2 ;;
esac
bare="${version#v}"

bump() {
  local file="$1" pattern="$2" replacement="$3"
  [ -f "$file" ] || return 0
  perl -0pi -e "s/$pattern/$replacement/m" "$file"
  echo "bumped $file"
}

bump "$repo/packages/dart/bowline/pubspec.yaml" '^version: .*$' "version: $bare"
bump "$repo/packages/python/bowline-client/pyproject.toml" '^version = ".*"$' "version = \"$bare\""
bump "$repo/packages/rust/bowline-client/Cargo.toml" '^version = ".*"$' "version = \"$bare\""
bump "$repo/packages/elixir/bowline_client/mix.exs" 'version: ".*"' "version: \"$bare\""
bump "$repo/python/bowline-agent/pyproject.toml" '^version = ".*"$' "version = \"$bare\""

for changelog in "$repo/packages/dart/bowline/CHANGELOG.md" "$repo/packages/python/bowline-client/CHANGELOG.md" "$repo/packages/rust/bowline-client/CHANGELOG.md" "$repo/packages/elixir/bowline_client/CHANGELOG.md"; do
  [ -f "$changelog" ] || continue
  if ! grep -q "^## $bare" "$changelog"; then
    perl -0pi -e "s/^(# [^\n]+\n)/\$1\n## $bare\n\n- Released with bowline $bare; see the root CHANGELOG for what changed.\n/" "$changelog"
    echo "noted $bare in $changelog"
  fi
done
