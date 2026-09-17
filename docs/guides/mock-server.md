# Mock server

`bowline mock` serves an API using only `bowline.contract.json`. A frontend team can build and test against it with the Go backend stopped, and CI can run the browser suite without a database.

## Generated data

```bash runnable
bowline mock --addr 127.0.0.1:18091 --seed 1 --no-playground & sleep 1; curl -fsS 'http://127.0.0.1:18091/api/invoices.get?input=%7B%22id%22%3A3%7D' | grep -q '"id":3'; kill %1
```

Every value is derived from the contract. Fields use their `example` tag if they have one. Enums pick a declared value. The `email`, `url`, and `uuid` rules produce matching strings. `min`, `max`, and `len` bound strings, numbers, and collections. Timestamps fall within the month before 2026-01-01. Fields named `id`, `*Id`, or `*ID` count up per type. The random stream behind each value is seeded by the seed, the procedure, and the field path, so adding a field somewhere else does not change an existing value, and the same seed produces the same bytes on every machine. This means reviewers can commit screenshots and fixtures taken from the mock without them changing on every run.

Set `example` tags where realism matters. From the ledger:

source: examples/ledger/ledger/types.go:37-38

```go
	Name  string `json:"name" validate:"required" example:"Ada Lovelace"`
	Email string `json:"email" validate:"required,email" example:"ada@example.com"`
```

## State

The mock keeps a small in-memory model so that a create-then-list or create-then-get sequence looks realistic. Every object whose type has an `id`-like field is stored under its type. A mutation whose output is a stored type creates a new object, copies every input field whose name and kind match an output field, and assigns the next id. A mutation whose input only has the object's own `id` updates the stored object instead. A query whose input is exactly one `id`-like field returns the stored object, or generates one with that id and stores it. A query that returns an array of a stored type, or a struct containing one such array, returns the table in insertion order, seeded with five objects on first use. Everything else is generated.

Inputs are validated using the contract rules, so the mock returns the same `INVALID_ARGUMENT` issues the real server would, byte for byte for the same input. `cmd/bowline/internal/mock/handler_test.go` checks this against the runtime. `GET` on a `Sensitive` query is refused. Subscriptions and uploads respond with `UNIMPLEMENTED`. Deprecated procedures carry the `Deprecation` header.

## Recording and replay

```bash
bowline mock --record http://localhost:8080/api
```

This proxies every request to the upstream, serves the real response, and writes one fixture per distinct interaction to `mocks/<procedure>/<sha256 of the canonical input>.json`. The canonical input has sorted keys, so `{"id":3}` and `{ "id": 3 }` share a fixture. Later:

```bash
bowline mock --replay
```

This matches by procedure and canonical input and serves the recorded status, headers, and body. On a miss it falls back to generated data with a warning. With `--strict` it responds with `UNIMPLEMENTED` instead and logs the miss. This keeps CI deterministic while local development stays unblocked. `spec/mock.md` describes the fixture file format.

## In a browser suite

The ledger's `examples/ledger/web/playwright.mock.config.ts` starts `bowline mock` instead of the Go server. `examples/ledger/web/e2e/mock.spec.ts` runs the list, validate, create, and void flow against it, and the `ledger-mock` job in `.github/workflows/e2e.yml` runs that suite on every push. The playground is served at `/_playground/` on the same address unless `--no-playground` is passed.
