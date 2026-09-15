# Changelog

## 0.2.0

- Contract: document format frozen at 1.0 with error variants, subscription and upload kinds, and the idempotent flag; `bowline migrate-contract` converts 0.x documents.
- Runtime: declared error variants, server-sent event subscriptions with heartbeats, typed multipart uploads, idempotency keys with a pluggable store, `Router.Subscribe` for other transports.
- Transport: `transport/websocket` multiplexes subscriptions over one connection.
- CLI: `diff` with added, widened, narrowed, removed, and breaking categories; `check --against` and a composite GitHub Action that comments on pull requests; `export openapi` and OpenAPI output from `gen`; Zod emission for the TypeScript target.
- Client: `.safe` results with typed error narrowing, subscriptions as async iterables with `subscribe` callbacks, a WebSocket transport, uploads, and the `idempotencyKey` option.
- Examples: the ledger streams changes, accepts attachments, declares `InvoiceLocked`, and its end-to-end test runs over both transports.

## 0.1.0

First public alpha.

- Runtime: expression-composed routers, `http.Handler`, sixteen-code error envelope, middleware, validation, output normalization, `Verify`.
- Analyzer: static router evaluation, full mapping table, enums, generics, `WireAs`, diagnostics with positions and fixes.
- CLI: `gen`, `check`, `dev`.
- TypeScript: generated client with hydration, `@bowline/client`, `@bowline/react-query`.
- Examples: ledger with web app and e2e test, minimal net/http.
