# Request signing audit

Audit of the `signing` package and the `bowline.Signed` handler option against the signing checklist. Every row names the test that holds it.

## Canonicalization

A signature covers exactly this string, each line terminated by `\n`:

```
<METHOD>
<request target with query string>
<lowercase hex SHA-256 of the body>
<unix timestamp>
<nonce>
```

- The method is upper-cased before signing, so a caller that sends `post` and a server that reads `POST` agree.
- The request target is the path as it appears on the wire, before any `http.StripPrefix`, together with its raw query string. The server reads it from `req.RequestURI`. This is the only form of the target both sides observe identically; taking it from `req.URL` after routing would let a proxy rewrite change the signed string.
- The body hash is over the bytes actually sent. A `GET` hashes the empty string, so a nil body and a zero-length body produce the same line (`TestAnEmptyBodyAndANilBodyHashTheSame`).
- The nonce is sixteen bytes from `crypto/rand`, base64url without padding. It is the fifth line only when present; a header without `n=` signs the four-line string, which is what keeps 0.7.0 callers working (`TestAnOlderClientWithoutANonceStillVerifies`).

Every element is fixed-position and newline-delimited, so no element can be shifted into another: a path ending in a digit cannot be read as part of the timestamp.

## Findings

| Item | Status | Evidence |
|---|---|---|
| HMAC over SHA-256 | held | `mac` uses `hmac.New(sha256.New, secret)` |
| Constant-time comparison | held | `hmac.Equal`, which is `subtle.ConstantTimeCompare`; `TestSignatureComparisonIsConstantTime` |
| Timestamp window | held, hardened | `TestClockSkewWindow`, `TestClockSkewRejectsTimestampsThatWouldOverflow` |
| Nonce and replay cache | added | `TestReplayWindowRejectsReuse`, `TestReplayCacheIsBounded`, `TestReplayCacheForgetsPastTheSkewWindow` |
| Canonical string covers method, path, body, timestamp | held | `TestVerifyRejectsTampering`, `TestSignedRejectsAMovedSignature` |
| Failures are indistinguishable to the caller | held | `TestSignedDoesNotLeakWhichCheckFailed` |
| Body is bounded before it is hashed | held | `verifySignature` wraps the body in `http.MaxBytesReader` at `signingLimit` |

### Replay protection was missing

Before this audit the 300-second window was the only replay protection, and a captured request could be repeated freely inside it. Two changes close that:

1. `Sign` now draws a per-signature nonce and includes it in the canonical string. Without it, two legitimate identical calls in the same second produce the same signature and are indistinguishable from a replay, so a cache alone would reject honest traffic.
2. `signing.ReplayCache` records every signature that passes verification and rejects a second sighting with `ErrReplay`. `bowline.Signed` builds one automatically, so existing servers gain the protection with no code change; `signing.Verify` leaves it off unless given `signing.WithReplayCache`.

The cache is two generations of a set, rotated when the skew window elapses or when the live generation reaches its bound. That keeps insert and lookup O(1), bounds memory at twice the configured size, and needs no background timer. Entries live between one and two windows.

Only verified signatures are recorded, so an attacker without the secret cannot flush the cache by flooding it. A legitimate key holder can, at the cost of shortening the window in which their own signatures are remembered.

The cache is per process. A fleet behind a load balancer protects each instance, not the fleet. Cross-instance protection needs shared state, which is what `bowline.Idempotent()` provides for the mutations that require it.

### The skew check could overflow

`Verify` computed `now.Unix() - timestamp` and multiplied the result by `time.Second` before comparing it with the window. A timestamp near the int64 extremes overflowed twice and could land back inside the accepted range. It was not exploitable, because the timestamp is part of the canonical string and a forged one needs the secret, but the check was wrong. It now compares seconds against seconds and never multiplies (`TestClockSkewRejectsTimestampsThatWouldOverflow`).

## Not fixed

- `StaticSecrets.Secret` returns the caller's own slice rather than a copy, so a caller could mutate a secret in place. Changing it would allocate on every request for a hazard that only a caller with the secret can trigger.
- The window is symmetric: a signature 300 seconds in the future is accepted, which tolerates a client whose clock runs fast. Narrowing the future half would break deployments with unsynchronized clocks.
