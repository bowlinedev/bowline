# Fiber

Fiber's handler type is `fiber.Handler` on fasthttp, not `http.Handler`, so it is the one router with an adapter module: `github.com/bowlinedev/bowline/adapters/fiber`.

source: examples/routers/fiber/main.go:12-16

```go
func App() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	bowlinefiber.Mount(app, "/api", conformance.Router(), conformance.Options()...)
	return app
}
```

`bowlinefiber.Mount(app, prefix, router, opts...)` registers `prefix + "/*"` for every method through `fasthttpadaptor`, so Bowline's 405 and `Allow` header come through unchanged. `bowlinefiber.Handler(router, opts...)` returns the `fiber.Handler` for custom registration, and `bowlinefiber.HTTPHandler(app)` turns a Fiber app back into an `http.Handler` for `httptest`.

Proof: `cd adapters/fiber && go test ./...` runs the conformance suite over a listening Fiber app, including `subscription-stream`, and `examples/routers/fiber` runs it again as an example.

Gotchas: fasthttp copies the request body into memory before the adaptor runs, so `bowline.MaxBodySize` still produces the 413 envelope but only after the copy; set Fiber's `BodyLimit` to the same value to refuse large bodies earlier. Server-sent events pass through the adaptor with flushing intact; if a custom Fiber middleware buffers the response, subscriptions stall.
