# Frameworks

Bowline's runtime is one `http.Handler`, so a Go router needs no adapter; it needs a mount that keeps the full path and forwards every method. The conformance suite in `conformance/` proves the mount: every router example under `examples/routers/` runs it, and any adapter written elsewhere can run it with three lines. On the client side the generated TypeScript client is plain `fetch`, and each framework binding is a thin layer that turns procedures into that framework's data primitives with one shared key shape, `[["invoices", "get"], input]`.

Every snippet in these guides is copied from a file in the repository and named by a `source:` line; `docs/guides_test.go` fails when a snippet and its source drift.

| Guide | What it covers |
|---|---|
| `stdlib.md` | `net/http` mux |
| `chi.md` | Chi |
| `gin.md` | Gin |
| `echo.md` | Echo |
| `connect.md` | Connect beside Bowline on one mux |
| `fiber.md` | the Fiber adapter module |
| `react-query.md` | `@bowline/react-query` |
| `swr.md` | `@bowline/swr` |
| `svelte.md` | `@bowline/svelte` stores and SvelteKit loads |
| `solid.md` | `@bowline/solid` |
| `vue.md` | `@bowline/vue` |
| `nextjs.md` | Next.js App Router with server components and actions |
| `remix.md` | React Router framework mode |
| `sveltekit.md` | SvelteKit pages and server loads |
| `astro.md` | Astro pages with a Solid island |
| `expo.md` | Expo and React Native |
| `go-client.md` | the generated Go client |

## Running the conformance suite

source: examples/routers/chi/conformance_test.go:11-13

```go
func TestConformance(t *testing.T) {
	conformance.Run(t, Mount)
}
```

`Mount` takes the Bowline handler and returns the framework's root handler with it mounted under `/api`. The suite reports one named subtest per wire rule: `success-body`, `get-on-mutation`, `validation-issues`, `body-limit`, `subscription-stream`, `upload`, one `code-*` case per error code, and so on, so a failing adapter says exactly which rule it breaks. `conformance.RunURL(t, "http://host/api")` drives a server that is already running, which is how non-Go adapters and deployed instances are checked.

## Gotchas shared by every router

- Register the mount for every method. Bowline answers `GET` and `POST` itself and returns 405 with an `Allow` header for the rest; a router that only registers `GET` and `POST` hides that answer behind its own 404 or 405.
- Keep trailing slashes and the full path. Bowline routes on the last path segment and accepts `echo/`; routers that redirect or clean paths break the `trailing-slash` case.
- Streaming needs a flushable writer. Subscriptions write server-sent events and flush after each one; a middleware that wraps `http.ResponseWriter` must forward `Flush`, and a reverse proxy must not buffer `text/event-stream`.
- Body limits are Bowline's. `bowline.MaxBodySize` produces the documented 413; a framework limit in front of it produces the framework's error instead.
