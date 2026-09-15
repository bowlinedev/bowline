#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
binary="/tmp/bowline-playground-e2e"

(cd "$here" && go build -buildvcs=false -o "$binary" ../../cmd/bowline)
cd "$here/../../examples/ledger"
exec "$binary" mock --addr 127.0.0.1:18090
