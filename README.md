# Bowline

Bowline generates typed API clients from Go code. You write Go functions and register them on a router. The CLI reads the Go code and writes a contract file plus a client for each language you configure. The contract file is committed and checked in CI, so the clients cannot drift from the server.

## Example

A procedure is a Go function. A router is a value that holds procedures.

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

A config file points at the router and lists the targets:

```json
{ "entry": "./api.Routes", "targets": { "ts": { "out": "web/src/bowline.ts" } } }
```

Running `bowline gen` writes `bowline.contract.json` and `web/src/bowline.ts`. The generated client is typed:

```ts
const client = createClient({ url: "http://localhost:8080/api" });
const greeting = await client.greet({ name: "ada" });
```

If you rename `Name` in the Go struct, the TypeScript code will fail to compile after regeneration.

## Features

- Typed clients. `time.Time` maps to `Date`, 64-bit integers map to `bigint`. The generated code does not use `any`.
- Validation from struct tags. The server enforces the rules and returns failures as a list of issues. The TypeScript target can also emit Zod schemas.
- `bowline check` fails CI if any generated file is out of date.
- `bowline dev` watches the module and regenerates on save.
- Subscriptions, either over server-sent events or multiplexed over a WebSocket.
- Multipart uploads. The Go function receives the decoded input and a streaming file.
- A single error envelope with a fixed set of codes. Procedures can also declare named error variants that clients can switch on.
- Idempotency keys for mutations.
- OpenAPI 3.1 export.
- A mock server built from the contract. It produces deterministic data and can record and replay real responses, so frontend work can continue without the Go server running.
- A playground that lists the procedures and lets you call them through a proxy. It can be embedded in your server or served by `bowline dev`.
- MCP support. Mark a procedure with `bowline.Tool()` and it is exposed to MCP clients with a JSON Schema derived from the Go types. Calls go through the same middleware as normal requests.
- Client generators for TypeScript, Go, Dart, Python, Rust and Elixir. Bindings for React Query, SWR, Svelte, Solid and Vue.
- Consumer contracts. A client can record which procedures and fields it uses. `bowline check` then reports which consumers a change would break.

The runtime is a single `http.Handler` and only imports the standard library. Overhead compared to writing the handler by hand is under 5%. See `docs/benchmarks.md` for the measurements. Note that CI only fails on allocation regressions, not on timing, since timings on shared runners are too noisy to gate on.

## Installation

Install the CLI and add the runtime to your Go module:

```bash
go install github.com/bowlinedev/bowline/cmd/bowline@latest
go get github.com/bowlinedev/bowline
```

Then install the client runtime for the language you are generating:

```bash
npm  install @bowlinedev/client     # plus @bowlinedev/react-query, swr, solid, svelte, vue
cargo add bowline-client
pip  install bowline-client
dart pub add bowline
```

## Documentation

`docs/quickstart.md` walks through setting up a project from scratch. `docs/cli.md` documents each command, its flags, exit codes and JSON output.

`docs/README.md` is the index for the rest. There are guides on errors, validation, type mapping, subscriptions, uploads, idempotency, the contract format and diff, OpenAPI, tools and MCP, the agent SDKs, evals, the mock server, the playground, consumer contracts, federation, the registry, request signing and security. Each of the Dart, Python, Rust and Elixir clients has its own page.

The normative specs are in `spec/`. `spec/contract.md` describes the contract document and `spec/mapping-table.md` describes how Go types map to TypeScript.

Example projects are in `examples/`. `ledger` is a Chi server with a React frontend and end-to-end tests, and it is what most of the CI suite runs against. `nethttp-minimal` is a single procedure on the standard library mux. `routers/` contains one example per supported Go router (Chi, Gin, Echo, Connect, Fiber, stdlib). There are also frontend examples for Next.js, Remix, SvelteKit, Astro and Expo, and `go-client` shows a Go program calling the ledger through the generated Go client.

## Status

Current version is 1.0.0. The exported Go API is frozen for the 1.x line. `scripts/apidiff.sh` checks this on every pull request and `docs/stability.md` lists what is covered. The contract format is at 1.2, which is backwards compatible with 1.0. If you are upgrading from a 0.x release, see `docs/migration-0.x.md`.

## Repository layout

The root module (`github.com/bowlinedev/bowline`) is the runtime and has no dependencies. `cmd/bowline` is the CLI. The remaining directories are separate Go modules that you only need if you use that feature: `transport/websocket`, `mcp`, `agent`, `playground`, `contracttest`, `gateway`, `registry`, `adapters/fiber`, and `conformance`.

Client runtimes are under `packages/`. This includes the npm packages and the Dart, Python, Rust and Elixir runtime packages. `python/bowline-agent` is the Python agent SDK.

## License

Apache-2.0. See `LICENSE`.

Code written by `bowline gen` into your project is yours. You can use it, change it and distribute it under whatever license you want, with no attribution requirement, even where the generated output includes fragments copied from Bowline's own generators. The Apache-2.0 license applies to Bowline itself: the runtime, the CLI, the generators and the client runtime packages.
