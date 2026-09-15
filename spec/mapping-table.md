# Go to contract to client mapping

This table is normative. The analyzer implements exactly these rows, the fidelity golden suite has one fixture package per row, and rows marked reject produce a diagnostic with `file:line:col`, the Go path to the offending field, and a fix suggestion. A construct not in this table is rejected.

| Go | Contract | TypeScript | Dart | Python | Rust | Elixir |
|---|---|---|---|---|---|---|
| `string`, `bool` | primitive | `string`, `boolean` | `String`, `bool` | `str`, `bool` | `String`, `bool` | `String.t()`, `boolean()` |
| `int`, `int64` | `int64` | `number` | `int` | `int` | `i64` | `integer()` |
| `uint`, `uint64` | `uint64` | `number` | `int` | `int` | `u64` | `integer()` |
| `int64` with `json:",string"` | `int64`, encoding string | `bigint` | `BigInt` | `BigInt` | `i64` with `codec::string_int` | `integer()` from a decimal string |
| `int8` to `int32`, `uint8` to `uint32` | width kept | `number` | `int` | `int` with bounds | `i8` to `u32` | `integer()` |
| `float32`, `float64` | width kept | `number` | `double` | `float` | `f32`, `f64` | `float()` |
| `time.Time` | `timestamp` | `Date` | `DateTime` in UTC | aware `datetime` | `chrono::DateTime<Utc>` | `DateTime.t()` in UTC |
| `time.Duration` | `duration` | `number` branded as nanoseconds | `int` as `DurationNs` | `DurationNs` | `DurationNs` | `integer()` nanoseconds |
| `[]byte` | `bytes` | `string` branded as base64 | `Uint8List` via base64 | `Base64Bytes` | `Vec<u8>` with `codec::base64` | `binary()` via base64 |
| `json.RawMessage` | `raw` | `unknown` | `Object?` | `JsonValue` | `serde_json::Value` | `term()` |
| `[]T` | array | `T[]` | `List<T>` | `list[T]` | `Vec<T>` | `[T]` |
| `[N]T` | array with length | `T[]` | `List<T>` with a `len` rule | `list[T]` with `min_length` and `max_length` | `[T; N]` | `[T]` with a `len` rule |
| `map[K]V`, K string, integer, or `TextMarshaler` | map | `Record<string, V>` | `Map<String, V>` | `dict[str, V]` | `BTreeMap<String, V>` | `%{String.t() => V}` |
| `*T` field | field nullable | `T \| null` | `T?` | `T \| None` | `Option<T>` | `T \| nil` |
| `*T` field with `omitempty` | field optional | `?: T` | `T?`, omitted when null | `T \| None = None` | `Option<T>` with `skip_serializing_if` | `T \| nil`, omitted when nil |
| `[]*T`, `map[K]*T` | element node nullable | `(T \| null)[]`, `Record<string, T \| null>` | `List<T?>`, `Map<String, T?>` | `list[T \| None]`, `dict[str, T \| None]` | `Vec<Option<T>>`, `BTreeMap<String, Option<T>>` | `[T \| nil]`, `%{String.t() => T \| nil}` |
| named slice, array, or map type | the underlying shape, no declaration | the underlying TypeScript type | the underlying type | the underlying type | the underlying type | the underlying type |
| `T` with `omitempty` or `omitzero` | optional | `?: T` | `T?` | `T \| None = None` | `Option<T>` | `T \| nil` |
| named struct | struct | `interface` | `class` with `fromJson`, `toJson`, `validate` | `BaseModel` | `struct` with serde derives and `impl Validate` | module with `defstruct`, `from_map`, `to_map`, `validate` |
| anonymous struct field | inline struct | inline object type | synthesized `Parent_Field` class | synthesized `Parent_Field` model | synthesized `Parent_Field` struct | nested `Parent.Field` module |
| embedded struct | flattened, as encoding/json does | flattened | flattened | flattened | flattened | flattened |
| `json:"-"`, unexported field | skipped | skipped | skipped | skipped | skipped | skipped |
| named string or integer type with typed constants in its package | enum | literal union | enhanced `enum` with `value` | `str, Enum` or `IntEnum` | `enum` with `serde(rename)` or generated integer impls | atoms with `from_value`/`to_value`, or integer literals |
| named string type without constants | primitive with `nominal` name | `string` | `typedef` | `NewType` | `pub type` | `@type` alias |
| generic type | generic | generic | `class Page<T>` with converter arguments | `Generic[T]` model | `struct Page<T>` | module with decoder and encoder function arguments |
| generic type with shape-changing constraint | monomorphized instantiations | concrete types | concrete classes | concrete models | concrete structs | concrete modules |
| recursive type | ref | recursive interface | direct references | forward references with `model_rebuild()` | `Box<T>` where the cycle would be infinite-sized | direct module references |
| type alias | resolved | resolved | resolved | resolved | resolved | resolved |
| `encoding.TextMarshaler` type | `string` | `string` | `String` | `str` | `String` | `String.t()` |
| `json.Marshaler` type without `WireAs` | reject | | | | |  |
| `json.Marshaler` type with `bowline.WireAs[T, W]()` | shape of W | shape of W | shape of W | shape of W | shape of W | shape of W |
| `any`, interface, `map[string]any` | reject, pointing at unions in M2 or `json.RawMessage` | | | | |  |
| `map[K]V` with struct key | reject | | | | |  |
| `**T`, `chan`, `func`, `complex`, `unsafe.Pointer`, `uintptr` | reject | | | | |  |
| anonymous struct as `In` or `Out` | reject, except `struct{}` | | `Empty` | `Empty` | `Empty` | `BowlineClient.Empty` |

`WireAs[T, W]()` returns an empty `Wire` value and has no runtime effect, so it is written as a package-level declaration: `var _ = bowline.WireAs[Money, string]()`. The analyzer finds every call anywhere in the package graph and records that `T` serializes as `W`. It is the single escape hatch for types whose wire shape Go cannot expose, and it is typed, so it is never `any`.

The Dart, Python, and Rust columns follow the same rules as the generators under `cmd/bowline/internal/gen/{dart,python,rust}`; the server guarantees that a 64-bit integer without `,string` fits in 53 bits, so plain integers are native in every language and only the string encoding needs an arbitrary-precision type. Field names keep the JSON name on the wire: Dart uses `lowerCamel`, Elixir uses `snake_case` atoms fields with the JSON key in `fromJson` and `toJson`, Rust uses `snake_case` with `#[serde(rename)]`, and Python keeps the JSON name unless it is not an identifier, in which case a Pydantic alias carries it.

TypeScript names: the Go type name. When two types in the contract share a name, both are emitted as `PackageName_TypeName`, where the package name is the last path element, capitalized. Field names are the JSON names.
