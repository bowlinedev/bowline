# Chi

Chi's `Mount` strips the prefix and forwards every method, which is what Bowline expects.

source: examples/routers/chi/main.go:12-16

```go
func Mount(h http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Mount("/api", h)
	return r
}
```

Tests: `cd examples/routers/chi && go test ./...`. The ledger example in `examples/ledger/cmd/server/main.go` is also a Chi server, with middleware, the WebSocket transport, the MCP endpoint, and the playground mounted next to the API.

Gotchas: Chi's `middleware.StripSlashes` and `RedirectSlashes` rewrite `echo/`. Leave them off the API subtree. Chi's `middleware.Timeout` cancels the request context, which ends subscriptions with `CANCELED`. Give streaming routes a longer timeout or none at all.
