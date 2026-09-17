# Subscriptions

A subscription is a procedure that pushes a sequence of values to the client. On the server it is a function that receives a typed stream. On the wire it uses server-sent events. In TypeScript it is an async iterable.

## Declaring one

source: examples/ledger/api/invoices.go:56-75

```go
func (a *API) watchInvoices(ctx context.Context, in WatchInput, stream *bowline.Stream[ledger.Invoice]) error {
	changes, stop := a.store.Watch()
	defer stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case inv, ok := <-changes:
			if !ok {
				return nil
			}
			if in.Status != nil && inv.Status != *in.Status {
				continue
			}
			if err := stream.Send(inv); err != nil {
				return err
			}
		}
	}
}
```

It is registered with `bowline.Subscription`:

source: examples/ledger/api/invoices.go:50-50

```go
		bowline.Subscription("watch", a.watchInvoices, bowline.Description("Watch streams every invoice change.")),
```

`Send` returns `context.Canceled` once the client has disconnected. Returning from the function ends the stream. Every value sent goes through the same normalizer as a query output, so nil slices become `[]` and times are RFC 3339. Returning an error ends the stream with an `error` event that carries the usual envelope, including any declared variants.

## The wire

A subscription is a `GET` with the input in the `input` query parameter, or a `POST` if it is marked `Sensitive()`. The request must include `Accept: text/event-stream`. Without that header the response is `INVALID_ARGUMENT` with a message explaining the transport. The response is `text/event-stream` with `Cache-Control: no-store`. The first bytes sent are a `: open` comment, written right after the headers so that proxies forward the headers immediately and the client knows the stream is live before the first message arrives:

```
event: message
data: {"id":3,"status":"sent"}

event: done
data: {}
```

An error looks like this:

```
event: error
data: {"error":{"code":"NOT_FOUND","message":"gone"}}
```

`bowline.Heartbeat(15 * time.Second)` on the handler writes a `: ping` comment at that interval, which keeps idle proxies from closing the connection. Reverse proxies that buffer responses need to be told not to. The handler sets `X-Accel-Buffering: no` for nginx.

The tests in `sse_test.go` cover: three messages followed by `done`, an `error` after two messages, a missing `Accept` header, validation before the stream opens, and `Send` noticing a disconnect.

## Many subscriptions over one connection

Browsers limit the number of connections per origin. An app that holds many subscriptions at once can multiplex them over a single WebSocket. Mount the transport module next to the API handler:

source: examples/ledger/cmd/server/main.go:80-80

```go
	r.Handle("/ws", bowlinews.Handler(routes, bowlinews.Options{OriginPatterns: []string{"localhost:*", "127.0.0.1:*"}, Handler: options}))
```

Then give the client a transport. Queries and mutations continue to use `fetch`, and only subscriptions go over the socket:

source: examples/ledger/web/src/api.ts:45-48

```ts
const options: ClientOptions = { url: "/api" };
if (transportName === "ws") {
  options.transport = websocketTransport(socketUrl());
}
```

Frames are JSON objects with an integer `id` chosen by the client. The client sends `subscribe` (with `path` and `input`) and `stop`. The server sends `data`, `error`, and `done`. When the socket closes, every active subscription fails with `UNAVAILABLE`. Reconnecting is up to the application. The tests in `transport/websocket/handler_test.go` run two subscriptions on one socket and then stop one of them.
