# Connect

Connect handlers and Bowline handlers are both `http.Handler`, so one mux can serve both. A Connect procedure path sits next to the Bowline prefix.

source: examples/routers/connect/main.go:27-32

```go
func Mount(h http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", h)
	mux.Handle(PingProcedure, connect.NewUnaryHandler(PingProcedure, ping, connect.WithCodec(jsonCodec{})))
	return mux
}
```

Tests: `cd examples/routers/connect && go test ./...` runs the conformance suite, then calls the Connect endpoint and a Bowline procedure on the same server.

Gotchas: Connect uses `POST` with its own content types under `/<package>.<Service>/<Method>`, and Bowline uses `/<prefix>/<dotted.path>`. The two do not collide as long as the prefix is not a Connect service name. For a gradual migration, keep the Connect services and add Bowline procedures alongside them. The generated TypeScript client and the Connect client can coexist on the same page, since both use plain `fetch`.
