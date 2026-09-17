# Errors

Every error returned by a Bowline procedure reaches the client as a single envelope with a code from a fixed set. On the Go side the procedure returns a `*bowline.Error`. On the wire this becomes `{"error": {...}}` with a matching HTTP status. The TypeScript client rejects the promise with a `BowlineError`.

## Codes

| Code | HTTP status | Typical use |
|---|---|---|
| `CANCELED` | 408 | the request context was canceled |
| `UNKNOWN` | 500 | an error of unknown origin |
| `INVALID_ARGUMENT` | 400 | malformed or invalid input, including validation failures |
| `DEADLINE_EXCEEDED` | 408 | the request context deadline passed |
| `NOT_FOUND` | 404 | the requested entity does not exist |
| `ALREADY_EXISTS` | 409 | creating something that exists |
| `PERMISSION_DENIED` | 403 | the caller is known but not allowed |
| `RESOURCE_EXHAUSTED` | 429 | quota or rate limit |
| `FAILED_PRECONDITION` | 412 | the system is not in the required state |
| `ABORTED` | 409 | a concurrency conflict |
| `OUT_OF_RANGE` | 400 | an argument outside its valid range |
| `UNIMPLEMENTED` | 404 | no such procedure |
| `INTERNAL` | 500 | a bug or an unexpected error |
| `UNAVAILABLE` | 503 | a dependency is down |
| `DATA_LOSS` | 500 | unrecoverable data loss |
| `UNAUTHENTICATED` | 401 | the caller is not identified |

The mapping is defined in `codes.go` and cannot be configured. This is so that every client can rely on it.

## Returning errors from Go

`bowline.Errorf` creates an error with a code and a formatted message. If you use `%w` to wrap a cause, `errors.Is` and `errors.As` will still find it. Here is an example from the ledger:

source: examples/ledger/api/invoices.go:78-81

```go
	inv, err := a.store.Invoice(in.ID)
	if errors.Is(err, ledger.ErrNotFound) {
		return ledger.Invoice{}, bowline.Errorf(bowline.NotFound, "invoice %d not found", in.ID)
	}
```

If there is a `*bowline.Error` anywhere in a wrapped chain, it is used as is. `context.Canceled` and `context.DeadlineExceeded` map to their respective codes. Any other error becomes `INTERNAL`. Panics are recovered, logged with a stack trace, and reported as `INTERNAL`.

`WithDetails` attaches a JSON-serializable value. The client receives it under `details`:

sketch: `userID` stands for whatever the application's middleware put on the context

```go
return nil, bowline.Errorf(bowline.FailedPrecondition, "invoice is locked").WithDetails(map[string]any{"lockedBy": userID})
```

## Declared variants

A procedure can declare typed errors that it may return. A variant is a named struct with a `Code` method. Its exported fields are sent under `details`, and the type name is sent as `type`, which lets clients switch on it.

source: examples/ledger/api/errors.go:11-18

```go
type InvoiceLocked struct {
	ID     int64         `json:"id"`
	Status ledger.Status `json:"status"`
}

func (e InvoiceLocked) Error() string { return fmt.Sprintf("invoice %d is %s", e.ID, e.Status) }

func (e InvoiceLocked) Code() bowline.Code { return bowline.FailedPrecondition }
```

The procedure declares the variant with `bowline.Errors`:

source: examples/ledger/api/invoices.go:49-49

```go
		bowline.Mutation("void", a.voidInvoice, bowline.Description("Void cancels a draft or sent invoice."), bowline.Meta("auth", "admin"), bowline.Errors(InvoiceLocked{}), bowline.Tool(bowline.Scope("billing"), bowline.Destructive()), bowline.Use(voidLimit())),
```

If the handler returns `InvoiceLocked{ID: 4, Status: "paid"}`, wrapped or not, the response is:

```json
{"error":{"code":"FAILED_PRECONDITION","message":"invoice 4 is paid","type":"InvoiceLocked","details":{"id":4,"status":"paid"}}}
```

Details go through the same normalizer as outputs, so nil slices become `[]` and `time.Time` fields are formatted as RFC 3339. A `Code()` method that returns anything other than a single constant is rejected by `bowline gen`, since the client needs to know the status at generation time. If an error has a `Code` method but the procedure did not declare it, it is still serialized with its code and message but without `type`. In development a warning is logged so that the missing declaration is noticed.

## Redaction

In development, every error includes the underlying message. When `bowline.Production(true)` is set on the handler, any error that maps to a 5xx status (`INTERNAL`, `UNKNOWN`, `UNAVAILABLE`, `DATA_LOSS`, and panics) has its message replaced with `internal error`, its `details` and `issues` dropped, and the original written to the logger. Messages for 4xx errors are intended for the caller and are kept as is, with one exception: a decoding failure becomes plain `invalid input`, so that the response never quotes the request body. Declared error variants are part of the contract and are never redacted, whatever their status. The ledger server switches this on based on the `ENV` variable in `examples/ledger/cmd/server/main.go`.

## The wire envelope

```json
{"error":{"code":"INVALID_ARGUMENT","message":"invalid input","issues":[{"path":["email"],"rule":"email","message":"must be a valid email address"}]}}
```

`details` and `issues` are only present when set. Successful responses have no envelope. The body is just the output value.

## On the client

sketch: the narrowing pattern; every generated client throws `BowlineError` and nothing else

```ts
import { BowlineError } from "@bowlinedev/client";

try {
  await client.invoices.get({ id: 42 });
} catch (err) {
  if (err instanceof BowlineError && err.code === "NOT_FOUND") {
    showMissing();
  }
}
```

`BowlineError` has `code`, `status`, `details`, and `issues` properties. A network failure rejects with `UNAVAILABLE` and status 0. An aborted request rejects with `CANCELED`. A non-JSON failure from a proxy rejects with `UNKNOWN` and the HTTP status. The test file `packages/client/src/client.test.ts` covers each of these cases.
