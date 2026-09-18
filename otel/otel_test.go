package otel_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
	bowlineotel "github.com/bowlinedev/bowline/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type in struct {
	Name string `json:"name" validate:"required"`
}

type out struct {
	Greeting string `json:"greeting"`
}

func router(seen *trace.SpanContext) *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("greet", func(ctx context.Context, i in) (out, error) {
			if seen != nil {
				*seen = trace.SpanContextFromContext(ctx)
			}
			if i.Name == "boom" {
				return out{}, bowline.Errorf(bowline.NotFound, "no such greeting")
			}
			return out{Greeting: "hello, " + i.Name}, nil
		}),
	)
}

func serve(t *testing.T, h http.Handler, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return serveAt(t, h, "greet", body, headers)
}

func serveAt(t *testing.T, h http.Handler, name, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/"+name, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func exporterProvider() (*tracetest.SpanRecorder, *sdktrace.TracerProvider) {
	rec := tracetest.NewSpanRecorder()
	return rec, sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
}

func attr(s sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.String()
		}
	}
	return ""
}

func TestASpanIsRecordedPerCall(t *testing.T) {
	rec, tp := exporterProvider()
	h := router(nil).Handler(bowlineotel.Handler(bowlineotel.WithTracerProvider(tp)))
	if w := serve(t, h, `{"name":"ada"}`, nil); w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("%d spans recorded", len(spans))
	}
	s := spans[0]
	if s.Name() != "greet" {
		t.Fatalf("span name %q", s.Name())
	}
	if s.SpanKind() != trace.SpanKindServer {
		t.Fatalf("span kind %v", s.SpanKind())
	}
	if got := attr(s, "bowline.procedure"); got != "greet" {
		t.Fatalf("bowline.procedure %q", got)
	}
	if got := attr(s, "bowline.code"); got != "OK" {
		t.Fatalf("bowline.code %q on a successful call", got)
	}
}

func TestTheErrorCodeLandsOnTheSpan(t *testing.T) {
	rec, tp := exporterProvider()
	h := router(nil).Handler(bowlineotel.Handler(bowlineotel.WithTracerProvider(tp)))
	serve(t, h, `{"name":"boom"}`, nil)
	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("%d spans", len(spans))
	}
	if got := attr(spans[0], "bowline.code"); got != "NOT_FOUND" {
		t.Fatalf("bowline.code %q", got)
	}
	if len(spans[0].Events()) == 0 {
		t.Fatal("the error was not recorded on the span")
	}
}

func TestAnIncomingTraceparentIsContinued(t *testing.T) {
	rec, tp := exporterProvider()
	var seen trace.SpanContext
	h := router(&seen).Handler(bowlineotel.Handler(bowlineotel.WithTracerProvider(tp)))
	const parent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	serve(t, h, `{"name":"ada"}`, map[string]string{"traceparent": parent})
	if !seen.IsValid() {
		t.Fatal("no span context reached the procedure")
	}
	if got := seen.TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace id %s, want the one from the incoming header", got)
	}
	if len(rec.Ended()) != 1 {
		t.Fatalf("%d spans", len(rec.Ended()))
	}
}

func TestErrorsIsUsedForWrappedCodes(t *testing.T) {
	rec, tp := exporterProvider()
	r := bowline.NewRouter(
		bowline.Query("wrapped", func(ctx context.Context, i in) (out, error) {
			return out{}, errors.Join(bowline.Errorf(bowline.PermissionDenied, "nope"))
		}),
	)
	h := r.Handler(bowlineotel.Handler(bowlineotel.WithTracerProvider(tp)))
	if w := serveAt(t, h, "wrapped", `{"name":"ada"}`, nil); w.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("%d spans", len(spans))
	}
	if got := attr(spans[0], "bowline.code"); got != "PERMISSION_DENIED" {
		t.Fatalf("bowline.code %q; a wrapped error should still be unwrapped", got)
	}
}
