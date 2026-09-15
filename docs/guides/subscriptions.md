# Subscriptions

A subscription is a procedure that pushes a sequence of values to the client. On the server it is a function that receives a typed stream; on the wire it is server-sent events; in TypeScript it is an async iterable.

## Declaring one

```go
func (a *API) watchInvoices(ctx context.Context, in WatchInput, stream *bowline.Stream[ledger.Invoice]) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case inv := <-a.store.Changes():
			if err := stream.Send(inv); err != nil {
				return err
			}
		}
	}
}

bowline.Subscription("watch", a.watchInvoices)
```

`Send` returns `context.Canceled` once the client has gone away, and returning from the function ends the stream. Every sent value passes through the same normalizer as a query output, so nil slices are `[]` and times are RFC 3339. Returning an error ends the stream with an `error` event carrying the usual envelope, including declared variants.

## The wire

A subscription is a `GET` with the input in the `input` query parameter, or a `POST` when marked `Sensitive()`, and it requires `Accept: text/event-stream`; a request without it gets `INVALID_ARGUMENT` explaining the transport. The response is `text/event-stream` with `Cache-Control: no-store`:

```
event: message
data: {"id":3,"status":"sent"}

event: done
data: {}
```

An error looks like:

```
event: error
data: {"error":{"code":"NOT_FOUND","message":"gone"}}
```

`bowline.Heartbeat(15 * time.Second)` on the handler writes a `: ping` comment at that interval so idle proxies keep the connection open. Reverse proxies that buffer responses must be told not to; the handler sets `X-Accel-Buffering: no` for nginx.

The tests in `sse_test.go` show every case: three messages then `done`, an `error` after two messages, the missing `Accept` header, validation before the stream opens, and `Send` observing a disconnect.

## Many subscriptions over one connection

Browsers limit connections per origin, so an app that holds many subscriptions can multiplex them over one WebSocket. Mount the transport module next to the API handler:

```go
import bowlinews "github.com/bowlinedev/bowline/transport/websocket"

mux.Handle("/ws", bowlinews.Handler(routes, bowlinews.Options{OriginPatterns: []string{"app.example.com"}}))
```

and hand the client a transport; queries and mutations keep using `fetch` while subscriptions use the socket:

```ts
import { websocketTransport } from "@bowline/client";

const client = createClient({ url: "/api", transport: websocketTransport("wss://api.example.com/ws") });
```

Frames are JSON objects with an integer `id` chosen by the client: `subscribe` with `path` and `input`, `stop`, and from the server `data`, `error`, and `done`. When the socket closes, every active subscription fails with `UNAVAILABLE`; reconnecting is the application's decision. The transport module tests in `transport/websocket/handler_test.go` run two subscriptions on one socket and stop one of them.
