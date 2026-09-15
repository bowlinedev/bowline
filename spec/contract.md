# Bowline contract document, version 1.2

`bowline.contract.json` is the language-neutral description of an API produced by `bowline gen`. Every generator, export, and tool reads this file and nothing else. It is a public specification; third parties may produce or consume it.

The machine-readable schema is `contract.schema.json` in this directory. A worked example is `examples/users.contract.json`.

## Top level

| Field | Meaning |
|---|---|
| `bowline` | Document version, `major.minor`. Readers accept any document with the same major and ignore unknown fields. |
| `hash` | `sha256:` plus the hex digest of the canonical serialization with `hash` and `positions` removed. |
| `types` | Map from canonical type ID to a type declaration. Keys are sorted. |
| `errors` | Map from canonical Go type name to a declared error variant. Always present, empty when no procedure declares variants. |
| `procedures` | List of procedures sorted by `path`. |
| `positions` | Optional map from type ID or procedure path to source file and line. Excluded from the hash. |

Canonical type IDs are the Go import path and type name, for example `github.com/acme/app/users.User`.

## Type declarations

`kind` is one of `struct`, `enum`, `generic`, or `primitive`.

- `struct`: `fields` in declaration order.
- `enum`: `base` is `string` or an integer primitive; `values` carry the Go constant name and its value.
- `generic`: `params` lists type parameter names; `body` is a type node that may contain `param` nodes.
- `primitive`: a named Go type with no constants; `primitive` names the underlying primitive.

A `struct` declaration produced by monomorphizing a generic instantiation carries `origin`, the ID of the generic declaration it was expanded from. Generators use it to know the name was synthesized.

## Type nodes

`kind` is one of `primitive`, `ref`, `array`, `map`, `struct`, `enum`, `generic`, `param`.

- `primitive`: `name` is one of `string`, `bool`, `int8`, `int16`, `int32`, `int64`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`, `timestamp`, `duration`, `bytes`, `raw`. `encoding: "string"` marks a 64-bit integer carried as a JSON string.
- `ref`: `id` names a declaration; `args` instantiates a generic.
- `array`: `elem`; `length` is set for fixed-size Go arrays.
- `map`: `key` and `value`. Keys are always strings on the wire; `key` records the Go key primitive.
- `struct`: an inline anonymous struct with `fields`.
- `param`: a type parameter reference inside a generic body.

Any type node may carry `nullable: true`, meaning the value at that position may be JSON `null`. It is set for pointer element types such as `[]*T` and `map[string]*T`. Pointer struct fields set `nullable` on the field instead.

## Fields

| Field | Meaning |
|---|---|
| `name` | JSON name |
| `type` | Type node |
| `optional` | May be absent from the wire. Set for `omitempty` and `omitzero`. |
| `nullable` | May be JSON `null`. Set for pointers without `omitempty`. |
| `rules` | Validation rules: `rule` and optional `param`, using the go-playground vocabulary subset. |
| `doc` | Doc comment |
| `example` | A sample wire value from the `example` struct tag, typed by the field's node: strings verbatim, numbers and booleans parsed, timestamps in RFC 3339, enum members only, and JSON for everything else. Mock servers and playgrounds prefer it over generated data. |

## Error variants

An entry in `errors` describes a typed error a procedure may return. `name` is the Go type name, `code` is one of the sixteen error codes and fixes the HTTP status, `fields` are the exported fields that travel under `details` on the wire, and `doc` is the type's doc comment. A procedure lists the keys of its variants in `errors`. A variant used by several procedures is declared once.

## Procedures

| Field | Meaning |
|---|---|
| `path` | Dotted path, mount names then the procedure name |
| `kind` | `query`, `mutation`, `subscription`, or `upload` |
| `method` | `GET` or `POST`; subscriptions follow query rules, uploads are always `POST` |
| `input`, `output` | Type nodes, normally `ref` |
| `goInput`, `goOutput` | Canonical Go names of the input and output types: full import path, a dot, the type name, generic arguments in square brackets spelled the same way, and `struct{}` for the empty struct. Used by the runtime to verify the committed document against the running router. |
| `errors` | Keys into the top-level `errors` map for the variants this procedure declares |
| `idempotent` | `true` when the mutation honors an `Idempotency-Key` header |
| `tool` | Present when the procedure is exposed as an agent tool. Carries `scopes`, `readOnly`, and `destructive`. Absent means never exposed. |
| `schemas` | Optional precomputed JSON Schemas, `input` and `output`, filled by `bowline gen` when `"schemas": true` is set. Servers that expose tools read them from here so the runtime needs no schema generator. |
| `doc` | Description |
| `deprecated` | Reason string when the procedure is deprecated |
| `meta` | Free-form string metadata declared in Go |

## Tools

A procedure with a `tool` object may be offered to language models as a callable tool. `scopes` are free-form names a deployment filters on; `readOnly` is set for queries and tells a model the call has no side effects; `destructive` marks a mutation that removes or irreversibly changes data. Subscriptions and uploads are never tools. The JSON Schemas a tool needs are derived from the procedure's input and output types by the CLI, so the runtime stays dependency-free.

## Wire encodings

- `timestamp`: RFC 3339 string.
- `duration`: integer nanoseconds.
- `bytes`: base64 string.
- `raw`: any JSON value, declared untyped by the author.
- 64-bit integers without `encoding: "string"` are JSON numbers within ±(2^53 - 1).

## Determinism

Object keys are sorted, procedures are sorted by path, struct fields and enum values keep declaration order, indentation is two spaces, and the file ends with a newline. Two documents describing the same API are byte-identical.

## Versioning

Additive changes increment the minor version. Removing or renaming a field increments the major version and ships with a migration command. Version 1.0 is the first frozen format, 1.1 added `tool` and `schemas` on procedures, and 1.2 added `example` on fields; `bowline migrate-contract` rewrites a 0.x document, and readers reject 0.x documents with a message naming that command.
