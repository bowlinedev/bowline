# Echo

Echo wraps an `http.Handler` with `echo.WrapHandler`. `Any` with a wildcard covers every method.

source: examples/routers/echo/main.go:12-17

```go
func Mount(h http.Handler) http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.Any("/api/*", echo.WrapHandler(h))
	return e
}
```

Tests: `cd examples/routers/echo && go test ./...`.

Gotchas: Echo's `middleware.RemoveTrailingSlash` and `AddTrailingSlash` rewrite paths. Leave them off the API group. Echo's default HTTP error handler never sees Bowline errors, because Bowline writes its own envelope. `middleware.BodyLimit` in front of the API produces Echo's 413 rather than Bowline's `INVALID_ARGUMENT` envelope. Use `bowline.MaxBodySize` instead.
