# Bowline documentation

- `quickstart.md`: from an empty directory to a typed TypeScript call in five minutes.
- `guides/errors.md`: the sixteen error codes, returning errors from Go, redaction, and `BowlineError` on the client.
- `guides/validation.md`: the eight validation rules, nested values, and how issues reach the client.
- `guides/fidelity.md`: how Go types become TypeScript types, the 64-bit rule, dates, `WireAs`, and what is rejected.
- `guides/subscriptions.md`: streaming values over server-sent events or a multiplexed WebSocket.
- `guides/uploads.md`: typed multipart uploads.
- `guides/idempotency.md`: idempotency keys, replay, and stores.
- `guides/contract.md`: the contract document, the semantic diff, and the breaking-change gate.
- `guides/openapi.md`: exporting OpenAPI 3.1 from the contract.
- `adoption-test.md`: the ten-minute adoption protocol run before each minor release.
- `../spec/contract.md`: the contract document format that every generator reads.
- `../spec/mapping-table.md`: the normative Go to contract to TypeScript mapping.
- `../spec/diff.md`: the semantic diff rules.
