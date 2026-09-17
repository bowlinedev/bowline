# Validation

Input validation is driven by `validate` struct tags. The rules are enforced on the server before the handler runs, recorded in the contract, and reported to clients as a list of `issues`, each with the path of the failing field.

## Rules

| Rule | Applies to | Meaning |
|---|---|---|
| `required` | any field | not the zero value; a non-empty collection |
| `min=N` | strings, numbers, collections | at least N characters, at least N, at least N items |
| `max=N` | strings, numbers, collections | at most N characters, at most N, at most N items |
| `len=N` | strings, numbers, collections | exactly N |
| `oneof=a b c` | strings, integers | one of the listed values |
| `email` | strings | a valid email address |
| `url` | strings | a URL with scheme and host |
| `uuid` | strings | an RFC 4122 UUID |

The syntax follows the go-playground `validate` vocabulary, but only these eight terms are supported. Any other term causes `bowline gen` to fail with a diagnostic, and also makes `bowline.NewRouter` panic. This means that if the application starts, its contract can be generated.

From `examples/ledger/ledger/types.go`:

source: examples/ledger/ledger/types.go:19-23

```go
type Line struct {
	Description string `json:"description" validate:"required,max=200" example:"Consulting"`
	Quantity    int32  `json:"quantity" validate:"min=1" example:"10"`
	UnitPrice   Money  `json:"unitPrice" example:"USD 150.00"`
}
```

## Examples

An `example` tag next to the rules provides a sample wire value. The analyzer checks it against the field's type: strings are taken as is, numbers and booleans are parsed, timestamps must be RFC 3339, enums must be a valid member, and structs and collections must be JSON. Examples are stored in the contract, emitted into the JSON Schemas for tools, used as initial form values in the playground, and preferred over generated data by the mock server. A bad example produces a diagnostic from `bowline gen`.

## Nested values

Nested structs, slices, and maps are always validated. There is no `dive` term. Paths in issues use JSON names and indices, so a problem with the second line item is reported as `lines.1.quantity`.

## What the client sees

A failing call returns `INVALID_ARGUMENT` with one issue for each broken rule:

```json
{"error":{"code":"INVALID_ARGUMENT","message":"invalid input","issues":[
  {"path":["lines","0","description"],"rule":"required","message":"is required"},
  {"path":["lines","0","quantity"],"rule":"min","message":"must be at least 1"}
]}}
```

The ledger web app renders these like so:

source: examples/ledger/web/src/invoices.tsx:82-90

```tsx
      {create.error instanceof BowlineError && (
        <ul role="alert" data-testid="issues">
          {create.error.issues.map((issue) => (
            <li key={issue.path.join(".")}>
              {issue.path.join(".")}: {issue.message}
            </li>
          ))}
        </ul>
      )}
```

## Cost

Rules are compiled once per procedure when the router is built. Checking a valid input is a single pass with no allocations. Only an invalid input pays for building the issue list. The overhead benchmark in `handler_bench_test.go` includes a `required` rule and stays within the 5 percent budget compared to a hand-written handler.

## Zod schemas

Setting `"zod": true` on the TypeScript target writes a `bowline.zod.ts` file next to the client. It exports `schemas` (one Zod schema per declared type), `inputs` (keyed by procedure path), and `errors` (keyed by variant name). These carry the same rules the server enforces, so a form can validate before sending the request, and the client and server can never disagree.

```json
{ "entry": "./api.Routes", "targets": { "ts": { "out": "web/src/bowline.ts", "zod": true } } }
```

sketch: how an application consumes the generated `inputs` map; no app in this repository parses client-side yet

```ts
import { inputs } from "./bowline.zod.js";

const parsed = inputs["invoices.create"].safeParse(form);
if (!parsed.success) {
  showIssues(parsed.error.issues);
}
```

Generic types become functions of their element schemas, for example `schemas.Page(schemas.Invoice)`. Recursive types use property getters, which is how Zod 4 handles a self-reference without a type annotation. The test `packages/client/src/zod.test.ts` shows an email rule rejecting an invalid value through a generated schema.
