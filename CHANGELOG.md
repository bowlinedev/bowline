# Changelog

## 1.4.0

- MCP: every listed tool now carries all four annotation hints. `idempotentHint` comes from the procedure's `Idempotent()` declaration, or from being a query, and `openWorldHint` is false because a procedure's domain is the API itself. Only two hints were sent before, and the protocol's defaults for the missing pair are the cautious ones — `destructiveHint` true, `openWorldHint` true — so a read-only query was being presented to hosts as potentially destructive and reaching outside the system.

- OpenAPI: a field's `example` now reaches the exported document. The contract has carried examples since format 1.2 and the export silently dropped every one, so documentation tools showed none and SDK generators had nothing to sample.
- OpenAPI: `"openapi": {"autoPatch": true}` in `bowline.json` documents the `PATCH` route that `AutoPatch()` serves, with both patch media types, the path parameters, the write procedure's security, and the `412`, `415` and `428` responses. A generated route that no reader could discover was documentation the contract was failing to tell the truth about.

- PATCH: a generated `PATCH` now runs through the CSRF guard and honours an `Idempotency-Key`, which it did not before. A forged cross-origin `PATCH` was accepted, and a repeated idempotency key ran the write twice; both are fixed and covered by tests.
- PATCH: JSON Patch now supports arrays, which RFC 6902 requires and the first implementation did not have at all: `add` inserts and shifts, `-` appends, `remove` shifts left, and indices are checked against the array length. It also enforces the rules it was missing — `replace` and `remove` fail on a location that does not exist, an index may not have a leading zero or be negative, `-` only addresses the end when adding, and a location may not be moved into one of its own children. `add` onto an existing object member replaces it, which it should, while `replace` on an absent member now fails, which it did not.
- Conditional requests: `If-Match` uses strong comparison and `If-None-Match` uses weak comparison, as RFC 9110 requires. Before, both stripped the weak prefix, so a weak entity tag wrongly satisfied `If-Match` — the one place where the distinction protects against a lost update. A `PATCH` whose `If-None-Match` matches is now refused with `412`.
- Errors: an `about:blank` problem now uses the HTTP status phrase as its `title`, which RFC 9457 recommends. A type base still uses the Bowline code.
- PATCH: `If-Match` is checked against the entity tag of the value just read, so a stale patch answers `412` without writing. `AutoPatch(RequireIfMatch())` refuses an unconditional patch with `428` and returns the current tag, which makes a lost update impossible. Without it, two patches to different fields can still interleave and drop one; that is inherent to read-modify-write and a test demonstrates it.

## 1.3.0

- Errors: `ProblemDetails()` adds an RFC 9457 `application/problem+json` representation, chosen by content negotiation so a client that does not ask for it still gets the frozen envelope. Production redaction applies to both shapes. `ProblemTypeBase` sets the `type` URI prefix; without it the type is `about:blank`, as the RFC specifies.
- Conditional requests: `ETags()` hashes the body of a cacheable read, sets an `ETag` and answers `304 Not Modified` for a matching `If-None-Match`. A procedure can set its own tag with `Call.SetETag`, which is what you want when the store already has a version. `IfMatch` and `IfNoneMatch` hand the caller's tags to a procedure so it can refuse a stale write itself; the runtime does not guess, because only the store knows the current version.
- PATCH: `AutoPatch()` derives a `PATCH` route for every path carrying both a `GET` query and a `PUT` mutation. It reads through the real query, applies the patch and writes through the real mutation, so validation, middleware and idempotency all run. Accepts `application/merge-patch+json` (RFC 7386) and `application/json-patch+json` (RFC 6902, including `test`).

- Security: a procedure can now describe how a caller authenticates. `Router.Scheme(name, scheme)` declares a `BearerAuth`, `BasicAuth` or `APIKeyAuth` scheme, `Router.Secure(names...)` requires schemes for everything beneath it, `Public()` opts a procedure out and `Requires(names...)` adds one. It describes and never enforces: middleware still decides who gets in, and a test pins that.
- OpenAPI: the export now carries `components.securitySchemes`, a `security` requirement per operation, and `tags` taken from the mount name. Until now the export could not say an API was authenticated at all, so an SDK generated from it by Fern, Speakeasy or openapi-generator had no credentials and documentation tools showed the API as open. This is what makes the export a usable fallback for languages Bowline does not generate.
- Contract format 1.4: adds the top-level `security` map and `security` on procedures. The schema's `$id` now tracks the format version, which it had stopped doing at 1.2.
- Performance: matching a REST route no longer allocates. Segment kinds are resolved when the table is built rather than per comparison, path parameters travel in a small slice instead of a map, and the segments themselves stay on the stack. A literal route match went from 77ns and one allocation to 57ns and none; a route with a parameter from 189ns and 384 bytes to 75ns and 32. The empty request body for a call that carries none is no longer allocated per call.

- Stores: three shared idempotency stores, each in its own module. `stores/sql` covers Postgres and SQLite and carries no third-party dependency at all, because it binds through `database/sql` and the application brings its own driver; it exposes `Migrate` to create the table and `Sweep` to delete expired rows. `stores/redis` covers Redis through `go-redis`, and needs no sweeping because Redis expires keys itself. Until now only the in-process store shipped, so any deployment with more than one replica had idempotency that silently did not work across them.
- Stores: `idempotencytest.Verify` is the conformance suite all four stores are held to, exported so an application's own implementation can be checked against the same invariants, including that exactly one of sixteen concurrent callers is told a key is new.

## 1.2.0

- Runtime: `bowline.CallTimeout(d)` bounds how long one procedure may run and answers `DEADLINE_EXCEEDED`; subscriptions are exempt because they are long-lived by design. `bowline.Drain(ctx)` ends in-flight subscriptions when that context is cancelled, so `http.Server.Shutdown` does not wait on an idle stream for its whole grace period. Both are provisional; see `docs/provisional.md`.
- Idempotency: the in-memory store is now bounded. It was the only in-memory store without a limit, so a caller sending a fresh `Idempotency-Key` on every request grew the process without bound, and its sweep scanned the whole map under the lock on every call. It now holds at most ten thousand keys, evicts the ones closest to expiry, never evicts a key whose request is still running, and sweeps at most once a second. Under enough unique-key traffic an older key is dropped early and a retry re-runs the mutation rather than replaying.
- Docs: `docs/guides/production.md` is the first-deployment checklist — the handler options that are off by default, the `http.Server` timeouts Bowline cannot set, the shutdown order for subscriptions, and what to watch.
- Testing: a soak harness (`scripts/soak.sh`) that runs the handler under concurrency and fails if goroutines, heap or the idempotency store grow; fault-injection tests for a failing idempotency store and a subscriber that stops reading; and a goroutine-leak assertion for subscriptions.

## 1.1.0

Procedures can now answer on a URL and an HTTP method you choose, rather than only on their RPC path. Everything that reads the contract follows: all six generated clients, the OpenAPI export, the mock server, the playground, and the consumer-contract verifier.

- REST routes: `bowline.Path("invoices/{id}")` gives a procedure a URL template and `bowline.Method("DELETE")` gives it a method. Path parameters bind to input fields by their JSON name, and on a method that carries no body the remaining fields bind from the query string. `bowline gen` rejects a template whose parameters do not match the input type, a parameter carried by anything but a string or integer, an optional parameter, and a field that cannot travel in a query string. `docs/guides/routing.md` covers it. The RPC path stays reachable, on the declared method.
- Clients: the TypeScript, Go, Dart, Python, Rust and Elixir clients build the declared URL, with path parameters and query strings. The contract carries the route as `httpPath`.
- Typed middleware: `bowline.Typed[In, Out]` gives a middleware the procedure's real input and output types instead of `any`, accepting a handler that takes either a value or a pointer.
- Lifecycle observers: `bowline.Observe`, `bowline.ObserveFunc` and `bowline.OnHandlerReady` watch calls from outside the middleware chain. `CallStarted` returns a context the call then sees, and `CallFinished` gets the error the procedure returned, before it is mapped to a status or redacted. A handler with no observers skips the path entirely.
- Observability: the new `otel` module turns each call into an OpenTelemetry server span named after the procedure, with `bowline.procedure`, `bowline.kind`, `http.request.method` and `bowline.code` attributes, plus `bowline.call.duration` and `bowline.call.count`. It extracts W3C trace context from the request. The runtime itself keeps its standard-library-only dependency list. See `docs/guides/observability.md`.
- Security: the CSRF guard now checks every unsafe method, not only `POST`. Before custom methods every mutation was a `POST`, so guarding `POST` was equivalent to guarding every state-changing request; once a procedure could declare `DELETE`, a forged cross-site `DELETE` reached it. `GET`, `HEAD`, `OPTIONS` and `TRACE` are exempt, everything else is checked.
- Consumer contracts: `contracttest` replays each recorded interaction against the route and method it was recorded with, instead of posting to the RPC path. A recording made before this release still replays.
- Mock server: `bowline mock` serves declared routes, binding path parameters and the query string against the contract's input type and answering `405` with `Allow` for a method a path does not take.
- Playground: calls declared routes and shows the method and path next to each procedure.
- Gateway: a composed contract no longer carries the services' declared routes, because the gateway serves each procedure at `service.procedure` and does not proxy those URLs. A client generated from a composed document therefore calls the gateway's own paths.
- Contract format 1.3: adds `httpPath` on procedures and widens `method`. Every 1.x reader accepts it; a reader that does not know the field ignores it. `contract.Version` moves with the format, which is the one exported constant whose value is not frozen.

## 1.0.0

First stable release. `docs/stability.md` states what will not change inside 1.x: the Go API, the minimum Go version, the contract document format, generated code, the CLI and its machine-readable output, and the wire format. `docs/migration-0.x.md` lists everything a 0.x upgrade asks of you, and `docs/lts.md` gives the support windows.

- Security: every error path in the runtime is audited in `docs/security/audit-2026.md`; production now redacts the message, details, and issues of every 5xx, including `bowline.Errorf(bowline.Internal, ...)` and panics, and decoder failures answer `invalid input` instead of echoing the request body.
- Runtime: `bowline.CSRF` rejects cross-origin mutations by origin and fetch metadata, `bowline.SecurityHeaders` sets the header set, `bowline.MaxBody` sets a per-procedure body limit recorded in the contract, and `bowline.RateLimit` is a token bucket middleware with key eviction.
- Benchmarks: `docs/benchmarks.md` reports handler overhead, regeneration latency, and generator throughput, regenerated from a run published on each `main` push with history. Allocation counts gate the build because they are deterministic; wall-clock timings are reported but never enforced, since a shared runner varies by tens of percent between runs of the same commit.
- CLI: `check --json` and `diff --json` for machine-readable output, `docs/cli.md` documenting every command, flag, exit code and output shape with a test that checks it against the binary, and `docs/stability.md` stating what does not change inside a major version.
- Generators: an external generator protocol so a new language can be added without forking (`docs/plugins.md`), and `bowline certify`, which proves a generator against the fidelity corpus and writes `docs/certified.md`.
- Signing: `Sign` draws a nonce into the canonical string and `bowline.Signed` rejects a replayed signature through a bounded `signing.ReplayCache`; a signature without a nonce still verifies, so 0.7.0 callers keep working.
- Fuzzing: eleven Go fuzz targets and a client property test run nightly and for ten seconds in CI; they found and fixed a panic value reaching production clients, four generator panics on malformed documents, non-deterministic TypeScript output when two type IDs share a name, and `@bowlinedev/client` throwing a bare `SyntaxError` on a non-JSON 2xx body.
- API: `docs/api-freeze.md` lists every exported identifier 1.x will guarantee, pinned by a test, and `scripts/apidiff.sh` reports incompatible changes against the previous tag on every pull request; it becomes a hard gate at the first 1.x tag.
- Incompatible since 0.7.0, both deliberate: `bowline.Call` is no longer comparable, because it now carries the response headers a middleware can set, and `signing.Verify` takes variadic options for the replay cache. Calls are unaffected; comparing a `Call` value or assigning `Verify` to a function variable is not.
- CSRF: a request carrying no `Origin`, `Referer`, or `Sec-Fetch-Site` is now allowed rather than rejected, matching `net/http.CrossOriginProtection`. Rejecting those protected nothing, since an attacker who is not driving a victim's browser can send the request directly, and it broke every generated non-browser client, `curl`, and service-to-service call. A browser that announced itself through `Sec-Fetch-Site` but sent no `Origin` is still refused, so `CSRF` is now safe to mount on a route that serves both browsers and machines.
- Performance: the handler reads the `input` query parameter without parsing the whole query string and reads a request body of known length into an exactly sized buffer. Bowline now runs slightly faster than the equivalent hand-written `net/http` handler, with fewer allocations; a large request body costs 15% less memory.
- Performance: the generators write string parts directly instead of concatenating them first, cutting Dart generation time by 10% and Elixir by 9%.

## 0.7.0

- Runtime: `bowline.WithContract` serves `.bowline/contract` and `.bowline/health`; `bowline.Signed` verifies HMAC request signatures before decoding, with the `signing` package for clients.
- Gateway: the `gateway` module composes several service contracts into one document and proxies each call to its owner, with pinned contract hashes, readiness that verifies the pins, header allowlisting, retries for idempotent queries only, and streaming for subscriptions and uploads.
- Registry: the `registry` module stores services, versions, consumers, and compositions behind a `Store` interface with a file-backed implementation, serves them over HTTP, and answers which consumers a candidate contract would break.
- CLI: `bowline gateway`, `bowline gateway compose`, `bowline registry serve`, `bowline publish`, `bowline check --registry`, and `bowline gen --from` to render targets from an existing document.
- Registry UI: `bowline registry serve --ui` serves an embedded browser app listing services and versions, drawing the dependency graph, browsing a contract, and running an impact query against a pasted or uploaded document.
- Examples: a two-service federation with a billing service that reads the ledger over a signed Go client, a gateway that composes both, and a web app on the composed client.
- Supply chain: a per-module dependency allowlist, `govulncheck` on every module, an advisory check scoped to the published packages, Dependabot, and build provenance attestations on release artifacts.
- Project: `docs/lts.md` with the support windows and the backport rule.

## 0.6.0

- CLI: `dart`, `python`, `rust`, and `elixir` generator targets with goldens compiled by each language's toolchain in CI; naming rules shared across every generator.
- Runtimes: `bowline` on pub.dev, `bowline-client` on PyPI and crates.io, and `bowline_client` on Hex, each with transport, error envelope, validation rules, server-sent events, and uploads.
- Examples: Dart, Python, Rust, and Elixir ledger clients that list, create, validate, void, and subscribe against the real server in CI; the ledger commits every generated client under the drift gate.
- Docs: guides per language with verified snippets, a certification page, and the mapping table extended to every target.

## 0.5.0

- Conformance: the `conformance` package drives any mount through the fixed wire rules; router examples for the standard mux, Chi, Gin, Echo, and Connect run it in CI.
- Adapters: `adapters/fiber` mounts a router on Fiber through fasthttp's adaptor and passes the suite, streaming included.
- Client: `procedureKey`, `walkProcedures`, `InputOf`, and `OutputOf` are shared from `@bowlinedev/client`; `@bowlinedev/client/server` exports `createServerClient` for server-side callers with header forwarding and `fetch` cache passthrough.
- Bindings: `@bowlinedev/swr`, `@bowlinedev/svelte` with SvelteKit `serverClient`, `@bowlinedev/solid`, and `@bowlinedev/vue`, all on the shared key shape.
- CLI: the `go` target generates a standard-library Go client with typed errors, generics, subscriptions as iterators, and uploads.
- Examples: Next.js, React Router, SvelteKit, Astro, and Expo apps against the ledger with end-to-end tests, and a Go-to-Go reports service on the generated client.
- Docs: framework guides whose snippets are verified against the examples by `docs/guides_test.go`.

## 0.4.0

- Contract: format 1.2 adds `example` on fields from the `example` struct tag, checked by the analyzer and emitted into JSON Schemas.
- CLI: `mock` serves generated or recorded responses from the contract alone, with a state model, `--record` and `--replay --strict`, and the playground at `/_playground/`; `verify-consumers` checks recorded consumer files against the contract; `diff` and `check --against` name the consumers a breaking change affects; `eval` gains `--backend mock|replay|url`; `dev --playground` serves the playground for the live contract.
- Playground: the `playground` module embeds a browser app with a router tree, a TypeScript type browser, generated forms, history, and shareable links, calling the API through a same-origin proxy.
- Client: the `record` option and `@bowlinedev/client/node`'s `fileSink` write consumer contracts; `ContractDocument` carries examples.
- Contract tests: the `contracttest` module replays consumer files against a router in `go test`.
- Examples: the ledger carries example tags, a recorded consumer file, a mock-backed browser suite, and the playground at `/playground/`.

## 0.3.0

- Runtime: `bowline.Tool`, `bowline.Scope`, and `bowline.Destructive` mark procedures as LLM tools; the fields are readable from `bowline.CallFrom(ctx).Procedure`.
- Contract: format 1.1 adds `tool` and optional embedded JSON Schemas per procedure; `"schemas": true` in `bowline.json` fills them.
- CLI: `export tools` in Anthropic, OpenAI, and JSON Schema shapes with scope and read-only filters, a `tools` generator target, `mcp` serving tools over stdio or HTTP through a running API, and `eval record` and `eval replay` for deterministic agent runs.
- MCP: the `mcp` module serves protocol revision 2025-06-18 in process with header forwarding, scoping, and rate limits.
- Agents: `github.com/bowlinedev/bowline/agent`, `@bowlinedev/agent`, and `bowline-agent` on PyPI emit tool definitions and dispatch typed calls, with recording tracers for `eval record --agent`.
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
- TypeScript: generated client with hydration, `@bowlinedev/client`, `@bowlinedev/react-query`.
- Examples: ledger with web app and e2e test, minimal net/http.
