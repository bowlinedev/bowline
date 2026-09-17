# Public API audit, 2026

Every exported identifier in the three guaranteed packages was classified keep, unexport, or remove before the surface was frozen. `docs/api-freeze.md` is the resulting list; this page is the reasoning behind it.

The surface is 269 identifiers: 135 in `bowline`, 109 in `contract`, 25 in `signing`. Packages under `internal/` were not considered, because `docs/stability.md` excludes them and nothing outside the repository can import them.

## Method

1. `go/doc` over each package's non-test files produced the candidate list, including struct fields and interface methods, since both are part of the promise.
2. Each identifier was classified against one question: would a user of this library reasonably write it in their own code, or is it visible only because it had to be?
3. Anything in the second group was checked for callers across every module in the workspace. An identifier used only by tests in its own package is a leak, because an unexported name would serve those tests equally well.
4. `TestPublicIdentifiersMatchFreezeList` compares the list with the real surface in both directions, so this audit stays in sync with the code.

## Kept

### `bowline`

| Group | Identifiers | Why |
|---|---|---|
| Router construction | `Router`, `NewRouter`, `Mount`, `Item`, `Query`, `Mutation`, `Subscription`, `Upload`, `Router.Handler`, `Router.Procedures`, `Router.Subscribe`, `Router.Use`, `Router.Verify` | The expression language every service is written in. |
| Procedure options | `ProcOption`, `Description`, `Deprecated`, `Sensitive`, `Meta`, `Use`, `Errors`, `Idempotent`, `MaxBody`, `Tool`, `Scope`, `Destructive`, `ToolOption` | Written by hand at every declaration site. |
| Procedure model | `Procedure` and its fields, `ProcedureKind`, `KindQuery`, `KindMutation`, `KindSubscription`, `KindUpload`, `Procedure.Method` | Returned by `Router.Procedures`; the analyzer, the MCP server, and the agent SDKs all read it across module boundaries. |
| Handler options | `HandlerOption`, `MaxBodySize`, `MaxUploadSize`, `Logger`, `Production`, `StrictInput`, `CSRF`, `CSRFOptions`, `SecurityHeaders`, `WithContract`, `Signed` | Passed to `Handler()` by every deployment. |
| Middleware | `Middleware`, `Next`, `Call`, `Call.Procedure`, `Call.Request`, `Call.ResponseHeader`, `CallFrom`, `RateLimit`, `RateLimitOptions` | The middleware contract; `ResponseHeader` is how a middleware sets `Retry-After`. |
| Errors | `Code` and its sixteen constants, `Code.HTTPStatus`, `Error` and its fields, `Error.Error`, `Error.Unwrap`, `Error.WithDetails`, `Errorf`, `Issue`, `Coded` | `Coded` is implemented by the user on each declared error variant and passed to `Errors`. |
| Streaming and uploads | `Stream`, `Stream.Send`, `StreamFailure` and its fields, `Heartbeat`, `File` | Appear in the signature of every subscription and upload procedure. |
| Idempotency | `IdempotencyStore` and its methods, `IdempotencyState`, `IdempotencyNew`, `IdempotencyStored`, `IdempotencyInFlight`, `Idempotency`, `MemoryIdempotencyStore`, `RequireIdempotencyKey`, `WithIdempotencyScope` | A user supplies their own store, so the interface and its state constants are implemented outside this repository. |
| Wire mapping | `Wire`, `WireAs` | `WireAs` is written as `var _ = bowline.WireAs[Money, string]()`; `Wire` is its return type and cannot be unexported without unexporting the function. |
| Version | `Version` | Read by servers that report their build. |

### `contract`

Everything in this package is kept. The contract document is a published format: `Document`, `TypeDecl`, `Type`, `Field`, `Rule`, `EnumValue`, `Procedure`, `ErrorDecl`, `Schemas`, `Tool`, `Position`, `Kind` and its constants are the in-memory shape of a file that third-party generators parse, and every exported field corresponds to a documented JSON key in `spec/contract.md`. `Parse`, `Schema`, `Migrate`, `GoTypeName`, `Document.Marshal`, `Document.ComputeHash` and `Document.SetHash` are used across the `gateway`, `registry`, `mcp`, `agent` and `cmd/bowline` modules. `Diff`, `Changes`, `Changes.Breaking`, `Change`, `Category` and its constants, `FormatText`, `FormatMarkdown` and `FormatJSON` are the semantic diff that the breaking-change gate and the registry impact query both call.

`Schemas.InputOrNil` and `Schemas.OutputOrNil` look like conveniences but are the only nil-safe accessors for optional embedded schemas, and both the tools exporter and the agent SDK call them.

### `signing`

| Identifier | Why |
|---|---|
| `Sign`, `Verify` | The two halves of the scheme; `Verify` is called by `bowline.Signed` and by anyone verifying outside the handler. |
| `SecretProvider`, `StaticSecrets` | A user implements `SecretProvider` to look secrets up in their own store. |
| `Transport` and its fields, `Transport.RoundTrip` | Dropped into an `http.Client` by every calling service. |
| `Header`, `Skew` | Named in the guide and needed by anyone writing a non-Go client. |
| `Option`, `WithReplayCache`, `NewReplayCache`, `ReplayCache`, `DefaultReplayCacheSize` | A deployment with more than one process supplies a shared cache instead of the per-process default. |
| `ErrMissingSignature`, `ErrMalformedSignature`, `ErrUnknownKey`, `ErrSignatureMismatch`, `ErrSkew`, `ErrReplay` | Sentinels for `errors.Is`. There are six so that each check can be matched separately. |

## Unexported

| Identifier | Was | Now | Why |
|---|---|---|---|
| `signing.ReplayCache.Len` | `func (c *ReplayCache) Len() int` | `size()` | Its only callers were tests in its own package, which an unexported method serves equally well. It also reports an implementation artifact rather than a meaningful number: the cache holds two rotating generations, so the value can reach twice the configured size and falls in steps as generations rotate. Freezing it would promise a number whose meaning is tied to the current rotation scheme. |

## Removed

Nothing. Every other identifier had at least one caller outside its own package's tests, or appears in a user-facing signature.

## Kept despite doubt

- **`bowline.WithIdempotencyScope`** is not mentioned in `docs/guides/idempotency.md`, so nobody is using it yet. It is kept because it takes and returns a `context.Context` and is the only way for application middleware to scope idempotency keys per tenant, which is exactly the case a shared store needs. The guide should gain an example.
- **`contract.Document.SetHash` and `ComputeHash`** expose the hashing step rather than a finished document. They are kept because the gateway pins upstreams by hash and the CLI writes the hash into generated documents, both outside this package, and because a caller that wants to verify a document needs to recompute the hash itself.
- **`bowline.Procedure`'s sixteen exported fields** are a wide surface to freeze. They are kept whole because the analyzer, the MCP server and both agent SDKs read them from other modules, and because narrowing the struct to an interface now would be a larger break than the one it avoids.

## Two incompatible changes already on this branch

`scripts/apidiff.sh` reports two changes against `v0.7.0`, both introduced deliberately after that tag:

- `bowline.Call` is no longer comparable, because it gained an unexported `http.Header` field for `ResponseHeader`. `CallFrom` returns `*Call`, so comparing two `Call` values was never a sensible thing to do, but `apidiff` is right that it is a change.
- `signing.Verify` gained a variadic `...Option` parameter. Calls are unaffected; assigning `Verify` to a function variable is not.

Both are permitted because `docs/stability.md` states the guarantee takes effect at 1.0, and `scripts/apidiff.sh` reports but does not fail while the baseline tag is a `v0.x`. Once a `v1` tag exists the script fails on any incompatible change with no further configuration; `scripts/apidiff.sh --strict` fails immediately and is what the release checklist runs.
