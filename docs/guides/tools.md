# Procedures as tools

A procedure becomes an LLM tool only when its declaration says so. Nothing is exposed by default, because a mutation an agent can reach by accident is a worse failure than a missing tool.

## Exposing

```go
bowline.Query("get", a.getInvoice,
	bowline.Description("Get returns one invoice by ID."),
	bowline.Tool(bowline.Scope("billing")),
)
bowline.Mutation("void", a.voidInvoice,
	bowline.Errors(InvoiceLocked{}),
	bowline.Tool(bowline.Scope("billing"), bowline.Destructive()),
)
```

`bowline.Tool(...)` marks exposure. `bowline.Scope(names...)` attaches scope names that consumers filter on, and `bowline.Destructive()` sets the hint that a client should confirm before calling. Queries carry a read-only hint automatically; mutations do not. `Tool()` on a subscription or an upload panics at `NewRouter`, and `bowline gen` reports it as a diagnostic, because neither has a request and response shape a tool call can carry.

The tool name is the procedure path with dots replaced by underscores, `invoices_get`, so it matches the `^[a-zA-Z0-9_-]{1,64}$` rule every provider enforces. Two procedures whose names collide under that rule, `a.b_c` and `a_b.c`, are a diagnostic.

## Schemas

Each exposed procedure carries a JSON Schema for its input and its output, derived from the contract: draft 2020-12, every reachable named type in `$defs`, validation rules as constraints, doc comments as descriptions. Set `"schemas": true` in `bowline.json` and `bowline gen` embeds them in the contract under `procedures[].schemas`:

```json
{ "entry": "./api.Routes", "schemas": true }
```

The derivation table is in `spec/contract.md`. In short: integers carry their width bounds, 64-bit integers encoded as strings carry a digit pattern, `time.Time` is `format: date-time`, `[]byte` is `contentEncoding: base64`, nullable values use `"type": [x, "null"]` or `anyOf`, `required` on a string becomes `minLength: 1`, `min` and `max` become the matching length, value, or item bounds, `oneof` becomes `enum`, and `email`, `url`, and `uuid` become formats. A model that sees `minLength: 1` and `format: email` makes fewer invalid calls than one that reads a bare string.

Declared error variants are listed as the description's last line, `Errors: InvoiceLocked`, because tool protocols have no structured error schema and the model benefits from knowing what can fail.

## Exporting

```bash runnable
bowline export tools --format anthropic --scope billing
```

`--format` is `anthropic`, `openai`, or `json-schema`; the last carries both schemas, the hints, and the scopes. `--scope` repeats and keeps tools that declare at least one given scope; `--read-only` keeps tools with the read-only hint; `--out` writes a file instead of standard output. The same list can be a `gen` target so it is regenerated and checked with everything else:

```json
{ "targets": { "tools": { "out": "agent/tools.json", "format": "openai" } } }
```

The goldens under `cmd/bowline/internal/tools/testdata` are the reference output for the ledger; the Go, TypeScript, and Python agent packages are tested against the same files, so every consumer sees one shape.

## Reading exposure at runtime

`bowline.CallFrom(ctx).Procedure` carries `Exposed`, `ReadOnly`, `Destructive`, and `Scopes`, so middleware can, for example, refuse destructive tools for a service account:

```go
func noDestructiveTools(next bowline.Next) bowline.Next {
	return func(ctx context.Context, in any) (any, error) {
		call := bowline.CallFrom(ctx)
		if call.Procedure.Destructive && isAgent(call.Request) {
			return nil, bowline.Errorf(bowline.PermissionDenied, "agents cannot call destructive tools")
		}
		return next(ctx, in)
	}
}
```

The analyzer fixture `tools` and the test `tool_test.go` in the runtime cover the declarations; `cmd/bowline/internal/jsonschema` holds one schema golden per fidelity row.
