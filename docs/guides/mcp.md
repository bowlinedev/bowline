# MCP server

The `mcp` module serves the procedures marked with `bowline.Tool()` to any Model Context Protocol client. It runs in the same process as the API and goes through the same middleware. It implements protocol version `2025-06-18` over the streamable HTTP transport and only depends on the standard library.

## Exposing procedures

A procedure is only a tool if its declaration says so. Queries become read-only tools. Mutations are read-write, and `Destructive()` marks the ones a client should confirm before calling.

source: examples/ledger/api/invoices.go:46-46

```go
		bowline.Query("get", a.getInvoice, bowline.Description("Get returns one invoice by ID."), bowline.Path("invoices/{id}"), bowline.Tool(bowline.Scope("billing"))),
```

source: examples/ledger/api/invoices.go:49-49

```go
		bowline.Mutation("void", a.voidInvoice, bowline.Description("Void cancels a draft or sent invoice."), bowline.Path("invoices/{id}"), bowline.Method("DELETE"), bowline.Meta("auth", "admin"), bowline.Errors(InvoiceLocked{}), bowline.Tool(bowline.Scope("billing"), bowline.Destructive()), bowline.Use(voidLimit())),
```

Subscriptions and uploads cannot be tools. `Tool()` on one of them panics in `NewRouter`. The tool name is the procedure path with dots replaced by underscores, so `invoices.get` becomes `invoices_get`. The description is the procedure's doc comment, followed by a final `Errors:` line that names the declared error variants, so a model knows which failures to expect.

Tool schemas come from the contract. Set `"schemas": true` in `bowline.json` so that `bowline gen` writes a JSON Schema for each exposed procedure's input and output into `bowline.contract.json`. `mcp.Handler` refuses a contract that has no schemas.

## Mounting the handler

Build the handler from the router and the embedded contract:

source: examples/ledger/cmd/server/main.go:73-76

```go
	tools, err := mcp.Handler(routes, api.Contract, mcp.Runtime(options...), mcp.RateLimit(120, 20))
	if err != nil {
		return nil, err
	}
```

Then mount it next to the API:

source: examples/ledger/cmd/server/main.go:79-81

```go
	r.Mount("/api", routes.Handler(slices.Concat(options, browserOptions())...))
	r.Handle("/ws", bowlinews.Handler(routes, bowlinews.Options{OriginPatterns: []string{"localhost:*", "127.0.0.1:*"}, Handler: options}))
	r.Handle("/mcp", tools)
```

Every `tools/call` becomes an in-memory HTTP request against the router's own handler. The call path is the same one your HTTP clients use, so decoding, validation, middleware, idempotency, and error mapping all run as normal. Nothing goes through a socket.

## Options

| Option | Effect |
|---|---|
| `Scopes(names...)` | list and serve only tools that declare at least one of the given scopes |
| `ReadOnly()` | list and serve only read-only tools |
| `RateLimit(perMinute, burst)` | a token bucket per caller, keyed by the SHA-256 of the `Authorization` header; callers without one share a single bucket |
| `ForwardHeaders(names...)` | request headers copied onto the in-process call; the default is `Authorization` and `Cookie` |
| `Runtime(opts...)` | `bowline.HandlerOption` values for the router handler the server dispatches to |
| `Logger(l)` | a `*slog.Logger` for dispatch failures |

A tool that is hidden by `Scopes` or `ReadOnly` is not listed. A call to it fails with JSON-RPC error `-32602`, which is the same response an unknown tool gets. An exhausted rate limit is not a protocol error. It is returned as a tool result with `isError: true` and the text `RESOURCE_EXHAUSTED: rate limit exceeded`, so the model sees it and can back off.

`mcp.NewServer(tools, dispatcher, opts...)` and `mcp.Serve(server, opts...)` are the building blocks under `Handler`. Use them if your process builds its tool list a different way, or if you want to dispatch to a remote Bowline API through your own `Dispatcher`.

## Without changing the app

`bowline mcp` serves the same protocol from the CLI and forwards every call to a running API over HTTP. This gives an app that cannot add a dependency, or one written before tools existed, an MCP server from its contract alone:

```bash
bowline mcp --url http://localhost:8080/api --header "Authorization: Bearer dev" --scope billing
```

Stdio is the default, which is what desktop MCP clients spawn. `--listen 127.0.0.1:9090` serves streamable HTTP instead, and forwards each caller's `Authorization` and `Cookie` headers upstream, where they take precedence over the static `--header` values. `--scope`, `--read-only`, and `--rate N --burst B` mirror the options above. Tools are read from `bowline.contract.json` in the working directory, so the command does not need to build the app.

## The wire

The transport accepts `POST` with a JSON-RPC message, or an array of messages, and responds with `application/json`. An array gets an array back. A notification (a message without an `id`) gets `202 Accepted` and no body. `GET` gets a `405` with an `Allow: POST` header, because the server never opens a server-sent event stream. A message that is not JSON gets `-32700`.

A call and its response:

```json
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"invoices_get","arguments":{"id":3}}}
```

```json
{"jsonrpc":"2.0","id":7,"result":{
  "content":[{"type":"text","text":"{\"id\":3,\"title\":\"Rent\"}"}],
  "structuredContent":{"id":3,"title":"Rent"}
}}
```

When the procedure fails, the Bowline error envelope is used as the structured content, and the text is the code, the message, and one line per validation issue:

```json
{"jsonrpc":"2.0","id":8,"result":{
  "isError":true,
  "content":[{"type":"text","text":"INVALID_ARGUMENT: invalid input\ntitle: is required"}],
  "structuredContent":{"error":{"code":"INVALID_ARGUMENT","message":"invalid input","issues":[{"path":["title"],"rule":"required","message":"is required"}]}}
}}
```

`tools/list` reports each tool's `inputSchema`, `outputSchema`, and annotations with `readOnlyHint`, `destructiveHint`, and the procedure path as `title`.

## Authentication

The MCP server does not add any authentication of its own. The forwarded headers reach your middleware through `bowline.CallFrom(ctx).Request`, so whatever rule guards the API also guards the tools:

source: examples/ledger/api/auth.go:24-42

```go
func RequireToken(token string) bowline.Middleware {
	return func(next bowline.Next) bowline.Next {
		if token == "" {
			return next
		}
		return func(ctx context.Context, in any) (any, error) {
			call := bowline.CallFrom(ctx)
			header := ""
			if call.Request != nil {
				header = call.Request.Header.Get("Authorization")
			}
			got, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				return nil, bowline.Errorf(bowline.Unauthenticated, "a bearer token is required")
			}
			return next(ctx, in)
		}
	}
}
```

A rejected call comes back as a tool result with `UNAUTHENTICATED: missing or invalid token`, which is what a model needs in order to ask its user for credentials. If you have HTTP-level middleware that needs to see the raw request, such as a cookie session check, put it in front of the `/mcp` handler itself. It will see the JSON-RPC request before any tool is dispatched.

The tests in `mcp/handler_test.go` cover the transport rules, header forwarding, scoping, and the rate limit. `mcp/server_test.go` covers the handshake and the shape of every `tools/call` outcome.
