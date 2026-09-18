#!/usr/bin/env bash
set -euo pipefail

version="${1:?usage: tag-modules.sh vX.Y.Z [commit]}"
commit="${2:-HEAD}"
case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "version must look like vX.Y.Z" >&2; exit 2 ;;
esac

git tag -a -m "bowline $version" "$version" "$commit"
git tag -a -m "bowline cli $version" "cmd/bowline/$version" "$commit"
git tag -a -m "bowline websocket transport $version" "transport/websocket/$version" "$commit"
git tag -a -m "bowline mcp server $version" "mcp/$version" "$commit"
git tag -a -m "bowline agent sdk $version" "agent/$version" "$commit"
git tag -a -m "bowline playground $version" "playground/$version" "$commit"
git tag -a -m "bowline contracttest $version" "contracttest/$version" "$commit"
git tag -a -m "bowline registry $version" "registry/$version" "$commit"
git tag -a -m "bowline fiber adapter $version" "adapters/fiber/$version" "$commit"
git tag -a -m "bowline gateway $version" "gateway/$version" "$commit"
git tag -a -m "bowline opentelemetry $version" "otel/$version" "$commit"
echo "created $version and its module tags; push the version tag alone first, because GitHub emits no push event for more than three tags at once:"
echo "  git push origin $version"
echo "  git push origin cmd/bowline/$version transport/websocket/$version mcp/$version agent/$version playground/$version contracttest/$version adapters/fiber/$version gateway/$version registry/$version otel/$version"
