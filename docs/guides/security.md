# Security

Two handler options harden a browser-facing deployment: `CSRF` rejects mutations that a page on another site started, and `SecurityHeaders` stops a browser from reinterpreting a response. Both are opt-in, because a service that only serves other services wants neither.

The full review of what every error path says in production is `../security/audit-2026.md`.

## CSRF

source: csrf.go:10-21

```go
type CSRFOptions struct {
	AllowedOrigins     []string
	TrustFetchMetadata bool
}

func CSRF(options CSRFOptions) HandlerOption {
	guard := &csrf{
		allowed:            slices.Clone(options.AllowedOrigins),
		trustFetchMetadata: options.TrustFetchMetadata,
	}
	return func(h *handler) { h.csrf = guard }
}
```

The check runs inside `ServeHTTP`, after the method is validated and before a single byte of the body is read, so a rejected request never reaches the decoder or the procedure.

source: csrf.go:33-53

```go
	if req.Method != http.MethodPost {
		return true
	}
	if c.trustFetchMetadata {
		switch req.Header.Get("Sec-Fetch-Site") {
		case "same-origin", "none":
			return true
		case "same-site", "cross-site":
			return c.listed(req.Header.Get("Origin"))
		}
	}
	if origin := req.Header.Get("Origin"); origin != "" {
		return c.sameHostOrListed(origin, req.Host)
	}
	if referer := req.Header.Get("Referer"); referer != "" {
		u, err := url.Parse(referer)
		if err != nil || u.Host == "" {
			return false
		}
		return c.sameHostOrListed(u.Scheme+"://"+u.Host, req.Host)
	}
```

In words:

1. Only `POST` is checked.
2. With `TrustFetchMetadata`, a `Sec-Fetch-Site` of `same-origin` or `none` passes and nothing else is consulted; `same-site` and `cross-site` pass only if `Origin` is in `AllowedOrigins`. An unrecognized value falls through to the next rule, so a future value never fails open or closed by accident.
3. Otherwise `Origin` must name the same host as the request, or be listed in `AllowedOrigins`.
4. Otherwise `Referer`'s origin must satisfy the same rule.
5. Otherwise the request is rejected, because a browser always sends one of these on a cross-site `POST`.

A rejection is one `PERMISSION_DENIED` with the message `cross-origin request rejected` and no further detail, so a probing page learns nothing about which origins are allowed.

Only the **host** is compared, not the scheme. A TLS-terminating proxy hands the handler a plain `http` request whose `Origin` still says `https`, and comparing schemes would reject every proxied deployment. If you need the scheme pinned, put the exact public origin in `AllowedOrigins`.

### Non-browser callers

Rule 5 rejects a caller that sends no `Origin`, `Referer`, or `Sec-Fetch-Site` — which is every `curl`, every generated non-browser client, and every service-to-service call. That is deliberate: a request with none of those headers cannot be distinguished from a forged form post. Mount `CSRF` only on the route your browser app uses, and leave the machine-to-machine mount without it, authenticated by `Signed` instead. The ledger example does exactly that:

source: examples/ledger/cmd/server/main.go:44-52

```go
func browserOptions() []bowline.HandlerOption {
	if os.Getenv("CSRF") != "on" {
		return nil
	}
	return []bowline.HandlerOption{bowline.CSRF(bowline.CSRFOptions{
		TrustFetchMetadata: true,
		AllowedOrigins:     allowedOrigins(),
	})}
}
```

Those options are added to the `/api` mount only; `/mcp` and `/ws` keep the base set.

### Why queries are exempt

A query is served over `GET` and never mutates state, so there is nothing for a cross-site request to change. A browser can already read a cross-origin `GET` response only if CORS allows it, and Bowline sets no CORS headers, so a forged query is a request the attacker cannot read the answer to.

If a query in your service does mutate state, that is a bug in the procedure, not something the transport can fix: make it a `Mutation`. Nothing in the runtime enforces it, because the handler cannot know what your code does.

`POST` already requires `application/json`, which a cross-site HTML form cannot produce — a form may only send `application/x-www-form-urlencoded`, `multipart/form-data`, or `text/plain`. `CSRF` is defense in depth for the case where a browser bug or a permissive CORS configuration relaxes that.

## What `Sensitive()` does and does not do

source: item.go:27-29

```go
func Sensitive() ProcOption {
	return func(p *Procedure) { p.Sensitive = true }
}
```

A sensitive query is served over `POST` instead of `GET`, so its input travels in the body rather than in the URL. That keeps it out of browser history, out of `Referer` headers, and out of the access logs of every proxy on the path.

It is **not** a CSRF control, an authorization control, or encryption. A sensitive procedure is as reachable as any other; the only thing that changes is where the input is written. Authorization belongs in your middleware, and the input is protected on the wire by TLS, not by this option.

## Security headers

source: csrf.go:68-76

```go
func (h *handler) secure(w http.ResponseWriter, method string) {
	if !h.securityHeaders {
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if method == http.MethodPost {
		w.Header().Set("Cache-Control", "no-store")
	}
}
```

`SecurityHeaders()` sets `X-Content-Type-Options: nosniff` on every response the handler writes, and `Cache-Control: no-store` on every error and on every `POST`. A successful query keeps whatever caching your own middleware chose, so a cacheable read stays cacheable.

## Reverse proxy checklist

The handler sets only what it can be sure about. Everything below belongs to the proxy or the framework that serves your HTML, because it applies to the whole site and not to one API mount:

| Header | Value | Why |
|---|---|---|
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` | the handler cannot know whether every subdomain is on TLS |
| `Content-Security-Policy` | at least `default-src 'none'; frame-ancestors 'none'` for the API | an API returns no markup, so everything can be denied |
| `X-Frame-Options` | `DENY` | for user agents that predate `frame-ancestors` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | keeps query strings out of outbound referers |
| `Access-Control-Allow-Origin` | set only if a browser on another origin must call you | Bowline sets no CORS headers, so cross-origin reads are blocked by default |

Also strip any inbound `X-Forwarded-*` header the client set before your proxy adds its own, and terminate TLS in front of the handler: nothing in the runtime upgrades a plaintext connection.

## Body limits

The handler reads at most `MaxBodySize` bytes, 1 MiB by default, and `MaxUploadSize` for the file part of an upload, 32 MiB by default. Both wrap the body before the first read, so an oversized request is refused without being buffered.

`MaxBodySize(0)` means zero bytes, not "unlimited" — the default applies only when the option is absent.

A single procedure can raise or lower its own ceiling:

source: item.go:31-33

```go
func MaxBody(n int64) ProcOption {
	return func(p *Procedure) { p.MaxBody = n }
}
```

Zero inherits the handler's limit. On an upload the value caps the whole multipart body rather than the JSON input part, so `MaxBody(4 << 20)` on an avatar upload and `MaxBody(500 << 20)` on a video upload can sit behind one handler. The analyzer records the value in the contract as `meta.maxBody`, so a client or the mock server can refuse an oversized request before it is sent.

## Rate limiting

`RateLimit` is a `Middleware`, so it goes on one procedure with `Use`, on a sub-router, or on the whole router.

source: ratelimit.go:13-19

```go
type RateLimitOptions struct {
	Key     func(ctx context.Context) string
	Rate    float64
	Burst   int
	MaxKeys int
	Now     func() time.Time
}
```

`Key` decides what is being limited — a tenant, an API key, a remote address. Returning an empty string bypasses the limiter entirely, which is how you exempt a trusted caller. `Rate` is tokens per second and `Burst` is the ceiling; `Burst` defaults to one second's worth. `MaxKeys` bounds the map, defaulting to 10000, and `Now` exists so a test can drive the clock.

Each key gets one token bucket, and the least recently used bucket is evicted once the map is full, so an attacker cycling keys cannot grow memory without bound:

source: ratelimit.go:87-113

```go
func (l *limiter) allow(key string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		for len(l.buckets) >= l.maxKeys {
			if !l.evict() {
				break
			}
		}
		b = &bucket{key: key, tokens: l.burst, updated: now}
		b.elem = l.order.PushFront(b)
		l.buckets[key] = b
	} else {
		if elapsed := now.Sub(b.updated); elapsed > 0 {
			b.tokens = math.Min(l.burst, b.tokens+elapsed.Seconds()*l.rate)
			b.updated = now
		}
		l.order.MoveToFront(b.elem)
	}
	if b.tokens < 1 {
		return time.Duration((1 - b.tokens) / l.rate * float64(time.Second)), false
	}
	b.tokens--
	return 0, true
}
```

A refused call is `RESOURCE_EXHAUSTED`, which maps to 429, and the middleware sets `Retry-After` through `bowline.CallFrom(ctx).ResponseHeader()`. The handler copies those headers onto the response once the chain returns, so any middleware can add a response header without holding the `http.ResponseWriter`.

The ledger limits its destructive mutation per caller address:

source: examples/ledger/api/invoices.go:35-42

```go
func voidLimit() bowline.Middleware {
	return bowline.RateLimit(bowline.RateLimitOptions{
		Key:     CallerAddress,
		Rate:    5,
		Burst:   10,
		MaxKeys: 4096,
	})
}
```

The limiter is per process. Behind several replicas each one holds its own buckets, so the effective limit is the configured rate times the replica count; put a shared limiter in front if that matters. Measured overhead is in `../security/audit-2026.md`.
