# Bowline

Bowline makes a Go codebase the single source of truth for an API contract and generates typed clients from it.

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

`bowline gen` writes `bowline.contract.json` and `web/src/bowline.ts`.

```ts
const client = createClient({ url: "http://localhost:8080/api" });
const greeting = await client.greet({ name: "ada" });
```

Rename `Name` in Go and TypeScript stops compiling.

## What it does

The part you'll use on day one:

- **Typed clients, never `any`.** `time.Time` becomes `Date`, 64-bit integers become `bigint`.
- **Validation from struct tags**, enforced on the server, delivered to the client as structured issues, and emitted as Zod schemas.
- **`bowline check`** fails CI the moment a generated file drifts from the Go code.
- **`bowline dev`** regenerates in well under a second per save.

The rest, when you need it:

- Subscriptions as async iterables, over SSE or multiplexed on one WebSocket.
- Typed uploads — your Go function gets the decoded input plus a streaming file.
- One error envelope with sixteen codes, plus declared error variants clients narrow on by name.
- Idempotency keys for mutations.
- OpenAPI 3.1, derived from the contract.
- A mock server built from the contract alone, so the frontend keeps working with your Go server stopped. Deterministic data, record and replay.
- A playground that browses the router and calls procedures through a same-origin proxy. Embed it, or let `bowline dev` serve it.
- Mark a procedure `bowline.Tool()` and it's an MCP tool — schema derived from your Go types, still behind whatever middleware a browser request passes through.
- Clients for TypeScript, Go, Dart, Python, Rust and Elixir. Bindings for React Query, SWR, Svelte, Solid and Vue on one shared key shape.
- Consumer contracts: the client records what it uses, and the gate names who a breaking change would hit.

The runtime is one `http.Handler` with nothing outside the standard library, and it stays under a 5% overhead budget against a hand-written handler. `docs/benchmarks.md` has the numbers and how they're measured — allocations gate the build, timings don't, because a shared CI runner varies by tens of percent between runs of the same commit.

## Install

The CLI, and the runtime your Go server imports:

```bash
go install github.com/bowlinedev/bowline/cmd/bowline@latest
go get github.com/bowlinedev/bowline
```

The client runtime for whichever language you generate into:

```bash
npm  install @bowlinedev/client     # plus @bowlinedev/react-query, swr, solid, svelte, vue
cargo add bowline-client
pip  install bowline-client
dart pub add bowline
```

## Docs

`docs/quickstart.md` gets you from an empty directory to a typed call in about five minutes. `docs/cli.md` covers every command, flag, exit code and JSON output.

`docs/README.md` indexes the rest: guides for errors, validation, type fidelity, subscriptions, uploads, idempotency, the contract and its semantic diff, OpenAPI, tools and MCP, the agent SDKs, evals, the mock server, the playground, consumer contracts, federation, the registry, request signing and security — plus a page each for the generated Dart, Python, Rust and Elixir clients.

`spec/contract.md` is the contract document every generator reads. `spec/mapping-table.md` is the normative Go-to-TypeScript mapping.

Examples live under `examples/`. `ledger` is a Chi server with a React app and an end-to-end test, and it's the one most of the CI suite exercises. `nethttp-minimal` is a single procedure on the standard mux. `routers/*` mount the conformance suite on Chi, Gin, Echo, Connect, Fiber and the standard library. There are frontend apps for Next.js, Remix, SvelteKit, Astro and Expo, and `go-client` calls the ledger through the generated Go client.

## Status

Version 1.0.0. The exported Go API is frozen for 1.x — `scripts/apidiff.sh` enforces it, and `docs/stability.md` states exactly what will not change. The contract document format is 1.2, an additive step over the frozen 1.0. Coming from 0.x, `docs/migration-0.x.md` lists everything the upgrade asks of you.

## Layout

The root module is the runtime, `github.com/bowlinedev/bowline`, and it has no dependencies. `cmd/bowline` is the CLI. Everything else is a separate module you only pull in if you use it: `transport/websocket`, `mcp`, `agent`, `playground`, `contracttest`, `gateway`, `registry`, `adapters/fiber`, and `conformance` for the wire suite every mount runs.

Client runtimes live in `packages/` — the npm packages, plus `packages/{dart,python,rust,elixir}` for the other generated clients. `python/bowline-agent` is the Python agent SDK.

## License

Apache-2.0. See `LICENSE`.

### Generated code is yours

Running `bowline gen` writes code into your project. That output is yours. Use it, change it, ship it under any licence you choose, with no attribution and no obligations from this one — including where the output contains parts copied from Bowline's generators.

Apache-2.0 covers Bowline itself: the runtime, the CLI, the generators, and the client packages you install as dependencies.
