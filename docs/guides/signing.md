# Request signing

Service-to-service calls carry an HMAC signature so a server can prove the caller holds a shared secret and that the request was not altered or replayed. It is one handler option on the server and one `http.RoundTripper` on the client, so the generated Go client needs no changes.

## On the server

source: signed.go:12-14

```go
func Signed(provider signing.SecretProvider) HandlerOption {
	return func(h *handler) { h.signatures = provider }
}
```

A provider maps a key ID to a secret. The simplest one is a map:

source: signing/signing.go:29-41

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

Pass it with the other handler options:

```go
routes.Handler(bowline.Signed(signing.StaticSecrets{"billing-2026": secret}))
```

Verification runs inside `ServeHTTP` before the input is decoded and before the route is looked up, so an unsigned caller learns nothing about which procedures exist and no procedure ever sees an unverified request. Every failure is one `UNAUTHENTICATED` response; the package's sentinel errors exist so your own logs can say which check failed without telling the caller.

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

```go
client := ledgerclient.New(url, ledgerclient.WithHTTPClient(&http.Client{
	Transport: &signing.Transport{KeyID: "billing-2026", Secret: secret},
}))
```

The transport buffers the body once to hash it, signs, and restores the body, so a redirect or a retry re-reads it intact.

## The header

```
Bowline-Signature: v1,t=1762084800,kid=billing-2026,sig=<base64 HMAC-SHA256>
```

The signed string is four newline-terminated lines: the method, the request target with its query string, the lowercase hex SHA-256 of the body (of the empty string for `GET`), and the timestamp. Those four values pin a signature to one call at one moment without parsing the body.

Verification requires the timestamp to be within 300 seconds of server time, the key ID to resolve, and the HMAC to match in constant time. The window is the only replay protection: a captured request can be repeated inside it. A nonce cache would need state shared between server instances and is deliberately out of scope, so keep the window in mind for mutations that are not idempotent, and reach for `bowline.Idempotent()` where repeating a call must not repeat its effect.

Signing covers the request target as it appears on the wire, before any `http.StripPrefix`, because that is the only string both sides see identically. A signed handler also protects `.bowline/contract` and `.bowline/health`, so anything probing a signed service, a gateway included, signs its probes too.
