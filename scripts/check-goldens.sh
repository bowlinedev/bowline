#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
target="${1:?usage: check-goldens.sh ts|go|dart|python|rust|elixir}"
testdata="$repo/cmd/bowline/internal/gen/$target/testdata"
[ -d "$testdata" ] || { echo "check-goldens: no testdata for $target" >&2; exit 2; }

case "$target" in
  ts)
    (cd "$repo" && pnpm --filter @bowline/client exec tsc -p tsconfig.golden.json)
    ;;
  go|goclient)
    (cd "$repo/cmd/bowline" && go test ./internal/gen/goclient -run TestGoldens)
    ;;
  dart)
    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT
    cp -R "$repo/packages/dart/bowline" "$work/bowline"
    mkdir -p "$work/goldens/lib"
    cat > "$work/goldens/pubspec.yaml" <<PUB
name: goldens
environment:
  sdk: ^3.5.0
dependencies:
  bowline:
    path: ../bowline
PUB
    for f in "$testdata"/*.golden.dart; do
      cp "$f" "$work/goldens/lib/$(basename "${f%.golden.dart}").dart"
    done
    (cd "$work/goldens" && dart pub get >/dev/null && dart analyze --fatal-infos lib)
    ;;
  python)
    project="$repo/packages/python/bowline-client"
    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT
    for f in "$testdata"/*.golden.py; do
      cp "$f" "$work/golden_$(basename "${f%.golden.py}").py"
    done
    (cd "$project" && uv run --frozen python -m mypy --strict --explicit-package-bases "$work"/*.py)
    (cd "$project" && uv run --frozen python -m pytest -q -p no:cacheprovider "$testdata/test_goldens.py")
    ;;
  rust)
    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT
    mkdir -p "$work/goldens/src"
    cat > "$work/goldens/Cargo.toml" <<TOML
[package]
name = "goldens"
version = "0.0.0"
edition = "2021"

[dependencies]
bowline-client = { path = "$repo/packages/rust/bowline-client" }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
chrono = { version = "0.4", features = ["serde"] }

[lib]
path = "src/lib.rs"
TOML
    : > "$work/goldens/src/lib.rs"
    for f in "$testdata"/*.golden.rs; do
      name="$(basename "${f%.golden.rs}" | tr '-' '_')"
      cp "$f" "$work/goldens/src/$name.rs"
      echo "pub mod $name;" >> "$work/goldens/src/lib.rs"
    done
    (cd "$work/goldens" && cargo check --quiet && cargo clippy --quiet -- -D warnings)
    ;;
  elixir)
    work="$(mktemp -d)"
    trap 'rm -rf "$work"' EXIT
    mkdir -p "$work/goldens/lib"
    cat > "$work/goldens/mix.exs" <<MIX
defmodule Goldens.MixProject do
  use Mix.Project

  def project do
    [app: :goldens, version: "0.0.0", elixir: "~> 1.18", deps: deps()]
  end

  defp deps do
    [{:bowline_client, path: "$repo/packages/elixir/bowline_client"}]
  end
end
MIX
    for f in "$testdata"/*.golden.ex; do
      cp "$f" "$work/goldens/lib/$(basename "${f%.golden.ex}").ex"
    done
    (cd "$work/goldens" && mix deps.get >/dev/null && mix compile --warnings-as-errors)
    ;;
  *)
    echo "check-goldens: unknown target $target" >&2
    exit 2
    ;;
esac
