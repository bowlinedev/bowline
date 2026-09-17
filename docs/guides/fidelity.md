# Type fidelity

Bowline does not emit `any`. Every Go type that reaches a procedure is either mapped exactly or rejected with a diagnostic that names the field and suggests a fix. The full mapping is in `spec/mapping-table.md`. This page covers the cases you will run into in normal use.

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

Struct fields with `omitempty` or `omitzero` become optional. Embedded structs are flattened the same way `encoding/json` flattens them.

## The 64-bit rule

JSON numbers are doubles in JavaScript, so integers larger than 2^53 lose precision. An `int64` field maps to `number` in TypeScript, and the server refuses to encode a value outside the safe range. That is reported as `INTERNAL` and logged with the field path, because by definition it is a server bug. If a field needs the full 64-bit range, tag it with `json:",string"`. It is then sent as a decimal string and the client converts it to a `bigint`:

source: cmd/bowline/internal/analyzer/testdata/fidelity/rows/stdlib/api.go:17-18

```go
	Big      int64           `json:"big,string"`
	Unsigned uint64          `json:"unsigned,string"`
```

## Dates

`time.Time` is sent as RFC 3339. The generated client knows which fields are timestamps and converts them to `Date` after parsing, so `invoice.createdAt.toLocaleDateString()` compiles and works. Inputs work in the other direction automatically because `Date` serializes itself.

## Types with custom marshaling

Go cannot tell the analyzer what a `MarshalJSON` method produces. A type with one is rejected unless its wire shape has been declared:

source: examples/ledger/ledger/money.go:12-17

```go
type Money struct {
	Cents    int64
	Currency string
}

var _ = bowline.WireAs[Money, string]()
```

Here is the marshalling that the analyzer cannot read:

source: examples/ledger/ledger/money.go:29-31

```go
func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}
```

After the declaration, `Money` appears as `string` in every client. The wire type can be any supported type, including a struct or an array. This is the only escape hatch, and it is typed. Types that implement `encoding.TextMarshaler` are mapped to `string` without needing a declaration.

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

Each diagnostic includes `file:line:col`, the Go path such as `User.Meta`, the message, and the fix. `bowline gen` reports all of them in one run.

## Queries and sensitive inputs

Queries are `GET` requests with the input JSON in the `input` query parameter. This makes them cacheable, but it also puts the input in URLs and access logs. Mark a query with `bowline.Sensitive()` to make it use `POST` instead. The ledger's `customers.search` in `examples/ledger/api/customers.go` does this.

## Reserved paths

A handler built with `bowline.WithContract(document)` serves two extra paths under its mount, next to the procedures:

| Path | Response |
|---|---|
| `GET .bowline/contract` | the contract document, byte for byte as it was passed in |
| `GET .bowline/health` | `{"ok":true,"hash":"sha256:…"}` with the document's hash |

Both respond with `Cache-Control: no-store`. Any method other than `GET` gets a 405 with `Allow: GET`. Without the option, both paths return `UNIMPLEMENTED` like any unknown procedure. If the document does not parse, `Handler()` panics when building the handler, so a broken contract does not reach production silently.

The gateway uses both paths. It pins each upstream to a contract hash and refuses to start when the live hash is different, and its readiness probe reports each upstream separately. The ledger passes `api.Contract`, the same bytes that `Router.Verify` checks at startup, so the served document and the compiled router always agree.

source: examples/ledger/cmd/server/main.go:67-67

```go
		bowline.WithContract(api.Contract),
```

Procedure names cannot contain a slash, so the reserved paths cannot collide with a procedure.

## Signed service-to-service calls

`bowline.Signed(provider)` requires every request to the handler to carry a `Bowline-Signature` header. The signature is verified before the input is decoded and before any procedure runs.

source: examples/federation/billing/cmd/server/main.go:27-29

```go
	if secret := os.Getenv("BILLING_INBOUND_SECRET"); secret != "" {
		options = append(options, bowline.Signed(signing.StaticSecrets{os.Getenv("BILLING_INBOUND_KEY"): []byte(secret)}))
	}
```

The header format is `v1,t=<unix seconds>,kid=<key id>,sig=<base64 HMAC-SHA256>`. The signed message is four newline-terminated lines: the method, the request target including its query string, the lowercase hex SHA-256 of the body (or of the empty string for `GET`), and the timestamp. A call fails with `UNAUTHENTICATED` when the header is missing or malformed, when the timestamp is more than 300 seconds off from server time, when the key ID does not resolve, or when the HMAC does not match. The response does not say which of these happened. The reason is logged instead.

Callers sign requests with `signing.Transport`, which is an `http.RoundTripper`. The generated Go client accepts it without any change to the generator:

source: examples/federation/billing/api/ledger.go:10-17

```go
func LedgerClient(url, keyID string, secret []byte) *ledgerclient.Client {
	if keyID == "" || len(secret) == 0 {
		return ledgerclient.New(url)
	}
	return ledgerclient.New(url, ledgerclient.WithHTTPClient(&http.Client{
		Transport: &signing.Transport{KeyID: keyID, Secret: secret},
	}))
}
```

The signature covers the request target as it appears on the wire, so a handler mounted behind `http.StripPrefix` still verifies correctly. Signing applies to every path the handler serves, including the reserved ones, so a probe of a signed handler must also be signed. Each signature carries a random nonce, and the handler remembers the ones it has seen, so a captured request cannot be replayed within the 300-second window. `docs/guides/signing.md` describes the canonical string and the cache.
