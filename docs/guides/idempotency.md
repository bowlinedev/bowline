# Idempotency keys

A client that retries a mutation after a timeout cannot know whether the first attempt ran. An idempotency key lets the server answer the retry with the stored response of the first attempt instead of running the mutation twice.

## Declaring

source: examples/ledger/api/invoices.go:48-48

```go
		bowline.Mutation("create", a.createInvoice, bowline.Idempotent()),
```

`Idempotent()` on anything but a mutation panics at `NewRouter`. The handler needs a store:

source: examples/ledger/cmd/server/main.go:68-68

```go
		bowline.Idempotency(bowline.MemoryIdempotencyStore(), 24*time.Hour),
```

The in-memory store is for single-process deployments and tests. Anything else implements `IdempotencyStore`, three methods: `Begin` claims a key or reports it in flight or stored, `Complete` stores a response with a TTL, and `Abort` releases a claim after a failure.

## Behavior

- A request with an `Idempotency-Key` header is claimed before it runs. A second request with the same key while the first is running gets 409 `ABORTED`.
- The stored response is replayed with the same status and body and an `Idempotent-Replayed: true` header. Responses with status 2xx through 4xx are stored, because they are deterministic; a 5xx releases the key so the client can retry.
- A request without the header runs normally unless `bowline.RequireIdempotencyKey()` is set, which turns a missing header into `INVALID_ARGUMENT`.
- Keys are scoped. `bowline.WithIdempotencyScope(ctx, tenantID)` in HTTP middleware that runs before the Bowline handler, usually the one that authenticates, keeps two callers' keys apart. The stored key is the scope, the procedure path, and the header joined.

## On the client

source: packages/client/src/client.test.ts:332-333

```ts
  await client.users.create({ name: "ada", big: 1n }, { idempotencyKey: "abc" });
  expect(seen?.get("idempotency-key")).toBe("abc");
```

Generate the key once per user action and reuse it for every retry of that action.

The tests in `idempotency_test.go` cover replay, in-flight conflicts, scopes, the required-header option, and the rule that failures are not stored.
