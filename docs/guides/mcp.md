# MCP server

The `mcp` module serves the procedures you mark with `bowline.Tool()` to any Model Context Protocol client, in the same process as the API and through the same middleware. It speaks protocol version `2025-06-18` over the streamable HTTP transport and depends on nothing outside the standard library.

## Exposing procedures

A procedure is a tool only when it says so. Queries become read-only tools; mutations are read-write, and `Destructive()` marks the ones a client should confirm before calling.

```go
bowline.Query("get", a.getInvoice, bowline.Tool(bowline.Scope("invoices:read")))
bowline.Mutation("void", a.voidInvoice, bowline.Tool(bowline.Scope("invoices:write"), bowline.Destructive()))
```

Subscriptions and uploads cannot be tools; `Tool()` on one panics at `NewRouter`. The tool name is the procedure path with dots replaced by underscores, so `invoices.get` is called `invoices_get`. The description is the procedure's doc comment followed by a final `Errors:` line naming the declared error variants, so a model knows which failures to expect.

Tool schemas come from the contract. Set `"schemas": true` in `bowline.json` so `bowline gen` writes a JSON Schema for each exposed procedure's input and output into `bowline.contract.json`; `mcp.Handler` refuses a contract that carries none.

## Mounting the handler

```go
import "github.com/bowlinedev/bowline/mcp"

//go:embed bowline.contract.json
var contractJSON []byte

routes := api.Routes(store)
handler, err := mcp.Handler(routes, contractJSON, mcp.RateLimit(120, 20))
if err != nil {
	log.Fatal(err)
}
mux.Handle("/api/", http.StripPrefix("/api", routes.Handler()))
mux.Handle("/mcp", handler)
```

Every `tools/call` becomes an in-memory HTTP request against the router's own handler, so the call path is the one your HTTP clients use: decoding, validation, middleware, idempotency, and error mapping all run unchanged. Nothing goes through a socket.

## Options

| Option | Effect |
|---|---|
| `Scopes(names...)` | list and serve only tools that declare at least one of the given scopes |
| `ReadOnly()` | list and serve only read-only tools |
| `RateLimit(perMinute, burst)` | a token bucket per caller, keyed by the SHA-256 of the `Authorization` header; callers without one share a single bucket |
| `ForwardHeaders(names...)` | request headers copied onto the in-process call; the default is `Authorization` and `Cookie` |
| `Runtime(opts...)` | `bowline.HandlerOption` values for the router handler the server dispatches to |
| `Logger(l)` | a `*slog.Logger` for dispatch failures |

A tool hidden by `Scopes` or `ReadOnly` is not listed and a call to it fails with JSON-RPC error `-32602`, the same answer an unknown tool gets. An exhausted rate limit is not a protocol error; it is a tool result with `isError: true` and the text `RESOURCE_EXHAUSTED: rate limit exceeded`, so the model sees it and can back off.

`mcp.NewServer(tools, dispatcher, opts...)` and `mcp.Serve(server, opts...)` are the pieces under `Handler`, for a process that builds its tool list another way or wants to dispatch to a remote Bowline API through its own `Dispatcher`.

## The wire

The transport accepts `POST` with a JSON-RPC message or an array of them and answers with `application/json`; an array is answered with an array. A notification, a message without an `id`, gets `202 Accepted` and no body. `GET` is answered with `405` and an `Allow: POST` header, because the server never opens a server-sent event stream. A message that is not JSON gets `-32700`.

A call and its answer:

```json
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":3}}}
```

```json
{"jsonrpc":"2.0","id":7,"result":{
  "content":[{"type":"text","text":"{\"id\":3,\"title\":\"Rent\"}"}],
  "structuredContent":{"id":3,"title":"Rent"}
}}
```

When the procedure fails, the Bowline error envelope is the structured content and the text is the code, the message, and one line per validation issue:

```json
{"jsonrpc":"2.0","id":8,"result":{
  "isError":true,
  "content":[{"type":"text","text":"INVALID_ARGUMENT: invalid input\ntitle: is required"}],
  "structuredContent":{"error":{"code":"INVALID_ARGUMENT","message":"invalid input","issues":[{"path":["title"],"rule":"required","message":"is required"}]}}
}}
```

`tools/list` reports each tool's `inputSchema`, `outputSchema`, and annotations with `readOnlyHint`, `destructiveHint`, and the procedure path as `title`.

## Authentication

The MCP server adds no authentication of its own. The forwarded headers reach your middleware through `bowline.CallFrom(ctx).Request`, so the rule that guards the API guards the tools:

```go
func requireAuth(next bowline.Next) bowline.Next {
	return func(ctx context.Context, in any) (any, error) {
		token := bowline.CallFrom(ctx).Request.Header.Get("Authorization")
		if !valid(token) {
			return nil, bowline.Errorf(bowline.Unauthenticated, "missing or invalid token")
		}
		return next(ctx, in)
	}
}

routes.Use(requireAuth)
```

A rejected call comes back as a tool result with `UNAUTHENTICATED: missing or invalid token`, which is what a model needs to ask its user for credentials. Put HTTP-level middleware that must see the raw request, such as a cookie session check, in front of the `/mcp` handler itself; it sees the JSON-RPC request before any tool is dispatched.

The tests in `mcp/handler_test.go` cover the transport rules, header forwarding, scoping, and the rate limit; `mcp/server_test.go` covers the handshake and the shape of every `tools/call` outcome.
