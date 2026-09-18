# Procedures as tools

A procedure only becomes an LLM tool if its declaration says so. Nothing is exposed by default. A mutation that an agent can reach by accident is a worse problem than a tool that is missing.

## Exposing

A read-only query with a scope:

source: examples/ledger/api/invoices.go:46-46

```go
		bowline.Query("get", a.getInvoice, bowline.Description("Get returns one invoice by ID."), bowline.Path("invoices/{id}"), bowline.Tool(bowline.Scope("billing"))),
```

A mutation that a client should confirm before calling:

source: examples/ledger/api/invoices.go:49-49

```go
		bowline.Mutation("void", a.voidInvoice, bowline.Description("Void cancels a draft or sent invoice."), bowline.Path("invoices/{id}"), bowline.Method("DELETE"), bowline.Meta("auth", "admin"), bowline.Errors(InvoiceLocked{}), bowline.Tool(bowline.Scope("billing"), bowline.Destructive()), bowline.Use(voidLimit())),
```

`bowline.Tool(...)` marks the procedure as exposed. `bowline.Scope(names...)` attaches scope names that consumers can filter on. `bowline.Destructive()` sets a hint that a client should confirm before calling. Queries get a read-only hint automatically, mutations do not. Using `Tool()` on a subscription or an upload panics in `NewRouter`, and `bowline gen` reports it as a diagnostic, because neither has a request and response shape that a tool call can carry.

The tool name is the procedure path with dots replaced by underscores, for example `invoices_get`. This matches the `^[a-zA-Z0-9_-]{1,64}$` rule that every provider enforces. If two procedures produce the same name under that rule (for example `a.b_c` and `a_b.c`), that is a diagnostic.

## Schemas

Each exposed procedure has a JSON Schema for its input and its output, derived from the contract. These use draft 2020-12, put every reachable named type in `$defs`, express validation rules as constraints, and use doc comments as descriptions. Set `"schemas": true` in `bowline.json` and `bowline gen` will embed them in the contract under `procedures[].schemas`:

```json
{ "entry": "./api.Routes", "schemas": true }
```

The full derivation table is in `spec/contract.md`. In summary: integers carry their width bounds, 64-bit integers encoded as strings carry a digit pattern, `time.Time` is `format: date-time`, `[]byte` is `contentEncoding: base64`, nullable values use `"type": [x, "null"]` or `anyOf`, `required` on a string becomes `minLength: 1`, `min` and `max` become the matching length, value, or item bounds, `oneof` becomes `enum`, and `email`, `url`, and `uuid` become formats. A model that can see `minLength: 1` and `format: email` makes fewer invalid calls than one that just sees a string.

Declared error variants are listed on the last line of the description as `Errors: InvoiceLocked`. Tool protocols have no structured error schema, and it helps the model to know what can fail.

## Exporting

```bash runnable
bowline export tools --format anthropic --scope billing
```

`--format` is one of `anthropic`, `openai`, or `json-schema`. The last one includes both schemas, the hints, and the scopes. `--scope` can be repeated and keeps tools that declare at least one of the given scopes. `--read-only` keeps only tools with the read-only hint. `--out` writes to a file instead of standard output. The same list can also be a `gen` target, so that it is regenerated and checked along with everything else:

```json
{ "targets": { "tools": { "out": "agent/tools.json", "format": "openai" } } }
```

The goldens under `cmd/bowline/internal/tools/testdata` are the reference output for the ledger. The Go, TypeScript, and Python agent packages are tested against the same files, so every consumer sees the same shape.

## Reading exposure at runtime

`bowline.CallFrom(ctx).Procedure` has `Exposed`, `ReadOnly`, `Destructive`, and `Scopes` fields. Middleware can use these, for example to refuse destructive tools for a service account:

sketch: an application-supplied middleware; `isAgent` is whatever the deployment uses to recognise a service account

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

The analyzer fixture `tools` and the runtime test `tool_test.go` cover the declarations. `cmd/bowline/internal/jsonschema` has one schema golden per fidelity row.
