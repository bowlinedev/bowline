# Runtime security audit, 2026

This is the review of every error path in the root module: what a client sees in production, what a developer sees without `Production(true)`, and the test that pins each answer. An item without a named test is not considered reviewed.

## Redaction rule

`classify` decides the status and the wire envelope for every error. The rule the audit settled on:

- A **declared error variant** is part of the contract. Its payload is what the client's generated type expects, so it is never redacted, at any status.
- Anything else that maps to a **5xx status** is replaced in production with the message `internal error`, and its `details` and `issues` are dropped. This covers plain Go errors, `bowline.Errorf(bowline.Internal, ...)`, `Unavailable`, `DataLoss`, `Unknown`, and panics.
- Anything that maps to a **4xx status** keeps its message. A 4xx message tells the caller what it did wrong and is written by the application for that purpose.
- Failed **decoding** of a request body is `INVALID_ARGUMENT`, so it is 4xx, but the decoder's message can quote a fragment of the request. In production it is reduced to `invalid input`; without `Production(true)` the decoder's own message is appended.

Evidence: `TestProductionRedactsEveryNonBowlineError`, `TestProductionRedactsDetailsAndIssuesOnServerErrors`, `TestDevelopmentKeepsServerErrorDetail`, `TestErrorBodiesNeverEchoRequestBody`.

## Every error path in the root module

| Source | Code | Status | Production message | Evidence |
|---|---|---|---|---|
| `handler.ServeHTTP` unknown path | `UNIMPLEMENTED` | 404 | `unknown procedure "<path>"` | `TestUnknownProcedureAndBadInput` |
| `handler.ServeHTTP` subscription without `Accept` | `INVALID_ARGUMENT` | 400 | `subscriptions are served as text/event-stream; send Accept: text/event-stream` | `TestSubscriptionRequiresAccept` |
| `handler.ServeHTTP` wrong method | `INVALID_ARGUMENT` | 405 | `method <m> not allowed for <path>; use <m>` | `TestMethodRules` |
| `handler.readInput` wrong content type | `INVALID_ARGUMENT` | 415 | `content type must be application/json` | `TestUnknownProcedureAndBadInput` |
| `handler.readInput` body over the limit | `INVALID_ARGUMENT` | 413 | `request body exceeds <n> bytes` | `TestBodyLimitAppliesBeforeDecode`, `TestBodyLimitClosesTheConnection` |
| `handler.readInput` read failure | `INVALID_ARGUMENT` | 400 | `invalid input` | `TestErrorBodiesNeverEchoRequestBody` |
| `handler.execute` decode failure | `INVALID_ARGUMENT` | 400 | `invalid input` | `TestErrorBodiesNeverEchoRequestBody` |
| `handler.execute` validation issues | `INVALID_ARGUMENT` | 400 | `invalid input` plus `issues` | `TestValidationIssuesAreReturned` |
| `handler.invoke` panic | `INTERNAL` | 500 | `internal error` | `TestPanicNeverLeaksStackInBody`, `TestPanicStackReachesTheLogger` |
| `handler.writeOutput` normalization failure | `INTERNAL` | 500 | `internal error` | `TestProductionRedactsEveryNonBowlineError` |
| `handler.writeOutput` encoding failure | `INTERNAL` | 500 | `internal error` | `TestProductionRedactsEveryNonBowlineError` |
| `serveUpload` wrong content type | `INVALID_ARGUMENT` | 415 | `uploads require multipart/form-data with an input part followed by a file part` | `TestUploadRequiresMultipartPost` |
| `serveUpload` misnamed first part | `INVALID_ARGUMENT` | 400 | `the first multipart part must be named input` | `TestUploadRejectsWrongPartOrder` |
| `serveUpload` input part read failure | `INVALID_ARGUMENT` | 400 | `invalid input` | `TestErrorBodiesNeverEchoRequestBody` |
| `serveUpload` input part over the limit | `INVALID_ARGUMENT` | 413 | `input part exceeds <n> bytes` | `TestUploadInputPartLimitAppliesBeforeDecode` |
| `serveUpload` misnamed second part | `INVALID_ARGUMENT` | 400 | `the second multipart part must be named file` | `TestUploadRejectsWrongPartOrder` |
| `serveUpload` file over the limit | `INVALID_ARGUMENT` | 413 | `upload exceeds <n> bytes` | `TestUploadSizeLimit` |
| `serveIdempotent` missing key | `INVALID_ARGUMENT` | 400 | `the Idempotency-Key header is required for <path>` | `TestRequireIdempotencyKey` |
| `serveIdempotent` store failure | `UNAVAILABLE` | 503 | `internal error` | `TestProductionRedactsEveryNonBowlineError` |
| `serveIdempotent` request in flight | `ABORTED` | 409 | `a request with this idempotency key is still in flight` | `TestInFlightConflict` |
| `serveSSE` writer cannot stream | `INTERNAL` | 500 | `internal error` | `TestProductionRedactsEveryNonBowlineError` |
| `verifySignature` body over the limit | `INVALID_ARGUMENT` | 413 | `request body exceeds <n> bytes` | `TestSignedRejectsATamperedBodyBeforeDecoding` |
| `verifySignature` bad signature | `UNAUTHENTICATED` | 401 | `a valid Bowline-Signature header is required` | `TestSignedRejectsUnsignedRequestsBeforeTheProcedureRuns` |
| `serveReserved` unknown reserved path | `UNIMPLEMENTED` | 404 | `unknown procedure "<name>"` | `TestReservedPathsAreUnimplementedWithoutTheOption` |
| `serveReserved` wrong method | `INVALID_ARGUMENT` | 405 | `method <m> not allowed for <name>; use GET` | `TestReservedPathsRejectOtherMethods` |
| `Router.Subscribe` unknown path | `UNIMPLEMENTED` | 404 | `unknown procedure "<path>"` | `TestSubscribeRejectsBadRequests` |
| `Router.Subscribe` wrong kind | `INVALID_ARGUMENT` | 400 | `<path> is a <kind>, not a subscription` | `TestSubscribeRejectsBadRequests` |
| `Router.Subscribe` decode failure | `INVALID_ARGUMENT` | 400 | `invalid input` | `TestErrorBodiesNeverEchoRequestBody` |

Paths that quote the request URL do so with `%q`, which escapes the value, and the response is always `application/json` with `X-Content-Type-Options: nosniff` when `SecurityHeaders()` is set, so a quoted path cannot be interpreted as markup by a browser.

## Findings and fixes

**F1. A 5xx `bowline.Error` was returned verbatim in production.** `classify` matched `*Error` before the production check, so `bowline.Errorf(bowline.Internal, "dial %s", dsn)` reached the client with the DSN in it. Only errors that were *not* Bowline errors were redacted. Fixed by moving every non-variant branch through `redact`, which replaces the message, `details`, and `issues` on any 5xx. Evidence: `TestProductionRedactsEveryNonBowlineError`, `TestProductionRedactsDetailsAndIssuesOnServerErrors`.

**F2. The panic path returned the panic value in production.** `handler.invoke` built `Errorf(Internal, "panic: %v", rec)`, which F1 then passed through untouched. A panic value frequently carries a query, a key, or a file path. Fixed by F1: production now answers `internal error`, and the stack still reaches the logger only. Evidence: `TestPanicNeverLeaksStackInBody`, `TestPanicStackReachesTheLogger`.

**F3. Decoder errors echoed request body content.** `encoding/json` puts the offending literal in the message: a number decoded into a string field produces `cannot unmarshal number 9182736450 into ...`, and `DisallowUnknownFields` names the field the request chose. Both reflect request content into the response. Fixed with `handler.invalidInput`, which answers `invalid input` in production and keeps the decoder's message otherwise. The same helper is used by the upload and `Subscribe` paths. Evidence: `TestErrorBodiesNeverEchoRequestBody`.

**F4. Body limits confirmed to apply before any read.** `readInput` wraps the body in `http.MaxBytesReader` before the first `Read`, `serveUpload` wraps the multipart reader's source, and `verifySignature` wraps before buffering for the signature. In every case the limit is enforced by the reader itself, so an oversized body is never fully buffered and the procedure never runs. `MaxBytesReader` is given the real `http.ResponseWriter`, so `net/http` marks the response and closes the connection instead of trying to drain a hostile body. Evidence: `TestBodyLimitAppliesBeforeDecode`, `TestBodyLimitClosesTheConnection`. No fix needed.

**F5. Validation issues carry no request content.** Every message `internal/validate` produces is built from the rule and its parameter (for example `must be at least 3 characters`, or `must be one of draft sent paid`) and never from the value under test. Issues are therefore safe to return at 400. They are still dropped on a 5xx by F1's `redact`. No fix needed.

## Open items

- `MaxBodySize(0)` is taken literally and rejects every body. The default of 1 MiB is applied only when the option is absent. This is a usability hazard rather than a leak. It is left as is so that a zero limit keeps its meaning, and it is documented in `docs/guides/security.md`.
- Redaction is keyed on the HTTP status, so an application that declares an error variant mapping to a 5xx code opts that variant out of redaction. That is the intended escape hatch: a declared variant is contract-visible by definition.

## Middleware overhead

`RateLimit` adds one mutex acquisition, one map lookup, and one list splice per call. Measured on an Apple M1 Pro with `go test -run XXX -bench RateLimit -benchmem .`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BenchmarkRateLimitAllow` | 72.85 | 0 | 0 |
| `BenchmarkRateLimitMiddleware` | 73.42 | 0 | 0 |

The limiter allocates only when it first sees a key, so a steady-state workload with a bounded key set runs allocation-free. `MaxKeys` caps the map, and the least recently used bucket is evicted when a new key arrives at the cap, so key churn cannot grow memory without bound.
