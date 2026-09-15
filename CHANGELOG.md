# Changelog

## 0.3.0

- Runtime: `bowline.Tool`, `bowline.Scope`, and `bowline.Destructive` mark procedures as LLM tools; the fields are readable from `bowline.CallFrom(ctx).Procedure`.
- Contract: format 1.1 adds `tool` and optional embedded JSON Schemas per procedure; `"schemas": true` in `bowline.json` fills them.
- CLI: `export tools` in Anthropic, OpenAI, and JSON Schema shapes with scope and read-only filters, a `tools` generator target, `mcp` serving tools over stdio or HTTP through a running API, and `eval record` and `eval replay` for deterministic agent runs.
- MCP: the `mcp` module serves protocol revision 2025-06-18 in process with header forwarding, scoping, and rate limits.
- Agents: `github.com/bowlinedev/bowline/agent`, `@bowline/agent`, and `bowline-agent` on PyPI emit tool definitions and dispatch typed calls, with recording tracers for `eval record --agent`.
- Examples: the ledger exposes four tools, guards invoices with `LEDGER_TOKEN`, runs on a fixed clock with `LEDGER_FIXED_TIME`, mounts `/mcp`, and ships a recorded run replayed in CI.

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
