# REST routes

Every procedure is reachable at its own RPC path without any configuration: `POST /api/invoices.create`, or `GET /api/invoices.get?input={...}` for a query. That is enough for a client that was generated from the contract, because it knows where everything lives.

It is not enough when something else has to call the API: a webhook that only sends `DELETE`, a partner who was promised `/v1/invoices/{id}`, or a CDN rule that caches by path. For those, a procedure can declare the route it answers on.

## Declaring a route

`Path` gives the procedure a URL template. `Method` gives it an HTTP method.

source: examples/ledger/api/invoices.go:46-49

```go
		bowline.Query("get", a.getInvoice, bowline.Description("Get returns one invoice by ID."), bowline.Path("invoices/{id}"), bowline.Tool(bowline.Scope("billing"))),
		bowline.Query("list", a.listInvoices, bowline.Description("List returns a page of invoices, optionally filtered by status."), bowline.Path("invoices"), bowline.Tool(bowline.Scope("billing"))),
		bowline.Mutation("create", a.createInvoice, bowline.Path("invoices"), bowline.Idempotent()),
		bowline.Mutation("void", a.voidInvoice, bowline.Description("Void cancels a draft or sent invoice."), bowline.Path("invoices/{id}"), bowline.Method("DELETE"), bowline.Meta("auth", "admin"), bowline.Errors(InvoiceLocked{}), bowline.Tool(bowline.Scope("billing"), bowline.Destructive()), bowline.Use(voidLimit())),
```

That is the whole ledger surface:

```
POST   /invoices
GET    /invoices/{id}
GET    /invoices
DELETE /invoices/{id}
```

The template has no leading or trailing slash, and a parameter wraps a whole segment in braces. `Method` takes `GET`, `POST`, `PUT`, `PATCH` or `DELETE`. Without it a query answers on `GET` and everything else on `POST`, as before.

The declared route is in the contract, so every generated client calls it. Nothing about the Go handler changes: the procedure still takes its input struct and returns its output.

## Where the input comes from

A procedure with a declared path assembles its input from three places:

1. **Path parameters.** Each `{name}` is matched against the input field with that JSON name.
2. **The query string**, for a method that carries no body (`GET`, `DELETE`, `HEAD`). Every remaining field is read from a query parameter of the same name. A repeated parameter fills a slice. An unknown parameter is ignored.
3. **The JSON body**, for a method that carries one.

So `GET /invoices?limit=20&status=sent` fills `ListInvoicesInput`, and `DELETE /invoices/4` fills `VoidInvoiceInput{ID: 4}`. Validation runs afterwards on the assembled value, so a malformed path parameter comes back as an `INVALID_ARGUMENT` with the field in `issues`, exactly like a bad body.

## What `bowline gen` refuses

The analyzer checks the route against the input type, so a mismatch is a build failure rather than a 404 in production:

- a parameter with no matching input field, or a field that is optional or nullable, since a path parameter is always present
- a parameter carried by anything other than a string or an integer, since it travels as one path segment
- on a method with no body, a field that cannot travel in a query string, such as a struct or a map
- `Method` on a subscription or an upload, which have their own transports

## Precedence and mounting

Routes are matched against the end of the request path, which is how a Bowline handler can be mounted anywhere. `r.Mount("/api", routes.Handler())` and `r.Mount("/v2", routes.Handler())` both work, and so does mounting at the root.

When two patterns could match, the longer one wins, and between two of the same length the one with more literal segments wins. So `invoices/summary` is reached before `invoices/{id}`. Two patterns of the same shape on the same method are rejected when the handler is built, rather than silently shadowing each other.

A request whose path matches but whose method does not gets a `405` with an `Allow` header listing the methods that path does accept.

## The RPC path stays

Declaring a route adds one; it does not remove the RPC path. `invoices.void` is still reachable at `/api/invoices.void`, but only with `DELETE`, because that is the method it declares. This matters for the contract diff: changing a method is a breaking change for any consumer that recorded a call, and `bowline check` reports it by name.

## In the other generators

The route travels in the contract as `httpPath`, so the TypeScript, Go, Dart, Python, Rust and Elixir clients all build the same URL. `bowline export openapi` emits the real paths with their `in: path` parameters, and `bowline mock` serves them, so the playground and the mock server behave like the running API.
