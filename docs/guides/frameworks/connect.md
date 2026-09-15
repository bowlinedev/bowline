# Connect

Connect handlers and Bowline handlers are both `http.Handler`, so one mux serves both: a Connect procedure path beside the Bowline prefix.

source: examples/routers/connect/main.go:27-32

```go
func Mount(h http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", h)
	mux.Handle(PingProcedure, connect.NewUnaryHandler(PingProcedure, ping, connect.WithCodec(jsonCodec{})))
	return mux
}
```

Proof: `cd examples/routers/connect && go test ./...` runs the conformance suite and then calls the Connect endpoint and a Bowline procedure on the same server.

Gotchas: Connect uses `POST` with its own content types under `/<package>.<Service>/<Method>`, and Bowline uses `/<prefix>/<dotted.path>`, so the two never collide as long as the prefix is not a Connect service name. A gradual migration keeps the Connect services and adds Bowline procedures beside them; the generated TypeScript client and the Connect client coexist in the same page because both are plain `fetch`.
