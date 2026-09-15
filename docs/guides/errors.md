# Errors

Every failure a Bowline procedure returns reaches the client as one envelope with a code from a fixed set. The Go side returns a `*bowline.Error`; the wire carries `{"error": {...}}` with a matching HTTP status; the TypeScript client rejects with a `BowlineError`.

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

The mapping is fixed in `codes.go` and is not configurable, so every client can rely on it.

## Returning errors from Go

`bowline.Errorf` builds an error with a code and a formatted message. `%w` wraps a cause that `errors.Is` and `errors.As` can still find. From `examples/ledger/api/invoices.go`:

```go
inv, err := a.store.Invoice(in.ID)
if errors.Is(err, ledger.ErrNotFound) {
	return ledger.Invoice{}, bowline.Errorf(bowline.NotFound, "invoice %d not found", in.ID)
}
```

A `*bowline.Error` anywhere in a wrapped chain is used as is. `context.Canceled` and `context.DeadlineExceeded` map to their codes. Any other error becomes `INTERNAL`. Panics are recovered, logged with a stack, and reported as `INTERNAL`.

`WithDetails` attaches a JSON-serializable value that the client receives under `details`:

```go
return nil, bowline.Errorf(bowline.FailedPrecondition, "invoice is locked").WithDetails(map[string]any{"lockedBy": userID})
```

## Declared variants

A procedure can declare typed errors it may return. A variant is a named struct with a `Code` method; its exported fields travel under `details`, and the type name travels as `type`, so clients can narrow on it.

```go
type InvoiceLocked struct {
	ID     int64         `json:"id"`
	Status ledger.Status `json:"status"`
}

func (e InvoiceLocked) Error() string { return fmt.Sprintf("invoice %d is %s", e.ID, e.Status) }
func (e InvoiceLocked) Code() bowline.Code { return bowline.FailedPrecondition }

bowline.Mutation("void", a.voidInvoice, bowline.Errors(InvoiceLocked{}))
```

Returning `InvoiceLocked{ID: 4, Status: "paid"}` from the handler, wrapped or not, produces:

```json
{"error":{"code":"FAILED_PRECONDITION","message":"invoice 4 is paid","type":"InvoiceLocked","details":{"id":4,"status":"paid"}}}
```

Details pass through the same normalizer as outputs, so nil slices are `[]` and `time.Time` fields are RFC 3339. A `Code()` method that returns anything but a single constant is rejected by `bowline gen`, because the client needs the status at generation time. An error with a `Code` method that a procedure did not declare is still serialized with its code and message, without `type`, and logged as a warning in development so the omission is noticed.

## Redaction

In development every `INTERNAL` error carries the underlying message. With `bowline.Production(true)` on the handler the message is replaced by `internal error` and the original is written to the logger. The ledger server switches on the `ENV` variable in `examples/ledger/cmd/server/main.go`.

## The wire envelope

```json
{"error":{"code":"INVALID_ARGUMENT","message":"invalid input","issues":[{"path":["email"],"rule":"email","message":"must be a valid email address"}]}}
```

`details` and `issues` are present only when set. Successful responses have no envelope at all; the body is the output value.

## On the client

```ts
import { BowlineError } from "@bowline/client";

try {
  await client.invoices.get({ id: 42 });
} catch (err) {
  if (err instanceof BowlineError && err.code === "NOT_FOUND") {
    showMissing();
  }
}
```

`BowlineError` carries `code`, `status`, `details`, and `issues`. Network failures reject with `UNAVAILABLE` and status 0; an aborted request rejects with `CANCELED`; a non-JSON failure from a proxy rejects with `UNKNOWN` and the HTTP status. The test file `packages/client/src/client.test.ts` exercises every case.
