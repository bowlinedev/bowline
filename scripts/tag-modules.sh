#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: tag-modules.sh vX.Y.Z}"
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "version must look like vX.Y.Z" >&2; exit 2 ;;
esac

git tag -a -m "bowline $version" "$version"
git tag -a -m "bowline cli $version" "cmd/bowline/$version"
echo "created tags $version and cmd/bowline/$version; push with: git push origin $version cmd/bowline/$version"
