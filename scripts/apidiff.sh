#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

packages=(. ./contract ./signing)
strict=0
baseline=""
for arg in "$@"; do
  case "$arg" in
    --strict) strict=1 ;;
    *) baseline="$arg" ;;
  esac
done

if [ -z "$baseline" ]; then
  baseline="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1)"
fi

if [ -z "$baseline" ] || ! git rev-parse -q --verify "refs/tags/$baseline" >/dev/null; then
  echo "apidiff: no release tag to compare against; nothing to check"
  exit 0
fi

if ! command -v apidiff >/dev/null; then
  GOFLAGS=-mod=mod go install golang.org/x/exp/cmd/apidiff@latest
fi
apidiff="$(command -v apidiff || echo "$(go env GOPATH)/bin/apidiff")"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

git worktree add -q --detach "$work/old" "$baseline"
trap 'git worktree remove -f "$work/old" >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

status=0
for pkg in "${packages[@]}"; do
  name="$(printf '%s' "$pkg" | tr './' '_')"
  (cd "$work/old" && GOWORK=off "$apidiff" -w "$work/old$name.api" "$pkg")
  printf -- '-- %s against %s\n' "$pkg" "$baseline"
  out="$(GOWORK=off "$apidiff" -incompatible "$work/old$name.api" "$pkg")"
  out="$(printf '%s\n' "$out" | grep -v '^- Version: value changed from ' || true)"
  if [ -n "$out" ]; then
    echo "$out"
    status=1
  fi
done

if [ "$status" -eq 0 ]; then
  echo "apidiff: no incompatible change against $baseline"
  exit 0
fi

case "$baseline" in
  v0.*) armed=0 ;;
  *) armed=1 ;;
esac
if [ "$strict" -eq 1 ]; then
  armed=1
fi

if [ "$armed" -eq 0 ]; then
  echo "apidiff: incompatible against $baseline, allowed because the stability guarantee takes effect at 1.0; see docs/stability.md"
  exit 0
fi
echo "apidiff: incompatible change against $baseline; see docs/stability.md" >&2
exit 1
