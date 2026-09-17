# Frameworks

Bowline's runtime is a single `http.Handler`, so a Go router does not need an adapter. It needs a mount that keeps the full path and forwards every method. The conformance suite in `conformance/` checks that a mount does this. Every router example under `examples/routers/` runs it, and any adapter written elsewhere can run it with three lines. On the client side, the generated TypeScript client uses plain `fetch`, and each framework binding is a thin layer that turns procedures into that framework's data primitives. All bindings use the same key shape, `[["invoices", "get"], input]`.

Every snippet in these guides is copied from a file in the repository and named by a `source:` line. `docs/guides_test.go` fails if a snippet and its source ever differ.

| Guide | What it covers |
|---|---|
| `stdlib.md` | `net/http` mux |
| `chi.md` | Chi |
| `gin.md` | Gin |
| `echo.md` | Echo |
| `connect.md` | Connect beside Bowline on one mux |
| `fiber.md` | the Fiber adapter module |
| `react-query.md` | `@bowlinedev/react-query` |
| `swr.md` | `@bowlinedev/swr` |
| `svelte.md` | `@bowlinedev/svelte` stores and SvelteKit loads |
| `solid.md` | `@bowlinedev/solid` |
| `vue.md` | `@bowlinedev/vue` |
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

`Mount` takes the Bowline handler and returns the framework's root handler with it mounted under `/api`. The suite reports one named subtest per wire rule: `success-body`, `get-on-mutation`, `validation-issues`, `body-limit`, `subscription-stream`, `upload`, one `code-*` case per error code, and so on. A failing adapter tells you exactly which rule it breaks. `conformance.RunURL(t, "http://host/api")` drives a server that is already running, which is how non-Go adapters and deployed instances are checked.

## Things to watch for with any router

- Register the mount for every method. Bowline answers `GET` and `POST` itself and returns 405 with an `Allow` header for anything else. A router that only registers `GET` and `POST` hides that response behind its own 404 or 405.
- Keep trailing slashes and the full path. Bowline routes on the last path segment and accepts `echo/`. Routers that redirect or clean paths break the `trailing-slash` case.
- Streaming needs a flushable writer. Subscriptions write server-sent events and flush after each one. A middleware that wraps `http.ResponseWriter` must forward `Flush`, and a reverse proxy must not buffer `text/event-stream`.
- Body limits are Bowline's. `bowline.MaxBodySize` produces the documented 413. A framework limit in front of it produces the framework's error instead.
