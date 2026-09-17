# Idempotency keys

When a client retries a mutation after a timeout, it cannot know whether the first attempt actually ran. An idempotency key lets the server answer the retry with the stored response from the first attempt, instead of running the mutation a second time.

## Declaring

source: examples/ledger/api/invoices.go:48-48

```go
		bowline.Mutation("create", a.createInvoice, bowline.Idempotent()),
```

Using `Idempotent()` on anything other than a mutation panics in `NewRouter`. The handler also needs a store:

source: examples/ledger/cmd/server/main.go:68-68

```go
		bowline.Idempotency(bowline.MemoryIdempotencyStore(), 24*time.Hour),
```

The in-memory store is intended for single-process deployments and tests. For anything else, implement `IdempotencyStore`. It has three methods: `Begin` claims a key or reports that it is in flight or already stored, `Complete` stores a response with a TTL, and `Abort` releases a claim after a failure.

## Behavior

- A request with an `Idempotency-Key` header claims the key before running. A second request with the same key while the first is still running gets a 409 `ABORTED`.
- A stored response is replayed with the same status and body, plus an `Idempotent-Replayed: true` header. Responses with status 2xx through 4xx are stored, because they are deterministic. A 5xx releases the key so the client can retry.
- A request without the header runs normally, unless `bowline.RequireIdempotencyKey()` is set, in which case a missing header is an `INVALID_ARGUMENT`.
- Keys are scoped. Calling `bowline.WithIdempotencyScope(ctx, tenantID)` in HTTP middleware that runs before the Bowline handler (usually the middleware that authenticates) keeps different callers' keys separate. The stored key is the scope, the procedure path, and the header value joined together.

## On the client

source: packages/client/src/client.test.ts:332-333

```ts
  await client.users.create({ name: "ada", big: 1n }, { idempotencyKey: "abc" });
  expect(seen?.get("idempotency-key")).toBe("abc");
```

Generate the key once per user action and reuse the same key for every retry of that action.

The tests in `idempotency_test.go` cover replay, in-flight conflicts, scopes, the required-header option, and the rule that failures are not stored.
