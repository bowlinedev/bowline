#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
binary="${TMPDIR:-/tmp}/bowline-ledger-mock"

(cd "$here/../../../cmd/bowline" && go build -buildvcs=false -o "$binary" .)
cd "$here/.."
exec "$binary" mock --addr 127.0.0.1:8080 --seed 1 --no-playground
