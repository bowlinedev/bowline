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
- Every procedure you mark with `bowline.Tool()` is an LLM tool with a faithful JSON Schema, served to MCP clients by one handler or one command, with your own middleware still deciding who may call it.
- Agent SDKs in Go, TypeScript, and Python that emit Anthropic and OpenAI tool definitions and dispatch typed calls, plus recorded runs that replay deterministically in CI.
- A mock server derived from the contract, with deterministic data, a small state model, and record-and-replay, so a frontend team works with the Go backend stopped.
- A playground that browses the router, renders every type as TypeScript, and calls procedures through a same-origin proxy, embeddable in your server or served by `bowline dev` and `bowline mock`.
- Consumer contracts: the client records what it uses, `bowline verify-consumers` and `contracttest` verify it, and the gate names the consumers a breaking change would hit.
- One `http.Handler` for every Go router, proven by a conformance suite that runs through the standard mux, Chi, Gin, Echo, Connect, and a Fiber adapter.
- Bindings for React Query, SWR, Svelte, Solid, and Vue on one shared key shape, a server-side caller for Next.js, React Router, SvelteKit, and Astro, and a generated Go client for service-to-service calls.
- Generated Dart, Python, Rust, and Elixir clients with typed errors, native validation, streaming, and uploads, each backed by a small runtime package and proven against the ledger in CI.
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
| `docs/guides/tools.md` | exposing procedures as LLM tools |
| `docs/guides/mcp.md` | the MCP server, in process or `bowline mcp` |
| `docs/guides/agents.md` | the Go, TypeScript, and Python agent SDKs |
| `docs/guides/evals.md` | recording and replaying agent runs |
| `docs/guides/mock-server.md` | the contract-derived mock server |
| `docs/guides/playground.md` | the embeddable playground |
| `docs/guides/consumer-contracts.md` | recording and verifying consumer usage |
| `docs/guides/frameworks/README.md` | every Go router and frontend framework, with verified snippets |
| `docs/guides/dart.md`, `python.md`, `rust.md`, `elixir.md` | the generated Dart, Python, Rust, and Elixir clients |
| `docs/certification.md` | what makes a client target official |
| `docs/plugins.md` | the external generator protocol for a new language |
| `docs/certified.md` | every generator that has passed `bowline certify` |
| `docs/guides/federation.md` | composing services behind a gateway |
| `docs/guides/registry.md` | the contract registry and impact queries |
| `docs/guides/signing.md` | signed service-to-service calls |
| `docs/guides/security.md` | CSRF, security headers, and the reverse proxy checklist |
| `docs/benchmarks.md` | handler overhead, regeneration latency, and generator throughput |
| `docs/cli.md` | every command, flag, exit code, and JSON output |
| `docs/stability.md` | what does not change inside a major version |
| `spec/contract.md` | the contract document every generator reads |
| `spec/mapping-table.md` | the normative Go to TypeScript mapping |

Examples: `examples/ledger` is a Chi server with a React web app and an end-to-end test; `examples/nethttp-minimal` is one procedure on the standard library mux; `examples/routers/*` mount the conformance router on each Go router; `examples/{nextjs,remix,sveltekit,astro,expo}` are frontend apps on the ledger; `examples/go-client` calls the ledger through the generated Go client.

## Your API as agent tools

```go
bowline.Query("get", a.getInvoice, bowline.Tool(bowline.Scope("billing")))
```

```bash
bowline mcp --url http://localhost:8080/api --header "Authorization: Bearer dev"
```

That is an MCP server for every exposed procedure, with schemas derived from your Go types and validation tags, and every call passing through the same middleware a browser request does. `bowline export tools --format anthropic` prints the same tools for a direct integration, and the agent packages dispatch calls from Go, TypeScript, or Python.

## Status

This is the 0.5 alpha. Applications need Go 1.24 or later; building the CLI needs Go 1.26 or later, and `go install` fetches that toolchain automatically. The contract document format is 1.1, an additive step from the frozen 1.0; the Go API and the generated code may still change before 1.0. Coming next: Dart, Python, and Rust clients.

## Layout

- `/` runtime module, `github.com/bowlinedev/bowline`
- `/cmd/bowline` the CLI: `gen`, `check`, `dev`, `diff`, `export`, `mcp`, `eval`, `mock`, `verify-consumers`, `migrate-contract`
- `/transport/websocket` the WebSocket subscription transport
- `/mcp` the MCP server module
- `/agent` the Go agent SDK
- `/playground` the embeddable playground handler
- `/contracttest` in-process consumer verification for `go test`
- `/conformance` the wire contract suite every mount runs
- `/gateway` contract composition and the federating proxy
- `/registry` the self-hosted contract registry
- `/signing` HMAC request signing, in the root module
- `/adapters/fiber` the Fiber adapter
- `/contract` contract document types
- `/packages` npm packages `@bowline/client`, `@bowline/react-query`, `@bowline/swr`, `@bowline/svelte`, `@bowline/solid`, `@bowline/vue`, and `@bowline/agent`, plus the playground app
- `/python/bowline-agent` the Python agent package
- `/packages/dart/bowline`, `/packages/python/bowline-client`, `/packages/rust/bowline-client`, `/packages/elixir/bowline_client` the client runtimes for the generated Dart, Python, Rust, and Elixir clients
- `/spec` contract specification and JSON Schema
- `/docs` guides and the adoption protocol

## License

Apache-2.0. See `LICENSE`.
