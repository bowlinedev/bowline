#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")/../../.." && pwd)"
work="${BOWLINE_REGISTRY_UI_E2E_DIR:-$(mktemp -d)}"
listen="${BOWLINE_REGISTRY_UI_LISTEN:-127.0.0.1:8097}"
token="${BOWLINE_REGISTRY_TOKEN:-registry-ui-e2e-token}"

(cd "$here/cmd/bowline" && go build -buildvcs=false -o "$work/bowline" .)

BOWLINE_REGISTRY_LISTEN="127.0.0.1:8098" BOWLINE_REGISTRY_TOKEN="$token" \
  "$here/examples/federation/registry-seed.sh" "$work/store"

exec "$work/bowline" registry serve --store "$work/store" --listen "$listen" --token "$token"
