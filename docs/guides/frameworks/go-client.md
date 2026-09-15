# Go client

The `go` target generates a Go client from the contract: one file, standard library only, importing nothing but Bowline itself, so a second service calls the first with the same types and error codes it uses on the server.

```json
{ "targets": { "go": { "out": "ledgerclient/client.go", "package": "ledgerclient" } } }
```

`bowline gen` writes and `bowline check` verifies the file with the other targets. The ledger generates `examples/ledger/ledgerclient`, and the reports service in `examples/go-client` calls it:

source: examples/go-client/main.go:117-125

```go
func NewLedgerClient(url, token string) *ledgerclient.Client {
	var opts []ledgerclient.Option
	if token != "" {
		opts = append(opts, ledgerclient.WithHeaders(func(context.Context) (http.Header, error) {
			return http.Header{"Authorization": {"Bearer " + token}}, nil
		}))
	}
	return ledgerclient.New(url, opts...)
}
```

source: examples/go-client/main.go:79-89

```go
func (r *Reports) customer(ctx context.Context, in CustomerInput) (CustomerReport, error) {
	c, err := r.ledger.Customers.Get(ctx, ledgerclient.GetCustomerInput{ID: in.ID})
	if err != nil {
		var be *bowline.Error
		if errors.As(err, &be) && be.Code == bowline.NotFound {
			return CustomerReport{}, bowline.Errorf(bowline.FailedPrecondition, "customer %d is not in the ledger yet", in.ID)
		}
		return CustomerReport{}, err
	}
	return CustomerReport{ID: c.ID, Name: c.Name, Email: c.Email}, nil
}
```

Every failure is a `*bowline.Error`, so `errors.As` and a switch on `Code` replace status-code parsing; declared error variants are reachable through `VariantOf(err)` and `DetailsAs[T](err)`. Structs mirror the contract with `time.Time`, `,string` integers, pointers for nullable and optional fields, named enum types with constants, and Go generics for generic declarations. Subscriptions are `iter.Seq2[Out, error]` over server-sent events and uploads take an `io.Reader` and a file name.

Proof: `cd examples/go-client && go test ./...` starts the ledger in-process, calls it through the generated client, and asserts the outstanding total, the round-tripped timestamp, the `NOT_FOUND` mapping, and the `InvoiceLocked` variant; `cmd/bowline/internal/gen/goclient` vets a golden for every fidelity fixture.

Gotchas: the generated file requires the root module at the version the CLI was built with, so bump both together. Connection failures come back as `UNAVAILABLE` and a canceled context as `CANCELED`, which is what the server's own callers see, so the codes compose across hops without translation.
