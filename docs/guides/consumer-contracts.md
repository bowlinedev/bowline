# Consumer contracts

A consumer contract records what a particular client actually uses: which procedures, with which inputs, and which fields of each response it read. The provider checks these recordings on every change. A backend change that would break a real consumer then fails CI with that consumer's name, rather than a generic breaking-change warning.

## Recording

`@bowlinedev/client` records via the `record` option. This is a sink that receives the procedure, method, input, and raw response (before hydration) for every call:

source: examples/ledger/web/src/api.ts:49-51

```ts
if (record !== undefined) {
  options.record = record;
}
```

Under Node, `@bowlinedev/client/node` exports `fileSink(consumer, path, { provider })`. It deduplicates by procedure and canonical input and writes the consumer file when you call `flush()`. In a browser test suite the app collects interactions on `window` and the test harness writes them out. The ledger does this in `examples/ledger/web/src/api.ts` and `examples/ledger/web/e2e/record.ts`. Running `RECORD=1 pnpm test:e2e` refreshes `examples/ledger/contracts/consumers/ledger-web.json`.

The file is plain JSON with one object per interaction. It lives in the provider repository under `contracts/consumers/`:

```json
{
  "bowline": "1.2",
  "consumer": "ledger-web",
  "provider": "ledger",
  "interactions": [
    {
      "procedure": "invoices.list",
      "method": "GET",
      "input": { "limit": 20 },
      "response": { "status": 200, "body": { "items": [] } }
    }
  ]
}
```

## Static verification

```bash runnable
bowline verify-consumers
```

This only needs the contract document and the consumer files. For each interaction it checks that the procedure still exists with the same method, that the input of a successful interaction is still accepted by the input type and its rules, and that every field present in a recorded response still exists at the same path with a compatible node. Compatible means the same primitive class, recorded enum values still declared, and arrays, maps, and structs matched structurally. Values are not compared, only shapes, so a consumer file does not go stale when the data changes. A failure names the consumer, the procedure, the path, and the reason:

```
broken    consumer ledger-web: invoices.list → response.items.0.total: field removed
```

The `ci` workflow runs this for the ledger. `cmd/bowline/internal/consumers/testdata` has a reference report for each kind of break.

## Dynamic verification

`github.com/bowlinedev/bowline/contracttest` replays each interaction against the router in-process. It requires the recorded status, the recorded error code and variant for error responses, and applies the shape rule to the live body. This catches things the contract cannot express, for example a procedure that now returns an error. From the ledger:

source: examples/ledger/api/consumers_test.go:12-17

```go
func TestConsumers(t *testing.T) {
	contracttest.VerifyConsumers(t, New(ledger.NewStore(time.Now), slog.Default(), "").Router(), "../contracts/consumers",
		contracttest.WithContract(Contract),
		contracttest.WithSetup(func(testing.TB) {}),
	)
}
```

`WithHeaders` supplies credentials for guarded procedures. `WithSetup` runs before every interaction. `WithHandler` replaces the router's handler when the app wraps it. Interactions run in file order, sorted by procedure and input, so that a `create` runs before the `void` of the invoice it created.

## In the breaking-change gate

`bowline check --against` and `bowline diff --consumers` annotate every breaking change with the consumers whose recordings touch the changed path and their interaction counts, or label it `unused by consumers`. A change to a shared type counts every consumer that reads that type through any procedure. The pull request comment then reads something like "removes field `total`; breaks `ledger-web` (1 interactions)", which is what a reviewer needs to know.

To see all three checks fail at once, rename `Total` in `examples/ledger/ledger/types.go`, regenerate, and run `bowline verify-consumers`, `go test ./api -run TestConsumers`, and `bowline check --against main` in `examples/ledger`.
