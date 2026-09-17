# OpenAPI export

The contract is the source of truth. An OpenAPI 3.1 document is one projection of it. `bowline export openapi` regenerates the document from the Go code, so it is never edited by hand and cannot drift.

```
bowline export openapi -o api/openapi.json
```

Without `-o` the document is written next to the contract as `openapi.json`. The title, version, and server URL come from an optional block in `bowline.json`:

```json
{
  "entry": "./api.Routes",
  "contract": "api/bowline.contract.json",
  "openapi": { "title": "Ledger", "version": "0.2.0", "serverUrl": "http://localhost:8080/api" }
}
```

## Mapping

- A query becomes `GET /{path}` with a required `input` query parameter whose content type is `application/json`. The parameter is optional if the input is the empty struct.
- A sensitive query and a mutation become `POST /{path}` with a JSON request body.
- A subscription becomes `GET /{path}` whose `200` response is `text/event-stream` with the message schema. The description explains the `message`, `error`, and `done` events.
- An upload becomes `POST /{path}` with a `multipart/form-data` body containing an `input` part encoded as JSON and a binary `file` part.
- Every struct, enum, and named primitive declaration becomes an entry under `components/schemas`, using JSON Schema 2020-12 with `additionalProperties: false`. If two declarations share a name, both are prefixed with their package name. This is the same rule the TypeScript generator uses.
- A generic instantiation becomes one schema per instantiation, named by joining the generic and its arguments with underscores, for example `Page_User`.
- Optional fields are left out of `required`. Nullable fields and elements get `null` added to their type, or a `oneOf` with `null` when they reference another schema.
- Validation rules map to `minLength` and `maxLength` on strings, `minimum` and `maximum` on numbers, `minItems` and `maxItems` on collections, `enum` for `oneof`, and the `format` values `email`, `uri`, and `uuid`.
- Integers narrower than 64 bits are `integer` with `format: int32`. `int64` and `uint64` are `integer` with `format: int64`, or `string` with `format: int64` when the Go field is tagged `json:",string"`.
- `time.Time` is a `string` with `format: date-time`. `time.Duration` is an `int64` integer described as nanoseconds. `[]byte` is a `string` with `contentEncoding: base64`. `json.RawMessage` is an empty schema that accepts any value.
- Every operation references the `INVALID_ARGUMENT` and `INTERNAL` responses. Each declared error variant becomes a schema for its details, an envelope schema that fixes `type` to the variant name, and a response under `components/responses` referenced from the operations that declare it. Variants that share a status appear as a `oneOf`.

The goldens under `cmd/bowline/internal/export/openapi/testdata` show the output for every fixture in the fidelity suite.
