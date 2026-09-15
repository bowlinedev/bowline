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
- Validation from struct tags, enforced on the server and delivered to clients as structured issues.
- One error envelope with sixteen fixed codes and HTTP statuses, and a `BowlineError` class that carries them.
- `bowline check` fails CI when the committed contract or client drifts from the Go code, and the server verifies the committed contract at startup.
- `bowline dev` regenerates in well under a second after every save.
- An `http.Handler` with no dependencies outside the standard library and an overhead under 5 percent against a hand-written handler.

## Documentation

| Page | Contents |
|---|---|
| `docs/quickstart.md` | five minutes from an empty directory to a typed call |
| `docs/guides/errors.md` | codes, `bowline.Errorf`, redaction, `BowlineError` |
| `docs/guides/validation.md` | the eight rules and how issues reach the client |
| `docs/guides/fidelity.md` | the type mapping, the 64-bit rule, dates, `WireAs` |
| `spec/contract.md` | the contract document every generator reads |
| `spec/mapping-table.md` | the normative Go to TypeScript mapping |

Examples: `examples/ledger` is a Chi server with a React web app and an end-to-end test; `examples/nethttp-minimal` is one procedure on the standard library mux.

## Status

This is the 0.1 alpha. Applications need Go 1.24 or later; building the CLI needs Go 1.26 or later, and `go install` fetches that toolchain automatically. Nothing is stable yet: the Go API, the contract document, and the generated code may all change before 1.0. Coming next: subscriptions, uploads, custom typed errors, a frozen contract format with semantic diff and a breaking-change gate, OpenAPI export, and Zod schemas.

## Layout

- `/` runtime module, `github.com/bowlinedev/bowline`
- `/cmd/bowline` the CLI: `gen`, `check`, `dev`
- `/contract` contract document types
- `/packages` npm packages `@bowline/client` and `@bowline/react-query`
- `/spec` contract specification and JSON Schema
- `/docs` guides and the adoption protocol

## License

Apache-2.0. See `LICENSE`.
