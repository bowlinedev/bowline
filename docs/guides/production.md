# Running in production

Every option below is off by default, because a default that changes behaviour is a default that surprises someone. That makes this page the checklist: a handler built with no options is a development handler.

## The handler

source: examples/ledger/cmd/server/main.go:64-72

```go
func newHandler(routes *bowline.Router, production bool) (http.Handler, error) {
	options := []bowline.HandlerOption{
		bowline.Production(production),
		bowline.WithContract(api.Contract),
		bowline.Idempotency(bowline.MemoryIdempotencyStore(), 24*time.Hour),
		bowline.Heartbeat(15 * time.Second),
		bowline.MaxUploadSize(8 << 20),
		bowline.SecurityHeaders(),
	}
```

| Option | Why it matters in production |
|---|---|
| `Production(true)` | Redacts the message, details and issues of every 5xx, including panics, and logs the original instead. Without it a stack trace or a connection string can reach a caller. |
| `CSRF(CSRFOptions{...})` | Rejects cross-origin state-changing requests. Required for any API a browser calls with cookies. Set `TrustFetchMetadata: true` only behind a proxy that strips `Sec-Fetch-Site` from untrusted callers. |
| `SecurityHeaders()` | Sets `X-Content-Type-Options: nosniff` and `Cache-Control: no-store` on responses that carry data. |
| `MaxBodySize(n)` | Caps the request body. The default is 1 MiB; a procedure can lower it with `MaxBody`. |
| `MaxUploadSize(n)` | Caps an upload. Separate from the body limit because uploads are streamed. |
| `CallTimeout(d)` | Bounds how long one procedure may run. Without it a slow handler holds a connection until the client or the server gives up. Subscriptions are exempt, because they are long-lived by design. |
| `Idempotency(store, ttl)` | Needed only if a mutation declares `Idempotent()`. See the store note below. |
| `Logger(l)` | Where errors and panics go. The default is `slog.Default()`. |

## Describing authentication

Bowline does not authenticate anyone. Your middleware does. What the runtime can do is *describe* the scheme so that the contract, the OpenAPI export and every generated client agree on how a caller proves who they are:

sketch: declare the scheme once, require it everywhere, opt the health check out

```go
func (a *API) Router() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("health", a.health, bowline.Public()),
		bowline.Mount("invoices", a.invoices()),
	).Use(a.requireToken).
		Scheme("bearer", bowline.BearerAuth("opaque")).
		Secure("bearer")
}
```

`Scheme` declares a named scheme: `BearerAuth(format)`, `BasicAuth()` or `APIKeyAuth(in, name)` where `in` is `header`, `query` or `cookie`. `Secure` requires named schemes for every procedure under that router, inherited through mounts. `Public()` opts one procedure out, and `Requires("adminKey")` adds another scheme to one procedure on top of what it inherits. Requiring two schemes means both apply.

This is a description, not a gate. A procedure that declares `bearer` still runs if the caller sends no token; only your middleware can refuse. `TestSecurityDeclarationDoesNotEnforce` pins that, because a declaration that looked like enforcement would be the worst kind of security bug.

The payoff is in the export. `bowline export openapi` now emits `components.securitySchemes`, a `security` requirement on each operation, and `tags` from the mount name. That is what a documentation tool reads to show a lock icon, and what Fern, Speakeasy or openapi-generator read to put credentials into an SDK for a language Bowline does not generate itself.

## Errors a non-Bowline client can read

A generated client understands Bowline's error envelope, because it was generated from the same contract. Anything else — a browser, a partner's HTTP library, an API gateway — does better with the standard shape. `ProblemDetails()` adds an RFC 9457 representation, chosen by content negotiation:

sketch: the frozen envelope stays the default; only a client that asks gets problem+json

```go
handler := routes.Handler(
	bowline.ProblemDetails(bowline.ProblemTypeBase("https://api.example.com/errors/")),
)
```

A request with `Accept: application/problem+json` gets `{"type": "...", "title": "Not Found", "status": 404, "detail": "...", "code": "NOT_FOUND"}`. Every other request gets the envelope it got before, which is why this does not break the frozen wire format. Redaction applies to both: a 5xx says `internal error` in either shape. Without a type base the `type` is `about:blank`, which is what RFC 9457 says an unspecified type means.

## Caching and optimistic concurrency

`ETags()` hashes the response of a cacheable read and answers `304 Not Modified` when the caller's `If-None-Match` still matches. It costs one SHA-256 over the response body, so it is off by default:

sketch: the runtime computes the tag; a procedure may set its own instead

```go
handler := routes.Handler(bowline.ETags())

func (a *API) getInvoice(ctx context.Context, in GetInvoiceInput) (ledger.Invoice, error) {
	inv, err := a.store.Invoice(in.ID)
	if err != nil {
		return ledger.Invoice{}, err
	}
	bowline.CallFrom(ctx).SetETag(inv.Version)
	return inv, nil
}
```

A procedure-supplied tag wins, which is what you want when the store already has a version: the tag then means "this row", not "these bytes".

For writes, `bowline.IfMatch(ctx)` returns the tags the caller sent so a mutation can refuse a stale update. The runtime deliberately does not decide this for you, because only the store knows the current version:

sketch: optimistic concurrency, enforced by the procedure

```go
if !slices.Contains(bowline.IfMatch(ctx), current.Version) {
	return ledger.Invoice{}, bowline.Errorf(bowline.FailedPrecondition, "the invoice has changed")
}
```

## PATCH without writing one

`AutoPatch()` derives a `PATCH` route for every path that has both a `GET` query and a `PUT` mutation. A patch request reads the current value through the real query, applies the patch, and writes the result through the real mutation — so validation, middleware, authorisation and idempotency all run exactly as they would for a hand-written call. Both `application/merge-patch+json` (RFC 7386) and `application/json-patch+json` (RFC 6902, including `test`) are accepted.

```
PATCH /invoices/3
Content-Type: application/merge-patch+json

{"note": "net 60"}
```

The read is a real call, so a `PATCH` to something that does not exist answers with the read's own `404`, and a failing `test` operation answers `400` without writing anything.

## The HTTP server

Bowline is an `http.Handler`, so the connection-level limits are the standard library's and Bowline cannot set them for you:

sketch: the timeouts a public listener needs; `ReadHeaderTimeout` is the one that stops a slow-header attack

```go
server := &http.Server{
	Addr:              addr,
	Handler:           mux,
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       30 * time.Second,
	WriteTimeout:      0,
	IdleTimeout:       120 * time.Second,
	MaxHeaderBytes:    1 << 16,
}
```

`WriteTimeout` must be `0` if you serve subscriptions or uploads, because it applies to the whole response and would cut a stream off mid-flight. Bound those with `CallTimeout` and the client's own deadline instead.

## Shutting down

`http.Server.Shutdown` waits for every handler to return. A subscription returns when its client disconnects, so a shutdown can wait for the full grace period on an idle stream. Give the handler a drain context and cancel it first:

sketch: drain first, then shut the listener down

```go
drain, stopStreams := context.WithCancel(context.Background())
handler := routes.Handler(bowline.Drain(drain), bowline.Production(true))

<-quit
stopStreams()
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.Shutdown(ctx)
```

Cancelling the drain context ends every in-flight subscription, so `Shutdown` only has to wait for unary calls. Without it, `Shutdown` blocks until the last subscriber leaves or the timeout fires.

## The idempotency store

`MemoryIdempotencyStore` keeps at most ten thousand keys in the process and evicts the ones closest to expiry when full. It never evicts a key whose request is still running. That bound exists so a caller sending a fresh `Idempotency-Key` on every request cannot grow the process without limit, and it has a consequence: under enough unique-key traffic an older key is dropped early, and a retry of that request runs the mutation again instead of replaying.

For anything with more than one process, or where a dropped replay is not acceptable, use a shared store. Three ship with Bowline, each in its own module so the runtime keeps its standard-library-only dependency list:

| Module | Backend | Dependencies |
|---|---|---|
| `stores/sql` | Postgres | none; it binds through `database/sql`, so you bring your own driver |
| `stores/sql` | SQLite | none; same |
| `stores/redis` | Redis | `github.com/redis/go-redis/v9` |

sketch: Postgres, with the driver your application already imports

```go
db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
store, err := bowlinesql.New(db, bowlinesql.Postgres)
if err := store.Migrate(ctx); err != nil {
	return err
}
handler := routes.Handler(bowline.Idempotency(store, 24*time.Hour))
```

`Migrate` creates the table and its expiry index, and is safe to call on every boot. `Sweep` deletes expired rows; run it from a periodic job, because a SQL store has no background timer of its own. The Redis store needs neither: Redis expires keys itself.

All three pass the same conformance suite, `idempotencytest.Verify`, which is exported so your own implementation can be held to the identical invariants: a fresh key is new, a claimed key is in flight, a completed key replays byte for byte, an aborted key is claimable again, and exactly one of sixteen concurrent callers is told the key is new.

To write your own, implement the three methods: `Begin` claims a key or reports it in flight or stored, `Complete` stores a response, `Abort` releases a claim after a failure. The claim must be atomic, or two processes will both run the mutation.

A store that returns an error fails closed: `Begin` failing answers `UNAVAILABLE` and the mutation never runs. A `Complete` that fails leaves the caller with its response and releases the key, so a retry re-runs the mutation rather than replaying a response that was never stored.

## What to watch

- `bowline.call.count` and `bowline.call.duration` by `bowline.code`, from the `otel` module. A rise in `INTERNAL` is the signal that matters; `INVALID_ARGUMENT` is usually a client.
- Goroutine count, if you serve subscriptions. One stream is one goroutine plus one for its heartbeat.
- Process memory, if you use the in-memory idempotency store or rate limiter. Both are bounded, but the bound is per process.

`scripts/soak.sh 300` runs the handler under concurrency for five minutes and fails if goroutines, heap, or the idempotency store grow. Run it against your own procedures before a first deployment; it is the cheapest substitute for traffic.

## Before the first deployment

- [ ] `Production(true)` is set, and an error from a procedure shows `internal error` rather than its message
- [ ] `CSRF` is set if a browser calls the API, and a forged cross-origin request answers 403
- [ ] `SecurityHeaders()` is set
- [ ] `CallTimeout` is set, and a slow procedure answers 408 rather than hanging
- [ ] `ReadHeaderTimeout` is set on the `http.Server`
- [ ] `Drain` is wired to shutdown if you serve subscriptions
- [ ] the idempotency store is shared if you run more than one process
- [ ] `bowline check --against` runs in CI, so a breaking contract change cannot merge silently
