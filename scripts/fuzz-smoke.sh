#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"
duration="${1:-10s}"
export GOWORK=off

run() {
  local module="$1" package="$2" target="$3"
  printf -- '-- %s %s\n' "$target" "$package"
  (cd "$module" && go test -run '^$' -fuzz "^$target\$" -fuzztime "$duration" "$package")
}

run . . FuzzHandler
run . ./contract FuzzContractParse
run . ./contract FuzzContractHash
run . ./internal/codec FuzzDecode
run . ./internal/codec FuzzNormalizeEquivalence
run . ./internal/validate FuzzValidateParseTag
run . ./internal/validate FuzzValidateCheck
run . ./signing FuzzVerify
run . ./signing FuzzReplayCache
run cmd/bowline ./internal/gen/ts FuzzTSGenerator
run cmd/bowline ./internal/analyzer FuzzAnalyzerMutatesFidelityRows

echo "fuzz smoke: every target survived $duration"
