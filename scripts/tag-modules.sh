#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: tag-modules.sh vX.Y.Z}"
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "version must look like vX.Y.Z" >&2; exit 2 ;;
esac

git tag -a -m "bowline $version" "$version"
git tag -a -m "bowline cli $version" "cmd/bowline/$version"
git tag -a -m "bowline websocket transport $version" "transport/websocket/$version"
git tag -a -m "bowline mcp server $version" "mcp/$version"
git tag -a -m "bowline agent sdk $version" "agent/$version"
git tag -a -m "bowline playground $version" "playground/$version"
git tag -a -m "bowline contracttest $version" "contracttest/$version"
echo "created tags $version, cmd/bowline/$version, transport/websocket/$version, mcp/$version, agent/$version, playground/$version, and contracttest/$version; push with: git push origin $version cmd/bowline/$version transport/websocket/$version mcp/$version agent/$version playground/$version contracttest/$version"
