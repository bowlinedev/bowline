# Semantic contract diff

`bowline diff <old.json> <new.json> [--format text|markdown|json]` compares two contract documents and lists every change with a path, a category, and one sentence. The command exits 1 when any change is `breaking`, so it can gate a pull request on its own.

Paths name the procedure and descend through `input`, `output`, `error <Name>`, `field <name>`, `elem`, and `value`, for example `procedure invoices.list output field items elem field total`.

## Categories

| Category | Meaning |
|---|---|
| `added` | new capability that no existing client can observe as a change |
| `removed` | reserved for removals that no client depends on |
| `widened` | the server accepts more than before; existing clients keep working |
| `narrowed` | the server produces less than before in a way existing clients tolerate |
| `breaking` | an existing client can fail to compile or fail at runtime |

## Variance

Inputs are contravariant: making an input accept more is safe, requiring more is breaking. Outputs are covariant: producing more is safe for consumers that ignore unknown fields, producing less or different is breaking. Types are compared structurally, following references and generic arguments, so renaming a type without changing its shape produces no change, and a change inside `Page<User>` is reported through every procedure whose input or output reaches it.

## Rules

| Change | In an input | In an output |
|---|---|---|
| procedure removed | breaking | |
| procedure added | added | |
| kind or method changed | breaking | |
| required field added | breaking | added |
| optional field added | widened | added |
| field removed | widened | breaking |
| optional to required | breaking | narrowed |
| required to optional | widened | breaking |
| nullable added | widened | breaking |
| nullable removed | breaking | narrowed |
| primitive or ref type changed | breaking | breaking |
| array length changed | breaking | breaking |
| enum value added | widened | breaking |
| enum value removed | breaking | narrowed |
| enum base changed | breaking | breaking |
| validation rule tightened | breaking | narrowed |
| validation rule loosened | widened | added |
| error variant added | | widened |
| error variant removed | | narrowed |
| error variant code changed | | breaking |
| error variant field changed | | as an output field |
| idempotent removed | breaking | |
| idempotent added | added | |
| deprecated set | added | |

Adding an enum value to an output is breaking because an exhaustive `switch` on the client stops compiling; the gate says so rather than surprising a consumer.

A validation rule tightens when it is added, when `min` grows, when `max` shrinks, when `oneof` loses a value, or when any other rule's parameter changes. It loosens when it is removed, when `min` shrinks, when `max` grows, or when `oneof` gains a value. A `oneof` that both gains and loses values counts as tightened.

## Output formats

Text prints one line per change with the category padded to nine columns. Markdown prints a bold summary line with the total and breaking counts followed by a bullet per change, which is what the CI gate posts as a pull request comment. JSON prints an array of objects with `path`, `category`, and `message`. With no changes, text and markdown print `no contract changes` and JSON prints `[]`.
