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

For anything with more than one process, or where a dropped replay is not acceptable, implement `IdempotencyStore` against a shared store with a real TTL. Three methods: `Begin` claims a key or reports it in flight or stored, `Complete` stores a response, `Abort` releases a claim after a failure.

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
