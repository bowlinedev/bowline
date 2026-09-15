# Mock server

`bowline mock` serves an API from `bowline.contract.json` alone. A frontend team builds and tests against it with the Go backend stopped, and CI runs the browser suite without a database.

## Generated data

```bash runnable
bowline mock --addr 127.0.0.1:18091 --seed 1 --no-playground & sleep 1; curl -fsS 'http://127.0.0.1:18091/api/invoices.get?input=%7B%22id%22%3A3%7D' | grep -q '"id":3'; kill %1
```

Every value is derived from the contract: fields carry their `example` tag when they have one, enums pick a declared value, `email`, `url`, and `uuid` rules produce matching strings, `min`, `max`, and `len` bound strings, numbers, and collections, timestamps land within a month before 2026-01-01, and fields named `id`, `*Id`, or `*ID` count up per type. The stream behind each value is seeded by the seed, the procedure, and the field path, so adding a field elsewhere never changes an existing value and the same seed produces the same bytes on every machine. Reviewers can commit screenshots and fixtures taken from the mock without churn.

Set `example` tags where realism matters. From `examples/ledger/ledger/types.go`:

```go
	Name  string `json:"name" validate:"required" example:"Ada Lovelace"`
	Email string `json:"email" validate:"required,email" example:"ada@example.com"`
```

## State

The mock keeps a small in-memory model so a create-then-list or create-then-get flow looks real. Every object whose type has an `id`-like field is stored under its type. A mutation whose output is a stored type creates a fresh object, copies every input field whose name and kind match an output field, and assigns the next id; a mutation whose input carries only the object's own `id` updates the stored object instead. A query whose input is exactly one `id`-like field returns the stored object, or generates one with that id and stores it. A query returning an array of a stored type, or a struct with one such array, returns the table in insertion order, seeded with five objects on first use. Everything else is generated.

Inputs are validated with the contract rules, so the mock answers the same `INVALID_ARGUMENT` issues the real server would, byte for byte for the same input; `cmd/bowline/internal/mock/handler_test.go` proves that against the runtime. `GET` on a `Sensitive` query is refused, subscriptions and uploads answer `UNIMPLEMENTED`, and deprecated procedures carry the `Deprecation` header.

## Recording and replay

```bash
bowline mock --record http://localhost:8080/api
```

proxies every request to the upstream, serves the real response, and writes one fixture per distinct interaction to `mocks/<procedure>/<sha256 of the canonical input>.json`, where the canonical input has sorted keys so `{"id":3}` and `{ "id": 3 }` share a fixture. Later:

```bash
bowline mock --replay
```

matches by procedure and canonical input and serves the recorded status, headers, and body. On a miss it falls back to generated data with a warning, or with `--strict` answers `UNIMPLEMENTED` and logs the miss, which keeps CI deterministic while local development stays unblocked. `spec/mock.md` describes the fixture file.

## In a browser suite

The ledger's `examples/ledger/web/playwright.mock.config.ts` starts `bowline mock` instead of the Go server; `examples/ledger/web/e2e/mock.spec.ts` runs the list, validate, create, and void flow against it, and the `ledger-mock` job in `.github/workflows/e2e.yml` runs that suite on every push. The playground is served at `/_playground/` on the same address unless `--no-playground` is given.
