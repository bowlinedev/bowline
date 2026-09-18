# Middleware and lifecycle

Middleware wraps a procedure call. It runs after the request has been decoded and validated, and it sees the input value the procedure is about to receive, not the raw HTTP request.

source: middleware.go:8-10

```go
type Next func(ctx context.Context, in any) (any, error)

type Middleware func(next Next) Next
```

`Next` is the rest of the chain. Returning without calling it short-circuits the procedure.

## Attaching it

`Use` on a router applies to every procedure under it, including nested routers. `Use` as a procedure option applies to one procedure. Router middleware runs first, outermost first, then the procedure's own.

source: examples/ledger/api/invoices.go:44-54

```go
func (a *API) invoices() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("get", a.getInvoice, bowline.Description("Get returns one invoice by ID."), bowline.Path("invoices/{id}"), bowline.Tool(bowline.Scope("billing"))),
		bowline.Query("list", a.listInvoices, bowline.Description("List returns a page of invoices, optionally filtered by status."), bowline.Path("invoices"), bowline.Tool(bowline.Scope("billing"))),
		bowline.Mutation("create", a.createInvoice, bowline.Path("invoices"), bowline.Idempotent()),
		bowline.Mutation("void", a.voidInvoice, bowline.Description("Void cancels a draft or sent invoice."), bowline.Path("invoices/{id}"), bowline.Method("DELETE"), bowline.Meta("auth", "admin"), bowline.Errors(InvoiceLocked{}), bowline.Tool(bowline.Scope("billing"), bowline.Destructive()), bowline.Use(voidLimit())),
		bowline.Subscription("watch", a.watchInvoices, bowline.Description("Watch streams every invoice change.")),
		bowline.Upload("attach", a.attach, bowline.Description("Attach stores a file against an invoice.")),
		bowline.Query("attachments", a.listAttachments),
	).Use(RequireToken(a.token))
}
```

Here `RequireToken` guards every invoice procedure and `voidLimit` rate-limits only the void.

## Seeing the typed input

A `Middleware` receives `any`, because one middleware serves procedures with different input types. When a middleware only makes sense for one procedure, `Typed` gives it the real types and does the assertion once:

sketch: an application-supplied middleware over a procedure that takes `VoidInvoiceInput` and returns `ledger.Invoice`

```go
func auditVoid() bowline.Middleware {
	return bowline.Typed(func(ctx context.Context, in VoidInvoiceInput, next bowline.TypedNext[VoidInvoiceInput, ledger.Invoice]) (ledger.Invoice, error) {
		out, err := next(ctx, in)
		if err == nil {
			audit(ctx, "invoice voided", out.ID)
		}
		return out, err
	})
}
```

`Typed[In, Out]` accepts a procedure whose input is `In` or `*In` and forwards it in the same shape, so it works whether the handler takes a value or a pointer. A type that does not line up is an `INTERNAL` error naming both types, rather than a panic.

## The call

`bowline.CallFrom(ctx)` returns the current call. `Procedure` describes what is being called, including its declared tool flags, and `Request` is the HTTP request when there is one, which is how middleware reads headers.

source: middleware.go:12-17

```go
type Call struct {
	Procedure *Procedure
	Request   *http.Request

	header http.Header
}
```

`call.ResponseHeader()` sets headers on the response the procedure is about to produce.

## Observers

Middleware runs inside a call. An observer watches calls without joining the chain, which is what tracing, metrics and audit logs want.

source: typed.go:49-53

```go
type Observer interface {
	HandlerReady(procedures []*Procedure)
	CallStarted(ctx context.Context, call *Call) context.Context
	CallFinished(ctx context.Context, call *Call, err error)
}
```

`CallStarted` returns a context that the procedure and the rest of the chain see, so an observer can attach a span or a request ID. `CallFinished` gets the error the procedure returned, before it is mapped to a status or redacted.

`Observe(o)` registers one. `ObserveFunc(started, finished)` takes two functions instead, and `OnHandlerReady(fn)` takes only the third, which is useful for registering metrics once per procedure at startup:

sketch: registering a counter per procedure when the handler is built

```go
handler := routes.Handler(bowline.OnHandlerReady(func(procedures []*bowline.Procedure) {
	for _, p := range procedures {
		metrics.Register(p.Path, string(p.Kind))
	}
}))
```

`HandlerReady` is called once, with every procedure the handler serves, sorted by path. Observers cost nothing when none are registered: the handler skips the whole path.

For OpenTelemetry, do not write this by hand. `guides/observability.md` covers the `otel` module, which is an observer.
