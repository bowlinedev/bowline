# Target certification

A client target counts as official once all of the checks below pass in this repository's CI with a pinned toolchain. Community targets can be added to the same list by passing the same checks.

You can run the checks yourself with `bowline certify`:

```bash
bowline certify --target kotlin --generator bowline-gen-kotlin --config certify.json
```

This runs every accepted fidelity row through the generator using the protocol described in `docs/plugins.md`. It then scans the output for escape-hatch types, runs the generator a second time to check that the output is deterministic, and runs whatever compile and test commands are listed in `certify.json`. Generators that pass are listed in `docs/certified.md`. Pass `--report` to write the row for you.

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
| `ts` | `cmd/bowline/internal/gen/ts` | `@bowlinedev/client` | `node` in `ci.yml` | `ledger` in `e2e.yml` | `docs/quickstart.md` |
| `go` | `cmd/bowline/internal/gen/goclient` | the root module | `go` in `ci.yml` | `go-client example` in `ci.yml` | `docs/guides/frameworks/go-client.md` |
| `dart` | `cmd/bowline/internal/gen/dart` | `packages/dart/bowline` | `goldens-dart` | `dart` in `e2e.yml` | `docs/guides/dart.md` |
| `python` | `cmd/bowline/internal/gen/python` | `packages/python/bowline-client` | `goldens-python` | `python` in `e2e.yml` | `docs/guides/python.md` |
| `rust` | `cmd/bowline/internal/gen/rust` | `packages/rust/bowline-client` | `goldens-rust` | `rust` in `e2e.yml` | `docs/guides/rust.md` |
| `elixir` | `cmd/bowline/internal/gen/elixir` | `packages/elixir/bowline_client` | `goldens-elixir` | `elixir` in `e2e.yml` | `docs/guides/elixir.md` |

## certify.json

The command looks for a `certify.json` file next to the module it is run from. The keys are:

| Key | Meaning |
|---|---|
| `target` | the target name, also the default for the escape-hatch list and file extension |
| `generator` | the command to run, or a built-in target name |
| `repository` | where the generator lives, for the certified list |
| `version` | the generator version being certified |
| `extension` | the extension for the file handed to the generator as `out` |
| `escapeHatch` | tokens that must not appear beyond the generator's own runtime code |
| `compile` | the command that compiles the generated output |
| `test` | the command that runs the target's runtime test suite |
| `conformance` | the client entry point a conformance run drives, recorded for the certified list |

The `compile` and `test` commands run in the module directory. The environment variable `BOWLINE_CERTIFY_DIR` points at a directory with one subdirectory of generated output per fidelity row. If either `compile` or `test` is missing, the target is not certified. The run reports the missing step as skipped and exits with status 1.

Certification checks the generator: every fidelity row renders, the output compiles with the real toolchain, no escape-hatch types are used, and the output is identical across two runs. It also checks that the target's runtime package still passes its own tests. It does not run a generated client against a live server. That is covered separately by the per-language end-to-end jobs, which run each generated ledger client against the real ledger in CI.

When counting escape-hatch tokens, the tool first generates an empty contract and uses that as a baseline. This is subtracted from the real count, so a language whose runtime code needs to mention the token (for example Go's `any` in a type parameter, or Elixir's `term()` in a decoder spec) is not penalised for it. Rows whose contract uses the `raw` primitive are exempt from the check. This is how Rust is allowed to use `serde_json::Value` in exactly the places where `raw` appears.

The built-in targets are configured under `cmd/bowline/certify/`. Run `scripts/certify-builtins.sh` to certify them all and regenerate `docs/certified.md`.

The client packages share the CLI's minor version number and are bumped together by `scripts/bump-clients.sh`. The `release-clients` workflow publishes each package on demand.
