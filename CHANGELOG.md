# Changelog

## Unreleased

- Security: every error path in the runtime is audited in `docs/security/audit-2026.md`; production now redacts the message, details, and issues of every 5xx, including `bowline.Errorf(bowline.Internal, ...)` and panics, and decoder failures answer `invalid input` instead of echoing the request body.
- Runtime: `bowline.CSRF` rejects cross-origin mutations by origin and fetch metadata, `bowline.SecurityHeaders` sets the header set, `bowline.MaxBody` sets a per-procedure body limit recorded in the contract, and `bowline.RateLimit` is a token bucket middleware with key eviction.
- Benchmarks: `docs/benchmarks.md` reports handler overhead, regeneration latency, and generator throughput, regenerated from a run published on each `main` push with history and a five percent regression gate.

## 0.7.0

- Runtime: `bowline.WithContract` serves `.bowline/contract` and `.bowline/health`; `bowline.Signed` verifies HMAC request signatures before decoding, with the `signing` package for clients.
- Gateway: the `gateway` module composes several service contracts into one document and proxies each call to its owner, with pinned contract hashes, readiness that verifies the pins, header allowlisting, retries for idempotent queries only, and streaming for subscriptions and uploads.
- Registry: the `registry` module stores services, versions, consumers, and compositions behind a `Store` interface with a file-backed implementation, serves them over HTTP, and answers which consumers a candidate contract would break.
- CLI: `bowline gateway`, `bowline gateway compose`, `bowline registry serve`, `bowline publish`, `bowline check --registry`, and `bowline gen --from` to render targets from an existing document.
- Registry UI: `bowline registry serve --ui` serves an embedded browser app listing services and versions, drawing the dependency graph, browsing a contract, and running an impact query against a pasted or uploaded document.
- Examples: a two-service federation with a billing service that reads the ledger over a signed Go client, a gateway that composes both, and a web app on the composed client.
- Supply chain: a per-module dependency allowlist, `govulncheck` on every module, an advisory check scoped to the published packages, Dependabot, and build provenance attestations on release artifacts.
- Project: `GOVERNANCE.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `MAINTAINERS.md`, and `docs/lts.md` with the support windows and the backport rule.

## 0.6.0

- CLI: `dart`, `python`, `rust`, and `elixir` generator targets with goldens compiled by each language's toolchain in CI; naming rules shared across every generator.
- Runtimes: `bowline` on pub.dev, `bowline-client` on PyPI and crates.io, and `bowline_client` on Hex, each with transport, error envelope, validation rules, server-sent events, and uploads.
- Examples: Dart, Python, Rust, and Elixir ledger clients that list, create, validate, void, and subscribe against the real server in CI; the ledger commits every generated client under the drift gate.
- Docs: guides per language with verified snippets, a certification page, and the mapping table extended to every target.

## 0.5.0

- Conformance: the `conformance` package drives any mount through the fixed wire rules; router examples for the standard mux, Chi, Gin, Echo, and Connect run it in CI.
- Adapters: `adapters/fiber` mounts a router on Fiber through fasthttp's adaptor and passes the suite, streaming included.
- Client: `procedureKey`, `walkProcedures`, `InputOf`, and `OutputOf` are shared from `@bowline/client`; `@bowline/client/server` exports `createServerClient` for server-side callers with header forwarding and `fetch` cache passthrough.
- Bindings: `@bowline/swr`, `@bowline/svelte` with SvelteKit `serverClient`, `@bowline/solid`, and `@bowline/vue`, all on the shared key shape.
- CLI: the `go` target generates a standard-library Go client with typed errors, generics, subscriptions as iterators, and uploads.
- Examples: Next.js, React Router, SvelteKit, Astro, and Expo apps against the ledger with end-to-end tests, and a Go-to-Go reports service on the generated client.
- Docs: framework guides whose snippets are verified against the examples by `docs/guides_test.go`.

## 0.4.0

- Contract: format 1.2 adds `example` on fields from the `example` struct tag, checked by the analyzer and emitted into JSON Schemas.
- CLI: `mock` serves generated or recorded responses from the contract alone, with a state model, `--record` and `--replay --strict`, and the playground at `/_playground/`; `verify-consumers` checks recorded consumer files against the contract; `diff` and `check --against` name the consumers a breaking change affects; `eval` gains `--backend mock|replay|url`; `dev --playground` serves the playground for the live contract.
- Playground: the `playground` module embeds a browser app with a router tree, a TypeScript type browser, generated forms, history, and shareable links, calling the API through a same-origin proxy.
- Client: the `record` option and `@bowline/client/node`'s `fileSink` write consumer contracts; `ContractDocument` carries examples.
- Contract tests: the `contracttest` module replays consumer files against a router in `go test`.
- Examples: the ledger carries example tags, a recorded consumer file, a mock-backed browser suite, and the playground at `/playground/`.

## 0.3.0

- Runtime: `bowline.Tool`, `bowline.Scope`, and `bowline.Destructive` mark procedures as LLM tools; the fields are readable from `bowline.CallFrom(ctx).Procedure`.
- Contract: format 1.1 adds `tool` and optional embedded JSON Schemas per procedure; `"schemas": true` in `bowline.json` fills them.
- CLI: `export tools` in Anthropic, OpenAI, and JSON Schema shapes with scope and read-only filters, a `tools` generator target, `mcp` serving tools over stdio or HTTP through a running API, and `eval record` and `eval replay` for deterministic agent runs.
- MCP: the `mcp` module serves protocol revision 2025-06-18 in process with header forwarding, scoping, and rate limits.
- Agents: `github.com/bowlinedev/bowline/agent`, `@bowline/agent`, and `bowline-agent` on PyPI emit tool definitions and dispatch typed calls, with recording tracers for `eval record --agent`.
- Examples: the ledger exposes four tools, guards invoices with `LEDGER_TOKEN`, runs on a fixed clock with `LEDGER_FIXED_TIME`, mounts `/mcp`, and ships a recorded run replayed in CI.

## 0.2.0

- Contract: document format frozen at 1.0 with error variants, subscription and upload kinds, and the idempotent flag; `bowline migrate-contract` converts 0.x documents.
- Runtime: declared error variants, server-sent event subscriptions with heartbeats, typed multipart uploads, idempotency keys with a pluggable store, `Router.Subscribe` for other transports.
- Transport: `transport/websocket` multiplexes subscriptions over one connection.
- CLI: `diff` with added, widened, narrowed, removed, and breaking categories; `check --against` and a composite GitHub Action that comments on pull requests; `export openapi` and OpenAPI output from `gen`; Zod emission for the TypeScript target.
- Client: `.safe` results with typed error narrowing, subscriptions as async iterables with `subscribe` callbacks, a WebSocket transport, uploads, and the `idempotencyKey` option.
- Examples: the ledger streams changes, accepts attachments, declares `InvoiceLocked`, and its end-to-end test runs over both transports.

## 0.1.0

First public alpha.

- Runtime: expression-composed routers, `http.Handler`, sixteen-code error envelope, middleware, validation, output normalization, `Verify`.
- Analyzer: static router evaluation, full mapping table, enums, generics, `WireAs`, diagnostics with positions and fixes.
- CLI: `gen`, `check`, `dev`.
- TypeScript: generated client with hydration, `@bowline/client`, `@bowline/react-query`.
- Examples: ledger with web app and e2e test, minimal net/http.
