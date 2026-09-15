# Ledger

A small invoicing API that exercises every Bowline feature: queries, mutations, typed errors, validation, subscriptions over server-sent events and WebSocket, uploads, idempotency keys, OpenAPI and Zod export, tool exposure, and an MCP endpoint.

## Run it

```bash
go run ./cmd/server
```

The API listens on `:8080` under `/api`, the WebSocket transport on `/ws`, and the MCP endpoint on `/mcp`. Two environment variables change its behavior:

| Variable | Effect |
|---|---|
| `LEDGER_TOKEN` | When set, every `invoices.*` call needs `Authorization: Bearer <token>`; other calls stay open. |
| `LEDGER_FIXED_TIME` | An RFC 3339 time used as the store clock so seeded timestamps are constant. |

The web app in `web/` is a Vite and React client; `pnpm --filter ledger-web test:e2e` runs the Playwright suite against both transports.

## Tools and MCP

`invoices.get`, `invoices.list`, and `invoices.void` are exposed with scope `billing` and `customers.search` with scope `crm`; `bowline export tools` lists them. Any MCP client can talk to the running server directly at `/mcp`, or through the CLI proxy:

```bash
bowline mcp --url http://localhost:8080/api --header "Authorization: Bearer dev"
```

## Recorded agent run

`evals/list-and-get.json` is a recording of four tool calls made against the server started with `LEDGER_TOKEN=dev` and `LEDGER_FIXED_TIME=2026-09-15T12:00:00Z`. The `eval` workflow replays it on every push with `scripts/eval-ledger.sh`. After an intentional API change, re-record it with `scripts/eval-ledger.sh record` and commit the new file; replay refuses a recording whose contract hash no longer matches.
