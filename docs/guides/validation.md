# Validation

Input validation comes from `validate` struct tags. Rules are enforced on the server before the handler runs, recorded in the contract, and delivered to every client as `issues` with the failing path.

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

The syntax is the go-playground `validate` vocabulary, restricted to these eight terms. Any other term fails `bowline gen` with a diagnostic and makes `bowline.NewRouter` panic, so an application that starts is one whose contract can be generated.

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

An `example` tag beside the rules gives a sample wire value that the analyzer checks against the field's type: strings verbatim, numbers and booleans parsed, timestamps in RFC 3339, enum members only, JSON for structs and collections. Examples are carried in the contract, emitted into JSON Schemas for tools, used as initial form values in the playground, and preferred over generated data by the mock server. A bad example is a diagnostic from `bowline gen`.

## Nested values

Nested structs, slices, and maps are always validated; there is no `dive` term. Paths in issues use JSON names and indices, so a bad second line reports `lines.1.quantity`.

## What the client sees

A failing call returns `INVALID_ARGUMENT` with one issue per broken rule:

```json
{"error":{"code":"INVALID_ARGUMENT","message":"invalid input","issues":[
  {"path":["lines","0","description"],"rule":"required","message":"is required"},
  {"path":["lines","0","quantity"],"rule":"min","message":"must be at least 1"}
]}}
```

The ledger web app renders these:

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

Rules are compiled once per procedure when the router is built. A valid input is checked in a single pass with no allocations; only an invalid input pays for building the issue list. The overhead benchmark in `handler_bench_test.go` includes a `required` rule and stays within the 5 percent budget against a hand-written handler.


## Zod schemas

Setting `"zod": true` on the TypeScript target writes `bowline.zod.ts` next to the client file. It exports `schemas` with one Zod schema per declared type, `inputs` keyed by procedure path, and `errors` keyed by variant name, all carrying the same rules the server enforces, so a form can validate before the request leaves the browser and the two can never disagree.

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

Generic types become functions of their element schemas, for example `schemas.Page(schemas.Invoice)`, and recursive types use property getters, which is how Zod 4 defers a self-reference without a type annotation. The test `packages/client/src/zod.test.ts` shows an email rule rejecting an invalid value through a generated schema.
