# Observability

The `otel` module is provisional as a whole; see `docs/provisional.md`.

The `otel` module turns every procedure call into an OpenTelemetry span and two metrics. It is a separate module, so the runtime keeps its standard-library-only dependency list:

```bash
go get github.com/bowlinedev/bowline/otel
```

## Wiring it up

`otel.Handler` returns a `bowline.HandlerOption`, so it goes where the other handler options go:

sketch: the providers come from your own OpenTelemetry setup

```go
r.Mount("/api", routes.Handler(
	otel.Handler(otel.WithTracerProvider(tp), otel.WithMeterProvider(mp)),
	bowline.Production(true),
))
```

With no options it is inert: a no-op tracer, no meter, and the W3C trace-context propagator. That makes it safe to leave wired in a test binary.

| Option | Effect |
|---|---|
| `WithTracerProvider(p)` | where spans go |
| `WithMeterProvider(p)` | where metrics go; without it, no metrics are recorded |
| `WithPropagator(p)` | how incoming trace context is read; the default is W3C `traceparent` |

## What it records

One server span per call, named after the procedure path, so a trace reads `invoices.void` rather than `POST /api`. The span carries:

| Attribute | Value |
|---|---|
| `bowline.procedure` | the procedure path, for example `invoices.void` |
| `bowline.kind` | `query`, `mutation`, `subscription` or `upload` |
| `http.request.method` | the method the procedure answers on |
| `bowline.code` | the Bowline error code, or `OK` |

A failed call records the error on the span and sets the span status to the code. The code comes from `errors.As`, so a wrapped `*bowline.Error` is still found, and any other error is `INTERNAL` — the same classification the client sees.

Two instruments, both labelled with `bowline.procedure` and `bowline.code`:

- `bowline.call.duration`, a histogram in seconds
- `bowline.call.count`, a counter of calls answered

Because the span closes on the error the procedure returned, not on the HTTP status, a trace distinguishes `NOT_FOUND` from `PERMISSION_DENIED` even though both are 4xx, and it sees the real message even when `Production(true)` redacts it on the wire.

## Incoming trace context

A caller's `traceparent` header is extracted from the request before the span starts, so a call from another traced service continues that trace. The generated clients do not add the header themselves; propagate it with whatever your HTTP client already uses.

## Doing it yourself

`otel.Handler` is a `bowline.Observer` and nothing more. If you already have a metrics pipeline, or you want an audit log rather than traces, write the observer instead. `guides/middleware.md` covers the interface.
