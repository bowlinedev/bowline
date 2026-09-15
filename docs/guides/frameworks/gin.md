# Gin

Gin wraps an `http.Handler` with `gin.WrapH`; the three redirect settings must be off so paths reach Bowline unchanged.

source: examples/routers/gin/main.go:12-20

```go
func Mount(h http.Handler) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.RemoveExtraSlash = false
	r.Any("/api/*path", gin.WrapH(h))
	return r
}
```

Proof: `cd examples/routers/gin && go test ./...`.

Gotchas: with `RedirectTrailingSlash` on, Gin answers `echo/` with a 301 instead of forwarding it, which fails the `trailing-slash` case. `gin.Default()` adds a recovery middleware that turns panics into an empty 500 before Bowline's own recovery can write the `INTERNAL` envelope; use `gin.New()` or put Bowline's handler outside that middleware. Gin's `c.Writer` implements `http.Flusher`, so subscriptions stream without extra work.
