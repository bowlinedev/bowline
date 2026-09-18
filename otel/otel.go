package otel

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bowlinedev/bowline"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const scope = "github.com/bowlinedev/bowline"

type Option func(*observer)

func WithTracerProvider(p trace.TracerProvider) Option {
	return func(o *observer) { o.tracer = p.Tracer(scope) }
}

func WithMeterProvider(p metric.MeterProvider) Option {
	return func(o *observer) { o.meter = p.Meter(scope) }
}

func WithPropagator(p propagation.TextMapPropagator) Option {
	return func(o *observer) { o.propagator = p }
}

type observer struct {
	tracer     trace.Tracer
	meter      metric.Meter
	propagator propagation.TextMapPropagator

	once     sync.Once
	duration metric.Float64Histogram
	calls    metric.Int64Counter

	mu    sync.Mutex
	spans map[*bowline.Call]spanState
}

type spanState struct {
	span    trace.Span
	started time.Time
}

func Handler(opts ...Option) bowline.HandlerOption {
	o := &observer{spans: map[*bowline.Call]spanState{}}
	for _, opt := range opts {
		opt(o)
	}
	if o.tracer == nil {
		o.tracer = noop.NewTracerProvider().Tracer(scope)
	}
	if o.propagator == nil {
		o.propagator = propagation.TraceContext{}
	}
	return bowline.Observe(o)
}

func (o *observer) instruments() {
	o.once.Do(func() {
		if o.meter == nil {
			return
		}
		o.duration, _ = o.meter.Float64Histogram("bowline.call.duration",
			metric.WithUnit("s"),
			metric.WithDescription("time a procedure took to answer"))
		o.calls, _ = o.meter.Int64Counter("bowline.call.count",
			metric.WithDescription("procedure calls answered"))
	})
}

func (o *observer) HandlerReady(procedures []*bowline.Procedure) {
	o.instruments()
}

func (o *observer) CallStarted(ctx context.Context, call *bowline.Call) context.Context {
	if call == nil || call.Procedure == nil {
		return ctx
	}
	o.instruments()
	if call.Request != nil {
		ctx = o.propagator.Extract(ctx, propagation.HeaderCarrier(call.Request.Header))
	}
	attrs := []attribute.KeyValue{
		attribute.String("bowline.procedure", call.Procedure.Path),
		attribute.String("bowline.kind", string(call.Procedure.Kind)),
		attribute.String("http.request.method", call.Procedure.Method()),
	}
	ctx, span := o.tracer.Start(ctx, call.Procedure.Path,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...))
	o.mu.Lock()
	o.spans[call] = spanState{span: span, started: time.Now()}
	o.mu.Unlock()
	return ctx
}

func (o *observer) CallFinished(ctx context.Context, call *bowline.Call, err error) {
	if call == nil {
		return
	}
	o.mu.Lock()
	state, ok := o.spans[call]
	delete(o.spans, call)
	o.mu.Unlock()
	if !ok {
		return
	}
	code := "OK"
	var coded *bowline.Error
	switch {
	case errors.As(err, &coded):
		code = string(coded.Code)
	case err != nil:
		code = string(bowline.Internal)
	}
	attrs := []attribute.KeyValue{
		attribute.String("bowline.procedure", call.Procedure.Path),
		attribute.String("bowline.code", code),
	}
	state.span.SetAttributes(attribute.String("bowline.code", code))
	if err != nil {
		state.span.RecordError(err)
		state.span.SetStatus(codes.Error, code)
	}
	state.span.End()
	elapsed := time.Since(state.started).Seconds()
	if o.duration != nil {
		o.duration.Record(ctx, elapsed, metric.WithAttributes(attrs...))
	}
	if o.calls != nil {
		o.calls.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}
