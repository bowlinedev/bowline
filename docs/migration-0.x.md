# Migrating across 0.x

The 0.x releases were additive. Nothing that was exported in 0.1.0 has been renamed or removed since, and a client generated against an older contract still works against a newer server. This page lists what each release requires from you, in order. You can skip the sections for releases you are already past.

See `docs/lts.md` for release cadence, support windows, and the backport policy.

## 0.1.0 to 0.2.0

The contract document format was frozen at 1.0 in this release. Documents written by the 0.1.0 alpha use an earlier shape and need to be converted once:

```bash
bowline migrate-contract api/bowline.contract.json
```

Commit the converted document. After this step, later releases only require regenerating.

Subscriptions, uploads, declared error variants, and idempotency keys were added in 0.2.0. They are all opt-in. A router that does not use them generates the same client it did before.

## 0.2.0 to 0.3.0

The format moved to 1.1 so it could carry the `tool` block and optional embedded JSON Schemas. These fields are only written when you ask for them, so regenerating is enough:

```bash
bowline gen
```

Set `"schemas": true` in `bowline.json` if you want the schemas inline. Tool metadata is included when you mark a procedure with `bowline.Tool`, `bowline.Scope`, or `bowline.Destructive`.

## 0.3.0 to 0.4.0

The format moved to 1.2 to add an `example` field on struct fields, filled from the `example` struct tag. Regenerate. Contracts that do not use the tag are unchanged.

`bowline verify-consumers` and the `record` client option were added here. Neither does anything unless you turn it on.

## 0.4.0 to 0.5.0

`procedureKey`, `walkProcedures`, `InputOf`, and `OutputOf` are now exported from `@bowlinedev/client`. The framework bindings re-export them, so existing imports through `@bowlinedev/react-query` continue to resolve.

If you wrote a router by hand for a framework other than `net/http`, run the conformance suite against it once:

source: examples/routers/chi/conformance_test.go:11-13

```go
func TestConformance(t *testing.T) {
	conformance.Run(t, Mount)
}
```

This is the same suite that CI runs for the standard mux, Chi, Gin, Echo, Connect, and Fiber.

## 0.5.0 to 0.6.0

The Dart, Python, Rust, and Elixir targets were added, along with a shared set of naming rules used by every generator. The TypeScript and Go targets already followed those rules, so their output did not change. Regenerate and diff to confirm.

## 0.6.0 to 0.7.0

Composition, the gateway, the registry, and request signing were added as separate modules. For an existing single service nothing in the root module changed, apart from two new opt-in handler options:

- `bowline.WithContract(document)` serves `.bowline/contract` and `.bowline/health`. A gateway needs this. A standalone service does not.
- `bowline.Signed(provider)` verifies HMAC signatures before decoding. Once added it rejects every unsigned request, so update the callers first.

A composed client is generated from the composed document instead of from a module:

```bash
bowline gateway compose -o composed.contract.json
bowline gen --from composed.contract.json
```

In the composed client, procedures are placed under their service name. `invoices.list` becomes `ledger.invoices.list`. A single-service client is not affected.
