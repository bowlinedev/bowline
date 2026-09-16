# Migrating across 0.x

Bowline's 0.x line has been additive since the first alpha. Nothing exported in 0.1.0 has been renamed or removed, and a client generated against an older contract keeps working against a newer server. This page lists what a release asks you to do, in order, so you can skip the releases you are already past.

For release cadence, support windows, and what gets backported, see `docs/lts.md`.

## 0.1.0 to 0.2.0

The contract document format froze at 1.0. Documents written by the 0.1.0 alpha use an earlier shape and must be converted once:

```bash
bowline migrate-contract api/bowline.contract.json
```

Commit the converted document. Everything after this is a regeneration, not a migration.

Subscriptions, uploads, declared error variants, and idempotency keys arrived in this release. They are opt-in: a router that does not use them generates the same client it did before.

## 0.2.0 to 0.3.0

The format moved to 1.1 to carry the `tool` block and optional embedded JSON Schemas. The new fields are written only when you ask for them, so regenerating is enough:

```bash
bowline gen
```

Set `"schemas": true` in `bowline.json` if you want the schemas inline. Tool metadata appears when you mark a procedure with `bowline.Tool`, `bowline.Scope`, or `bowline.Destructive`.

## 0.3.0 to 0.4.0

The format moved to 1.2 to carry `example` on fields, filled from the `example` struct tag. Regenerate; contracts without the tag are unchanged.

`bowline verify-consumers` and the `record` client option arrived here. Neither runs unless you turn it on.

## 0.4.0 to 0.5.0

`procedureKey`, `walkProcedures`, `InputOf`, and `OutputOf` are exported from `@bowline/client`. The framework bindings re-export them, so imports that already went through `@bowline/react-query` keep resolving.

If you wrote a router by hand against a framework other than `net/http`, run the conformance suite against it once:

```go
conformance.Run(t, mount)
```

It is the same suite CI runs for the standard mux, Chi, Gin, Echo, Connect, and Fiber.

## 0.5.0 to 0.6.0

The Dart, Python, Rust, and Elixir targets arrived, along with naming rules shared across every generator. The TypeScript and Go targets already followed those rules, so their output is unchanged; regenerate and diff to confirm.

## 0.6.0 to 0.7.0

Composition, the gateway, the registry, and request signing arrived as separate modules. Nothing in the root module changed for an existing service except two new opt-in handler options:

- `bowline.WithContract(document)` serves `.bowline/contract` and `.bowline/health`. A gateway needs it; a standalone service does not.
- `bowline.Signed(provider)` verifies HMAC signatures before decoding. Adding it rejects every unsigned request, so roll it out to callers first.

A composed client is generated from the composed document rather than from a module:

```bash
bowline gateway compose -o composed.contract.json
bowline gen --from composed.contract.json
```

Procedures move under their service namespace in that client: `invoices.list` becomes `ledger.invoices.list`. A single-service client is unaffected.
