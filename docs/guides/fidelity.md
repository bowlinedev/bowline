# Type fidelity

Bowline never emits `any`. Every Go type that reaches a procedure is either mapped exactly or rejected with a diagnostic that names the field and suggests a fix. The full mapping is in `spec/mapping-table.md`; this page covers what you meet day to day.

## The common cases

| Go | TypeScript |
|---|---|
| `string`, `bool` | `string`, `boolean` |
| `int`, `int32`, `int64`, `float64` | `number` |
| `int64` tagged `json:",string"` | `bigint` |
| `time.Time` | `Date` |
| `[]T` | `T[]`, never `null` |
| `map[string]T` | `Record<string, T>` |
| `*T` field | `T \| null` |
| `*T` field with `omitempty` | `T?` |
| named string type with constants | a literal union such as `"draft" \| "sent"` |
| `Page[T]` | `Page<T>` |
| `json.RawMessage` | `unknown` |

Struct fields with `omitempty` or `omitzero` become optional. Embedded structs are flattened exactly as `encoding/json` flattens them.

## The 64-bit rule

JSON numbers are doubles in JavaScript, so integers beyond 2^53 lose precision. An `int64` field is a `number` in TypeScript, and the server refuses to encode a value outside the safe range; that is reported as `INTERNAL` and logged with the field path, because it is a server bug by definition. A field that needs the full range is tagged `json:",string"`; it travels as a decimal string and the client hydrates it into a `bigint`:

```go
type Event struct {
	Sequence uint64 `json:"sequence,string"`
}
```

## Dates

`time.Time` is RFC 3339 on the wire. The generated client knows which fields are timestamps and converts them to `Date` after parsing, so `invoice.createdAt.toLocaleDateString()` compiles and works. Inputs go the other way automatically because `Date` serializes itself.

## Types with custom marshaling

Go cannot tell the analyzer what a `MarshalJSON` method produces, so such a type is rejected unless its wire shape is declared once:

```go
type Money struct {
	Cents    int64
	Currency string
}

func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

var _ = bowline.WireAs[Money, string]()
```

After the declaration `Money` appears as `string` in every client. The wire type can be any supported type, including a struct or an array. This is the only escape hatch and it is typed. Types that implement `encoding.TextMarshaler` are mapped to `string` without a declaration.

## What is rejected

| Construct | Diagnostic | Fix |
|---|---|---|
| `any`, interfaces, `map[string]any` | interfaces are not supported | use a struct, or `json.RawMessage` for an untyped payload |
| `**T` | pointer to pointer is not supported | use a single pointer |
| `map` with a struct key | map key type is not supported | use a string, integer, or `TextMarshaler` key |
| `chan`, `func`, `complex`, `uintptr` | not supported | use a struct, slice, map, or primitive |
| `json.Marshaler` without `WireAs` | wire shape cannot be inferred | add a `WireAs` declaration |
| an anonymous struct as input or output | must be a named type | declare a named struct |
| a validation term outside the eight rules | unsupported validation rule | see the validation guide |

Diagnostics come with `file:line:col`, the Go path such as `User.Meta`, the message, and the fix, and `bowline gen` reports all of them at once.

## Queries and sensitive inputs

Queries are `GET` requests with the input JSON in the `input` query parameter, which makes them cacheable but also puts the input in URLs and access logs. Mark a query `bowline.Sensitive()` to force `POST`; the ledger's `customers.search` in `examples/ledger/api/customers.go` does this.

## Reserved paths

A handler built with `bowline.WithContract(document)` serves two paths under its mount beside the procedures:

| Path | Response |
|---|---|
| `GET .bowline/contract` | the contract document, byte for byte as it was passed in |
| `GET .bowline/health` | `{"ok":true,"hash":"sha256:…"}` with the document's hash |

Both answer `Cache-Control: no-store`, and any method other than `GET` is 405 with `Allow: GET`. Without the option both paths are `UNIMPLEMENTED`, like any unknown procedure. A document that does not parse panics when `Handler()` builds the handler, so a broken contract never reaches production silently.

The gateway uses both: it pins every upstream to a contract hash and refuses to start when the live hash differs, and its readiness probe reports each upstream separately. The ledger passes `api.Contract`, the same bytes `Router.Verify` checks at startup, so the served document and the compiled router can never disagree.

```go
r.Mount("/api", routes.Handler(bowline.WithContract(api.Contract)))
```

Procedure names cannot contain a slash, so the reserved paths can never shadow a procedure.
