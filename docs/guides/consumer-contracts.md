# Consumer contracts

A consumer contract records what one client actually uses: which procedures, with which inputs, and which fields of each response it read. The provider verifies those recordings on every change, so a backend change that breaks a real consumer fails CI with the consumer's name, not a generic breaking-change warning.

## Recording

`@bowline/client` records through the `record` option, a sink that receives every call's procedure, method, input, and raw response before hydration:

```ts
export const client = createClient({ url: "/api", record: sink });
```

Under Node, `@bowline/client/node` exports `fileSink(consumer, path, { provider })`, which deduplicates by procedure and canonical input and writes the consumer file on `flush()`. In a browser suite the app collects interactions on `window` and the test harness writes them; the ledger does exactly that in `examples/ledger/web/src/api.ts` and `examples/ledger/web/e2e/record.ts`, and `RECORD=1 pnpm test:e2e` refreshes `examples/ledger/contracts/consumers/ledger-web.json`.

The file is plain JSON, one object per interaction, and lives in the provider repository under `contracts/consumers/`:

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

needs only the contract document and the consumer files. For every interaction it checks that the procedure still exists with the same method, that a successful interaction's input is still accepted by the input type and its rules, and that every field present in a recorded response still exists at the same path with a compatible node: same primitive class, recorded enum values still declared, arrays, maps, and structs matched structurally. Values are never compared, only shapes, so a consumer file does not go stale when data changes. A failure names the consumer, the procedure, the path, and the reason:

```
broken    consumer ledger-web: invoices.list → response.items.0.total: field removed
```

The `ci` workflow runs it for the ledger; `cmd/bowline/internal/consumers/testdata` holds the reference report for each kind of break.

## Dynamic verification

`github.com/bowlinedev/bowline/contracttest` replays each interaction against the router in-process and requires the recorded status, the recorded error code and variant for error responses, and the shape rule on the live body. It catches what the contract cannot express, such as a procedure that now errors. From `examples/ledger/api/consumers_test.go`:

```go
func TestConsumers(t *testing.T) {
	contracttest.VerifyConsumers(t, New(ledger.NewStore(time.Now), slog.Default(), "").Router(), "../contracts/consumers",
		contracttest.WithContract(Contract),
		contracttest.WithSetup(func(testing.TB) {}),
	)
}
```

`WithHeaders` supplies credentials for guarded procedures, `WithSetup` runs before every interaction, and `WithHandler` replaces the router's handler when the app wraps it. Interactions run in file order, sorted by procedure and input, so a `create` precedes the `void` of the invoice it made.

## In the breaking-change gate

`bowline check --against` and `bowline diff --consumers` annotate every breaking change with the consumers whose recordings touch the changed path and their interaction counts, or label it `unused by consumers`. A change to a shared type counts every consumer that reads that type through any procedure. The pull request comment therefore reads "removes field `total`; breaks `ledger-web` (1 interactions)", which is the sentence a reviewer needs.

To see all three fail at once, rename `Total` in `examples/ledger/ledger/types.go`, regenerate, and run `bowline verify-consumers`, `go test ./api -run TestConsumers`, and `bowline check --against main` in `examples/ledger`.
