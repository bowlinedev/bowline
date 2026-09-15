# CLI reference

Every command and flag on this page is covered by the stability guarantee in `docs/stability.md`: within 1.x nothing here is removed or changes meaning, and new flags are additive with a default that preserves existing behaviour.

A test in `cmd/bowline` fails if this page names a command or flag the binary's own usage text does not, so the two cannot drift apart.

## Exit codes

Every command uses the same three:

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | the command ran and the answer is no: drift, a breaking change, a failed replay, an unreachable upstream |
| 2 | the command line is wrong: an unknown flag, a missing argument, an unknown command |

A `1` means the tool worked and you have a problem. A `2` means the tool did not get far enough to have an opinion.

## Configuration

Every command that analyzes a module reads `bowline.json` from the working directory:

| Key | Meaning |
|---|---|
| `entry` | the router expression to evaluate, for example `./api.Routes` |
| `contract` | where to write the contract document; defaults to `bowline.contract.json` |
| `schemas` | embed a JSON Schema per procedure in the contract |
| `targets` | a map of target name to `{ "out": path }`, plus the per-target keys below |
| `openapi` | `{ "out", "title", "version", "serverUrl" }` to emit an OpenAPI 3.1 document |
| `dev` | `{ "app": command }` for the process `bowline dev` restarts |

Target keys: `out` is required; `zod` emits companion Zod schemas on the `ts` target; `format` picks the encoding for the `tools` target; `package` sets the package or module name on targets that have one; `command` runs an external generator instead of a built-in one, as described in `docs/plugins.md`.

## Commands

### `bowline gen`

Analyzes the module and writes the contract and every configured target.

| Flag | Meaning |
|---|---|
| `--from <contract.json>` | render the targets from an existing document instead of analyzing the module; the source document is never rewritten |

Prints one line per file, `wrote` or `unchanged`. Exits 1 on an analyzer diagnostic, having written nothing.

### `bowline check`

Verifies that what is committed matches what the module would generate now.

| Flag | Meaning |
|---|---|
| `--against <ref>` | diff the committed contract against that git ref instead of checking for drift; exits 1 on a breaking change |
| `--allow-breaking` | with `--against`, report breaking changes without failing |
| `--consumers <dir>` | annotate the report with recorded consumer contracts; defaults to `contracts/consumers` when it exists |
| `--registry <url>` | ask a registry which consumers the candidate contract would break |
| `--service <name>` | with `--registry`, the service to compare against |
| `--strict` | with `--registry`, fail when a consumer's usage cannot be attributed |
| `--token <t>` | with `--registry`, the bearer token; defaults to `BOWLINE_REGISTRY_TOKEN` |
| `--json` | write the machine-readable report described below |

Without `--json`, drift is reported as `ok`, `outdated`, or `missing` per file.

#### `check --json`

One JSON object on stdout. `mode` tells you which of the three checks ran:

```json
{
  "mode": "drift",
  "ok": false,
  "files": [
    { "path": "api/bowline.contract.json", "status": "ok" },
    { "path": "web/src/bowline.ts", "status": "outdated" }
  ],
  "breaking": 0
}
```

`status` is `ok`, `outdated`, or `missing`. With `--against` the mode is `against` and the object carries `ref`, `changes` (the same array `diff --json` produces), and `breaking`, the count of changes in the `breaking` category. With `--registry` the mode is `registry` and the object carries `impact`, the registry's report with its `baseline`, `changes`, `affected`, and `unattributed` arrays.

`ok` is the answer, and it always agrees with the exit code: `true` with exit 0, `false` with exit 1.

### `bowline dev`

Regenerates on every save and restarts the configured app.

| Flag | Meaning |
|---|---|
| `--playground <addr>` | also serve the playground against the live contract |

Runs until interrupted.

### `bowline diff`

Lists the semantic changes between two contract documents on disk.

    bowline diff old.json new.json

| Flag | Meaning |
|---|---|
| `--format text\|markdown\|json` | output encoding; defaults to `text` |
| `--json` | shorthand for `--format json` |
| `--consumers <dir>` | name the recorded consumers each breaking change affects |

Exits 1 when any change is breaking, which is what makes it usable as a gate.

#### `diff --json`

A JSON array of changes on stdout:

```json
[
  {
    "path": "procedure invoices.create output field total",
    "category": "breaking",
    "message": "field removed"
  }
]
```

`category` is one of `added`, `widened`, `narrowed`, `removed`, or `breaking`. The array is empty, not null, when nothing changed.

### `bowline export openapi`

Writes an OpenAPI 3.1 document derived from the contract.

| Flag | Meaning |
|---|---|
| `-o <path>` | where to write it; defaults to the `openapi` block in `bowline.json` |

### `bowline export tools`

Writes LLM tool definitions for every procedure marked as a tool.

| Flag | Meaning |
|---|---|
| `--format anthropic\|openai\|json-schema` | encoding; defaults to `json-schema` |
| `--scope <s>` | only tools carrying this scope |
| `--read-only` | only tools marked read-only |
| `--out <path>` | where to write; stdout when absent |

### `bowline mcp`

Serves the exposed procedures to MCP clients, proxying to a running handler.

| Flag | Meaning |
|---|---|
| `--url <base>` | base URL of the running handler; required |
| `--stdio` | serve on stdin and stdout; the default |
| `--listen <addr>` | serve streamable HTTP on this address instead |
| `--scope <s>` | expose only tools with this scope; repeatable |
| `--read-only` | expose only tools with the read-only hint |
| `--rate <n>` | calls per minute per client; 0 disables the limit |
| `--burst <n>` | burst size for `--rate`; defaults to `--rate` |
| `--header "K: v"` | static header sent upstream; repeatable |

### `bowline mock`

Serves generated or recorded responses from the contract alone, with no server.

| Flag | Meaning |
|---|---|
| `--addr <addr>` | address to listen on; defaults to `:8090` |
| `--seed <n>` | seed for generated data; the same seed gives the same values |
| `--record <url>` | proxy every request to that upstream and record fixtures |
| `--replay` | serve recorded fixtures before generating |
| `--strict` | with `--replay`, answer `UNIMPLEMENTED` on a miss instead of generating |
| `--fixtures <dir>` | fixture directory; defaults to `mocks` |
| `--no-playground` | do not serve the playground at `/_playground/` |

### `bowline eval record`

Runs scripted tool calls and writes a recording.

| Flag | Meaning |
|---|---|
| `--script <path>` | the script of calls to run |
| `--out <path>` | the recording to write |
| `--backend mock\|replay\|url` | where calls go; defaults to the in-process mock |
| `--url <base>` | the handler for `--backend url` |
| `--fixtures <dir>` | fixture directory for `--backend replay` |
| `--seed <n>` | seed for `--backend mock` |
| `--volatile <key>` | key stripped from outputs at any depth before comparing; repeatable |
| `--header "K: v"` | static header sent upstream; repeatable |
| `--agent` | read JSON lines of calls from stdin instead of `--script` |

### `bowline eval replay`

Re-runs a recording and fails on any changed result.

    bowline eval replay recording.json

| Flag | Meaning |
|---|---|
| `--backend mock\|replay\|url` | where calls go; defaults to the in-process mock |
| `--url <base>` | the handler for `--backend url` |
| `--fixtures <dir>` | fixture directory for `--backend replay` |
| `--seed <n>` | seed for `--backend mock` |
| `--strict-messages` | compare error messages as well as codes |
| `--header "K: v"` | static header sent upstream; repeatable |

Exits 1 when the recording's contract hash no longer matches, which is the signal to re-record after an intentional API change.

### `bowline gateway`

Composes the configured services and proxies each call to its owner.

| Flag | Meaning |
|---|---|
| `-c <path>` | gateway configuration; defaults to `bowline.gateway.json` |
| `--token <t>` | bearer token for registry upstreams; defaults to `BOWLINE_REGISTRY_TOKEN` |

`bowline gateway compose -o <path>` writes the composed document without serving, taking the same flags.

### `bowline registry serve`

Serves the contract registry over HTTP from a directory of records.

| Flag | Meaning |
|---|---|
| `--store <dir>` | directory holding the records; required |
| `--listen <addr>` | address to listen on; defaults to `:8095` |
| `--token <t>` | bearer token accepted for writes; repeatable. Reads are always open |
| `--ui` | serve the browser UI at `/`; on by default, disable with `--ui=false` |

### `bowline publish`

Publishes this module's contract as a version of a service, or what a consumer uses of one.

| Flag | Meaning |
|---|---|
| `--registry <url>` | base URL of the registry |
| `--service <name>` | the service whose contract to publish |
| `--tag <name>` | tag to move to the published version; defaults to `main` |
| `--ref <sha>` | the source revision the contract was built from |
| `--contract <path>` | the document to publish; generated from the module when absent |
| `--consumer <name>` | the consumer whose usage to publish |
| `--provider <service>` | the service that consumer calls |
| `--usage <path>` | the recorded consumer usage file |
| `--token <t>` | bearer token; defaults to `BOWLINE_REGISTRY_TOKEN` |

### `bowline verify-consumers`

Checks recorded consumer interactions against the current contract.

    bowline verify-consumers [dir]

The directory defaults to `contracts/consumers`. Exits 1 when any recorded interaction no longer holds.

### `bowline migrate-contract`

Rewrites a contract document from an older format version in place.

    bowline migrate-contract [path]

### `bowline version`

Prints the version this binary was built from.

## Environment

| Variable | Read by |
|---|---|
| `BOWLINE_REGISTRY_TOKEN` | `check --registry`, `publish`, `gateway` when a token flag is absent |
| `NO_COLOR` | `dev`, to suppress colour in its output |
