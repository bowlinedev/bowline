# Bowline

Bowline makes a Go codebase the single source of truth for an API contract and projects that contract, with full type fidelity, into typed clients. You write plain Go functions; the contract, the TypeScript types, and the client are generated, committed, and checked for drift in CI.

## Three files

A procedure is a Go function. A router is a value.

```go
type GreetInput struct {
	Name string `json:"name" validate:"required"`
}

type Greeting struct {
	Message string `json:"message"`
}

func Greet(ctx context.Context, in GreetInput) (Greeting, error) {
	return Greeting{Message: "hello, " + in.Name}, nil
}

func Routes() *bowline.Router {
	return bowline.NewRouter(bowline.Query("greet", Greet))
}
```

`bowline.json` names the router and the targets.

```json
{ "entry": "./api.Routes", "targets": { "ts": { "out": "web/src/bowline.ts" } } }
```

`bowline gen` writes `bowline.contract.json` and `web/src/bowline.ts`. The call is typed end to end.

```ts
const client = createClient({ url: "http://localhost:8080/api" });
const greeting = await client.greet({ name: "ada" });
```

## What you get

- A typed client for every procedure, with `Date` and `bigint` where Go has `time.Time` and 64-bit integers, never `any`.
- Subscriptions as async iterables over server-sent events, or multiplexed over one WebSocket with the transport module.
- Typed uploads: a Go function receives the decoded input and a streaming file.
- Validation from struct tags, enforced on the server, delivered to clients as structured issues, and emitted as Zod schemas.
- One error envelope with sixteen fixed codes, plus declared error variants that clients narrow on by name.
- Idempotency keys for mutations with a pluggable store.
- A frozen contract format with a semantic diff, and a CI gate that comments breaking changes on pull requests.
- OpenAPI 3.1 export derived from the contract.
- `bowline check` fails CI when any generated file drifts from the Go code, and the server verifies the committed contract at startup.
- `bowline dev` regenerates in well under a second after every save.
- An `http.Handler` with no dependencies outside the standard library and an overhead under 5 percent against a hand-written handler.

## Documentation

| Page | Contents |
|---|---|
| `docs/quickstart.md` | five minutes from an empty directory to a typed call |
| `docs/guides/errors.md` | codes, `bowline.Errorf`, redaction, `BowlineError` |
| `docs/guides/validation.md` | the eight rules and how issues reach the client |
| `docs/guides/fidelity.md` | the type mapping, the 64-bit rule, dates, `WireAs` |
| `docs/guides/subscriptions.md` | streaming over server-sent events and WebSocket |
| `docs/guides/uploads.md` | typed multipart uploads |
| `docs/guides/idempotency.md` | idempotency keys and stores |
| `docs/guides/contract.md` | the contract, the semantic diff, and the breaking-change gate |
| `docs/guides/openapi.md` | OpenAPI 3.1 export |
| `spec/contract.md` | the contract document every generator reads |
| `spec/mapping-table.md` | the normative Go to TypeScript mapping |

Examples: `examples/ledger` is a Chi server with a React web app and an end-to-end test; `examples/nethttp-minimal` is one procedure on the standard library mux.

## Status

This is the 0.2 alpha. Applications need Go 1.24 or later; building the CLI needs Go 1.26 or later, and `go install` fetches that toolchain automatically. The contract document format is frozen at 1.0; the Go API and the generated code may still change before 1.0. Coming next: every procedure as an LLM tool and `bowline mcp`.

## Layout

- `/` runtime module, `github.com/bowlinedev/bowline`
- `/cmd/bowline` the CLI: `gen`, `check`, `dev`, `diff`, `export`, `migrate-contract`
- `/transport/websocket` the WebSocket subscription transport
- `/contract` contract document types
- `/packages` npm packages `@bowline/client` and `@bowline/react-query`
- `/spec` contract specification and JSON Schema
- `/docs` guides and the adoption protocol

## License

Apache-2.0. See `LICENSE`.
