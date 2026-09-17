# Request signing

Service-to-service calls can carry an HMAC signature, so that a server can verify the caller holds a shared secret and that the request was not altered or replayed. On the server it is one handler option. On the client it is one `http.RoundTripper`. The generated Go client does not need to change.

## On the server

source: signed.go:12-19

```go
func Signed(provider signing.SecretProvider) HandlerOption {
	return func(h *handler) {
		h.signatures = provider
		if h.replay == nil {
			h.replay = signing.NewReplayCache(signing.DefaultReplayCacheSize)
		}
	}
}
```

A provider maps a key ID to a secret. The simplest provider is a map:

source: signing/signing.go:41-53

```go
type SecretProvider interface {
	Secret(ctx context.Context, keyID string) ([]byte, error)
}

type StaticSecrets map[string][]byte

func (s StaticSecrets) Secret(ctx context.Context, keyID string) ([]byte, error) {
	secret, ok := s[keyID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKey, keyID)
	}
	return secret, nil
}
```

Pass it along with the other handler options:

source: examples/federation/billing/cmd/server/main.go:27-29

```go
	if secret := os.Getenv("BILLING_INBOUND_SECRET"); secret != "" {
		options = append(options, bowline.Signed(signing.StaticSecrets{os.Getenv("BILLING_INBOUND_KEY"): []byte(secret)}))
	}
```

Verification happens inside `ServeHTTP`, before the input is decoded and before the route is looked up. An unsigned caller learns nothing about which procedures exist, and no procedure ever sees an unverified request. Every failure produces the same `UNAUTHENTICATED` response. The package has sentinel errors so that your own logs can record which check failed, without telling the caller.

## On the client

source: signing/transport.go:10-15

```go
type Transport struct {
	Base   http.RoundTripper
	KeyID  string
	Secret []byte
	Now    func() time.Time
}
```

source: examples/federation/billing/api/ledger.go:14-16

```go
	return ledgerclient.New(url, ledgerclient.WithHTTPClient(&http.Client{
		Transport: &signing.Transport{KeyID: keyID, Secret: secret},
	}))
```

The transport reads the body once to hash it, signs, and then restores the body, so a redirect or a retry can re-read it.

## The header

```
Bowline-Signature: v1,t=1762084800,kid=billing-2026,sig=<base64 HMAC-SHA256>,n=<base64url nonce>
```

The signed string is five newline-terminated lines: the method, the request target including its query string, the lowercase hex SHA-256 of the body (or of the empty string for `GET`), the timestamp, and the nonce. Together these tie a signature to one call at one moment without parsing the body. The nonce is sixteen random bytes generated per signature. This is what makes two identical calls in the same second produce different signatures.

A header without `n=` omits the nonce line and still verifies, so a caller built against 0.7.0 keeps working. It does not get replay protection though, because without a nonce two identical calls in the same second cannot be told apart from a replay.

Verification requires the timestamp to be within 300 seconds of server time, the key ID to resolve, and the HMAC to match (compared in constant time). A signature that passes all three checks is then recorded, and a second request carrying the same signature is rejected. The record is a bounded in-memory set per process. It holds at most `signing.DefaultReplayCacheSize` signatures per window and forgets anything older than two windows, so memory stays bounded without needing a timer. Nothing is shared between server instances. A fleet behind a load balancer protects each instance separately rather than the fleet as a whole. For mutations where repeating a call must not repeat its effect, use `bowline.Idempotent()`, which is durable and shared.

Signing covers the request target as it appears on the wire, before any `http.StripPrefix`, since that is the only string both sides see the same way. A signed handler also protects `.bowline/contract` and `.bowline/health`, so anything that probes a signed service, including a gateway, has to sign its probes too.
