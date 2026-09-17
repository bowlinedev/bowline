# net/http

The standard mux needs a trailing slash on the pattern, so that every procedure under the prefix reaches the handler.

source: examples/routers/stdlib/main.go:11-15

```go
func Mount(h http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", h)
	return mux
}
```

Tests: `cd examples/routers/stdlib && go test ./...` runs the conformance suite through `Mount` and again against a live server.

Gotchas: `http.ServeMux` redirects `/api` to `/api/` with a 301. This only matters for a request to the bare prefix, since procedure paths always have a segment after it. Go 1.22 method patterns like `GET /api/` would hide the 405 that Bowline returns for other methods, so register the path without a method.
