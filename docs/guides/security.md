# Security

Two handler options are available for hardening a browser-facing deployment. `CSRF` rejects mutations that were started by a page on another site, and `SecurityHeaders` stops a browser from reinterpreting a response. Both are opt-in, since a service that only talks to other services does not need either.

The full review of what every error path says in production is in `../security/audit-2026.md`.

## CSRF

source: csrf.go:10-13

```go
type CSRFOptions struct {
	AllowedOrigins     []string
	TrustFetchMetadata bool
}
```

The check runs inside `ServeHTTP`, after the method is validated and before any of the body is read. A rejected request never reaches the decoder or the procedure.

source: internal/csrf/csrf.go:18-40

```go
func (c *Guard) Allows(req *http.Request) bool {
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
	return req.Header.Get("Sec-Fetch-Site") == ""
```

In words:

1. Only `POST` is checked.
2. With `TrustFetchMetadata`, a `Sec-Fetch-Site` of `same-origin` or `none` passes and nothing else is looked at. `same-site` and `cross-site` pass only if `Origin` is in `AllowedOrigins`. An unrecognized value falls through to the next rule, so a future value cannot accidentally fail open or closed.
3. Otherwise `Origin` must be the same host as the request, or be in `AllowedOrigins`.
4. Otherwise the origin from `Referer` must satisfy the same rule.
5. A request carrying none of the three headers is allowed. It is not a browser, and CSRF is a browser attack.
6. Otherwise the request is rejected. A browser that announced itself through `Sec-Fetch-Site` but sent no `Origin` cannot be verified.

A rejection is a single `PERMISSION_DENIED` with the message `cross-origin request rejected` and nothing else, so a probing page learns nothing about which origins are allowed.

Only the **host** is compared, not the scheme. A TLS-terminating proxy hands the handler a plain `http` request whose `Origin` still says `https`, and comparing schemes would reject every proxied deployment. If you need the scheme pinned, put the exact public origin in `AllowedOrigins`.

### Non-browser callers

Rule 5 is the same decision the standard library makes in `net/http.CrossOriginProtection`, and it is worth spelling out why. Every `curl`, every generated non-browser client, and every service-to-service call sends no `Origin`, `Referer`, or `Sec-Fetch-Site`. Rejecting those gains nothing. An attacker who is not driving a victim's browser does not need a forged form; they can send the request directly with whatever headers they like. What rejecting them would cost is every legitimate non-browser caller, which is most of them.

Browsers have sent `Origin` on cross-origin `POST` since 2016 and `Sec-Fetch-Site` since 2023, so a browser-driven forgery always carries at least one of the three and is caught by rules 2 through 4. This means `CSRF` is safe to mount on a route that serves both browsers and machines. It is still not authentication. Pair it with `Signed` or your own scheme for service-to-service calls. The ledger example enables it on the browser mount:

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

These options are only added to the `/api` mount. `/mcp` and `/ws` keep the base set.

### Why queries are exempt

A query is served over `GET` and does not mutate state, so there is nothing for a cross-site request to change. A browser can only read a cross-origin `GET` response if CORS allows it, and Bowline sets no CORS headers, so a forged query is a request whose answer the attacker cannot read.

If a query in your service does mutate state, that is a bug in the procedure and not something the transport can fix. Make it a `Mutation`. The runtime does not enforce this, because the handler cannot know what your code does.

`POST` already requires `application/json`, which a cross-site HTML form cannot produce. A form can only send `application/x-www-form-urlencoded`, `multipart/form-data`, or `text/plain`. `CSRF` is defense in depth for the case where a browser bug or a permissive CORS configuration relaxes that.

## What `Sensitive()` does and does not do

source: item.go:27-29

```go
func Sensitive() ProcOption {
	return func(p *Procedure) { p.Sensitive = true }
}
```

A sensitive query is served over `POST` instead of `GET`, so its input goes in the body rather than the URL. That keeps it out of browser history, out of `Referer` headers, and out of the access logs of every proxy on the path.

It is **not** a CSRF control, an authorization control, or encryption. A sensitive procedure is as reachable as any other. The only thing that changes is where the input is written. Authorization belongs in your middleware, and the input is protected on the wire by TLS, not by this option.

## Security headers

source: csrf.go:24-32

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

`SecurityHeaders()` sets `X-Content-Type-Options: nosniff` on every response the handler writes, and `Cache-Control: no-store` on every error and on every `POST`. A successful query keeps whatever caching your own middleware set, so a cacheable read stays cacheable.

## Reverse proxy checklist

The handler only sets what it can be sure about. Everything below belongs to the proxy or the framework that serves your HTML, because it applies to the whole site and not to one API mount:

| Header | Value | Why |
|---|---|---|
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` | the handler cannot know whether every subdomain is on TLS |
| `Content-Security-Policy` | at least `default-src 'none'; frame-ancestors 'none'` for the API | an API returns no markup, so everything can be denied |
| `X-Frame-Options` | `DENY` | for user agents that predate `frame-ancestors` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | keeps query strings out of outbound referers |
| `Access-Control-Allow-Origin` | set only if a browser on another origin must call you | Bowline sets no CORS headers, so cross-origin reads are blocked by default |

Also strip any inbound `X-Forwarded-*` header the client set before your proxy adds its own, and terminate TLS in front of the handler. Nothing in the runtime upgrades a plaintext connection.

## Body limits

The handler reads at most `MaxBodySize` bytes (1 MiB by default), and at most `MaxUploadSize` for the file part of an upload (32 MiB by default). Both wrap the body before the first read, so an oversized request is refused without being buffered.

`MaxBodySize(0)` means zero bytes, not unlimited. The default only applies when the option is not set at all.

A single procedure can raise or lower its own limit:

source: item.go:31-33

```go
func MaxBody(n int64) ProcOption {
	return func(p *Procedure) { p.MaxBody = n }
}
```

Zero inherits the handler's limit. On an upload the value caps the whole multipart body rather than just the JSON input part, so `MaxBody(4 << 20)` on an avatar upload and `MaxBody(500 << 20)` on a video upload can both sit behind one handler. The analyzer records the value in the contract as `meta.maxBody`, so a client or the mock server can refuse an oversized request before sending it.

## Rate limiting

`RateLimit` is a `Middleware`, so it can go on one procedure with `Use`, on a sub-router, or on the whole router.

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

`Key` decides what is being limited: a tenant, an API key, a remote address. Returning an empty string bypasses the limiter completely, which is how you exempt a trusted caller. `Rate` is tokens per second and `Burst` is the ceiling. `Burst` defaults to one second's worth. `MaxKeys` bounds the map and defaults to 10000. `Now` exists so a test can control the clock.

Each key gets one token bucket. When the map is full, the least recently used bucket is evicted, so an attacker cycling through keys cannot grow memory without limit:

source: internal/ratelimit/ratelimit.go:51-77

```go
func (l *Limiter) Allow(key string) (time.Duration, bool) {
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

A refused call gets `RESOURCE_EXHAUSTED`, which maps to 429. The middleware sets `Retry-After` through `bowline.CallFrom(ctx).ResponseHeader()`. The handler copies those headers onto the response once the chain returns, so any middleware can add a response header without needing the `http.ResponseWriter`.

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

The limiter is per process. Behind several replicas, each replica has its own buckets, so the effective limit is the configured rate multiplied by the number of replicas. Put a shared limiter in front if that matters. Measured overhead is in `../security/audit-2026.md`.
