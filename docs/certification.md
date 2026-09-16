# Target certification

A client target is official when every check below is green in this repository's CI with a pinned toolchain. Community targets reach the same list by the same checks.

Run them yourself with `bowline certify`:

```bash
bowline certify --target kotlin --generator bowline-gen-kotlin --config certify.json
```

It pipes every accepted fidelity row through the generator over the protocol in `docs/plugins.md`, scans for escape-hatch types, runs the generator twice to check determinism, and runs the compile and test commands your `certify.json` declares. `docs/certified.md` lists every generator that has passed; `--report` writes the row.

| Check | What it proves |
|---|---|
| Fidelity goldens | Every `expected.contract.json` under `cmd/bowline/internal/analyzer/testdata/fidelity/rows` produces a golden file for the target, reviewed against `spec/mapping-table.md`. |
| Goldens compile | The language's own toolchain type-checks or compiles every golden: `tsc` for TypeScript, `go vet` for Go, `dart analyze --fatal-infos`, `mypy --strict` and `pytest`, `cargo clippy -- -D warnings` and `cargo test`. |
| Reject rule | A contract construct the target cannot represent fails `bowline gen` with a diagnostic naming the type and the target; it never degrades to a dynamic type. The shared check is `cmd/bowline/internal/gen/support`, and every generator runs it before emitting. |
| Runtime package | The target's runtime package has tests for transport encoding, the error envelope, 64-bit string integers, base64 bytes, and RFC 3339 timestamps, and its release workflow dry-runs cleanly. |
| Drift gate | The ledger example commits the generated client and `bowline check` fails when it drifts. |
| End to end | An example in the language lists, creates with a validation failure, and voids ledger invoices against the real server in CI. |
| Guide | A guide under `docs/guides/` whose snippets come from files CI runs. |

## Official targets

| Target | Generator | Runtime | Goldens job | End-to-end job | Guide |
|---|---|---|---|---|---|
| `ts` | `cmd/bowline/internal/gen/ts` | `@bowline/client` | `node` in `ci.yml` | `ledger` in `e2e.yml` | `docs/quickstart.md` |
| `go` | `cmd/bowline/internal/gen/goclient` | the root module | `go` in `ci.yml` | `go-client example` in `ci.yml` | `docs/guides/frameworks/go-client.md` |
| `dart` | `cmd/bowline/internal/gen/dart` | `packages/dart/bowline` | `goldens-dart` | `dart` in `e2e.yml` | `docs/guides/dart.md` |
| `python` | `cmd/bowline/internal/gen/python` | `packages/python/bowline-client` | `goldens-python` | `python` in `e2e.yml` | `docs/guides/python.md` |
| `rust` | `cmd/bowline/internal/gen/rust` | `packages/rust/bowline-client` | `goldens-rust` | `rust` in `e2e.yml` | `docs/guides/rust.md` |
| `elixir` | `cmd/bowline/internal/gen/elixir` | `packages/elixir/bowline_client` | `goldens-elixir` | `elixir` in `e2e.yml` | `docs/guides/elixir.md` |

## certify.json

The command reads a `certify.json` beside the module it is run in:

| Key | Meaning |
|---|---|
| `target` | the target name, also the default for the escape-hatch list and file extension |
| `generator` | the command to run, or a built-in target name |
| `repository` | where the generator lives, for the certified list |
| `version` | the generator version being certified |
| `extension` | the extension for the file handed to the generator as `out` |
| `escapeHatch` | tokens that must not appear beyond the generator's own runtime code |
| `compile` | the command that compiles the generated output |
| `test` | the command that runs the conformance suite |
| `conformance` | the client entry point the conformance run drives |

`compile` and `test` run in the module directory with `BOWLINE_CERTIFY_DIR` pointing at a directory holding one subdirectory of generated output per fidelity row. A target with no `compile` or no `test` is not certified: the run reports them as skipped and exits 1.

Escape-hatch counting subtracts a baseline measured by generating an empty contract, so a language whose runtime code legitimately mentions the token — Go's `any` in a type parameter, Elixir's `term()` in a decoder spec — is not penalised for it. A row whose contract uses the `raw` primitive is exempt, which is how Rust's `serde_json::Value` is allowed exactly where `raw` appears.

The six built-in targets are configured under `cmd/bowline/certify/` and certified by `scripts/certify-builtins.sh`, which regenerates `docs/certified.md`.

Client packages share the CLI's minor version and are bumped together by `scripts/bump-clients.sh`; the `release-clients` workflow publishes each on demand once the registry accounts exist.
