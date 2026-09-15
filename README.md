# Bowline

Bowline makes a Go codebase the single source of truth for an API contract and projects that contract, with full type fidelity, into typed clients.

Developers write plain Go functions:

    func getUser(ctx context.Context, in GetInput) (User, error)

Bowline reads the router, writes `bowline.contract.json`, and generates clients from it. The runtime is an `http.Handler` with no dependencies outside the standard library.

Status: pre-alpha. Nothing here is stable yet.

## Layout

- `/` runtime module, `github.com/bowlinedev/bowline`
- `/cmd/bowline` CLI module
- `/contract` contract document types
- `/packages` npm packages
- `/spec` contract specification and JSON Schema

## License

Apache-2.0. See `LICENSE`.
