# Go to contract to TypeScript mapping

This table is normative. The analyzer implements exactly these rows, the fidelity golden suite has one fixture package per row, and rows marked reject produce a diagnostic with `file:line:col`, the Go path to the offending field, and a fix suggestion. A construct not in this table is rejected.

| Go | Contract | TypeScript |
|---|---|---|
| `string`, `bool` | primitive | `string`, `boolean` |
| `int`, `int64` | `int64` | `number` |
| `uint`, `uint64` | `uint64` | `number` |
| `int64` with `json:",string"` | `int64`, encoding string | `bigint` |
| `int8` to `int32`, `uint8` to `uint32` | width kept | `number` |
| `float32`, `float64` | width kept | `number` |
| `time.Time` | `timestamp` | `Date` |
| `time.Duration` | `duration` | `number` branded as nanoseconds |
| `[]byte` | `bytes` | `string` branded as base64 |
| `json.RawMessage` | `raw` | `unknown` |
| `[]T` | array | `T[]` |
| `[N]T` | array with length | `T[]` |
| `map[K]V`, K string, integer, or `TextMarshaler` | map | `Record<string, V>` |
| `*T` field | field nullable | `T \| null` |
| `*T` field with `omitempty` | field optional | `?: T` |
| `[]*T`, `map[K]*T` | element node nullable | `(T \| null)[]`, `Record<string, T \| null>` |
| named slice, array, or map type | the underlying shape, no declaration | the underlying TypeScript type |
| `T` with `omitempty` or `omitzero` | optional | `?: T` |
| named struct | struct | `interface` |
| anonymous struct field | inline struct | inline object type |
| embedded struct | flattened, as encoding/json does | flattened |
| `json:"-"`, unexported field | skipped | skipped |
| named string or integer type with typed constants in its package | enum | literal union |
| named string type without constants | primitive with `nominal` name | `string` |
| generic type | generic | generic |
| generic type with shape-changing constraint | monomorphized instantiations | concrete types |
| recursive type | ref | recursive interface |
| type alias | resolved | resolved |
| `encoding.TextMarshaler` type | `string` | `string` |
| `json.Marshaler` type without `WireAs` | reject | |
| `json.Marshaler` type with `bowline.WireAs[T, W]()` | shape of W | shape of W |
| `any`, interface, `map[string]any` | reject, pointing at unions in M2 or `json.RawMessage` | |
| `map[K]V` with struct key | reject | |
| `**T`, `chan`, `func`, `complex`, `unsafe.Pointer`, `uintptr` | reject | |
| anonymous struct as `In` or `Out` | reject, except `struct{}` | |

`WireAs[T, W]()` returns an empty `Wire` value and has no runtime effect, so it is written as a package-level declaration: `var _ = bowline.WireAs[Money, string]()`. The analyzer finds every call anywhere in the package graph and records that `T` serializes as `W`. It is the single escape hatch for types whose wire shape Go cannot expose, and it is typed, so it is never `any`.

TypeScript names: the Go type name. When two types in the contract share a name, both are emitted as `PackageName_TypeName`, where the package name is the last path element, capitalized. Field names are the JSON names.
