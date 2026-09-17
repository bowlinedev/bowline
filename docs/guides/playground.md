# Playground

The playground is a browser app that reads `bowline.contract.json` and lets you browse the router tree, view every type as TypeScript, and call procedures using a generated form. It does not need a build of the app and does not need CORS configured, because it calls the API through a same-origin proxy.

## In your server

source: examples/ledger/cmd/server/main.go:83-83

```go
		r.Handle("/playground/*", http.StripPrefix("/playground", playground.New(api.Contract, playground.WithUpstream("/api"), playground.WithTitle("Ledger playground"))))
```

`playground.New` takes the contract bytes and returns an `http.Handler` that serves the embedded app, `contract.json`, and `/proxy/<procedure>`. `WithUpstream` sets where calls are sent. This can be a same-origin path like `/api`, which is resolved against the incoming request's host, or an absolute URL. The proxy forwards the method, the body, the `input` query parameter, and an allowlist of headers (`Authorization`, `Content-Type`, `Accept`, `Idempotency-Key`, and any `X-` header). `WithHeaderAllowlist` extends the list and `WithTitle` sets the page title. The ledger mounts it outside of production in `examples/ledger/cmd/server/main.go`, and `examples/ledger/cmd/server/mcp_test.go` checks that production builds do not.

## From the CLI

`bowline dev --playground 127.0.0.1:8091` serves the playground for the live contract. It re-reads the contract after every regeneration and forwards calls to the `dev.app` address in `bowline.json`:

```json
{ "entry": "./api.Routes", "dev": { "app": "http://localhost:8080/api" } }
```

`bowline mock` serves it at `/_playground/` on its own address, so the mock and its explorer start together.

## The app

There are three panes. The first is the router tree, showing kinds, docs, deprecations, and tool hints. The second is a type browser that renders each declaration as TypeScript using the same mapping the generator uses. The third is the call panel, with a form derived from the input type: enums as selects, optional fields that can be toggled, nested structs that collapse, arrays with add and remove, and a raw JSON editor kept in sync with the form. Example tags fill in the form's initial values. A headers editor is persisted in the browser, the response shows timestamps in local time, and the last fifty calls are kept as history. The whole view state except `Authorization` is encoded in the URL fragment, so a link reproduces a request.

The `playground/` module is standard library only and embeds the bundle from `playground/ui/dist`. A fresh clone only has a placeholder page with the build instruction. `pnpm --filter @bowlinedev/playground build` fills the directory, CI builds it before the browser tests, and the release workflow commits the bundle into the tree that the `playground/vX.Y.Z` tag points at, since the Go module proxy serves exactly that tree. The Playwright smoke test in `packages/playground/e2e/playground.spec.ts` drives `invoices.get` through the proxy against the mock.
