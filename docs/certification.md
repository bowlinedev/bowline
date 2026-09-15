# Target certification

A client target is official when every check below is green in this repository's CI with a pinned toolchain. Community targets reach the same list by the same checks.

| Check | What it proves |
|---|---|
| Fidelity goldens | Every `expected.contract.json` under `cmd/bowline/internal/analyzer/testdata/fidelity/rows` produces a golden file for the target, reviewed against `spec/mapping-table.md`. |
| Goldens compile | The language's own toolchain type-checks or compiles every golden: `tsc` for TypeScript, `go vet` for Go, `dart analyze --fatal-infos`, `mypy --strict` and `pytest`, `cargo clippy -- -D warnings` and `cargo test`. |
| Reject rule | A contract construct the target cannot represent fails `bowline gen` with a diagnostic naming the type and the target; it never degrades to a dynamic type. |
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

Client packages share the CLI's minor version and are bumped together by `scripts/bump-clients.sh`; the `release-clients` workflow publishes each on demand once the registry accounts exist.
