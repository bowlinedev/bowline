# Fiber

Fiber's handler type is `fiber.Handler` on top of fasthttp, not `http.Handler`. It is the one router that needs an adapter module: `github.com/bowlinedev/bowline/adapters/fiber`.

source: examples/routers/fiber/main.go:12-16

```go
func App() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	bowlinefiber.Mount(app, "/api", conformance.Router(), conformance.Options()...)
	return app
}
```

`bowlinefiber.Mount(app, prefix, router, opts...)` registers `prefix + "/*"` for every method through `fasthttpadaptor`, so Bowline's 405 and `Allow` header come through unchanged. `bowlinefiber.Handler(router, opts...)` returns the `fiber.Handler` if you want to register it yourself. `bowlinefiber.HTTPHandler(app)` turns a Fiber app back into an `http.Handler` for use with `httptest`.

Tests: `cd adapters/fiber && go test ./...` runs the conformance suite over a listening Fiber app, including `subscription-stream`. `examples/routers/fiber` runs it again as an example.

Gotchas: fasthttp copies the request body into memory before the adaptor runs. `bowline.MaxBodySize` still produces the 413 envelope, but only after the copy. Set Fiber's `BodyLimit` to the same value to refuse large bodies earlier. Server-sent events pass through the adaptor with flushing intact, but if a custom Fiber middleware buffers the response, subscriptions will stall.
