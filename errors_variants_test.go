package bowline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

type invoiceLocked struct {
	ID     int64     `json:"id"`
	Status string    `json:"status"`
	Since  time.Time `json:"since"`
	Tags   []string  `json:"tags"`
}

func (e invoiceLocked) Error() string { return fmt.Sprintf("invoice %d is %s", e.ID, e.Status) }
func (e invoiceLocked) Code() Code    { return FailedPrecondition }

type quotaExceeded struct {
	Limit int `json:"limit"`
}

func (e *quotaExceeded) Error() string { return "quota exceeded" }
func (e *quotaExceeded) Code() Code    { return ResourceExhausted }

type undeclared struct{}

func (undeclared) Error() string { return "undeclared" }
func (undeclared) Code() Code    { return Aborted }

func variantProc(ctx context.Context, in getInput) (user, error) {
	switch in.ID {
	case 1:
		return user{}, invoiceLocked{ID: 1, Status: "paid", Since: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)}
	case 2:
		return user{}, fmt.Errorf("wrapped: %w", &quotaExceeded{Limit: 5})
	case 3:
		return user{}, undeclared{}
	}
	return user{ID: in.ID}, nil
}

func variantHandler(opts ...HandlerOption) http.Handler {
	return NewRouter(Query("get", variantProc, Errors(invoiceLocked{}, &quotaExceeded{}))).Handler(opts...)
}

func TestDeclaredVariantIsSerializedWithType(t *testing.T) {
	rec := do(variantHandler(), http.MethodPost, "/get", `{"id":1}`, nil)
	if rec.Code != 412 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Error struct {
			Code    Code            `json:"code"`
			Message string          `json:"message"`
			Type    string          `json:"type"`
			Details json.RawMessage `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != FailedPrecondition || env.Error.Type != "invoiceLocked" || env.Error.Message != "invoice 1 is paid" {
		t.Fatalf("body %s", rec.Body.String())
	}
	if !strings.Contains(string(env.Error.Details), `"status":"paid"`) || !strings.Contains(string(env.Error.Details), `"tags":[]`) || !strings.Contains(string(env.Error.Details), `"since":"2026-09-15T00:00:00Z"`) {
		t.Fatalf("details %s", env.Error.Details)
	}
}

func TestWrappedPointerVariantIsFound(t *testing.T) {
	rec := do(variantHandler(), http.MethodPost, "/get", `{"id":2}`, nil)
	if rec.Code != 429 || !strings.Contains(rec.Body.String(), `"type":"quotaExceeded"`) || !strings.Contains(rec.Body.String(), `"limit":5`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestUndeclaredCodedErrorKeepsCode(t *testing.T) {
	var logs strings.Builder
	h := variantHandler(Logger(slog.New(slog.NewTextHandler(&logs, nil))))
	rec := do(h, http.MethodPost, "/get", `{"id":3}`, nil)
	if rec.Code != 409 || strings.Contains(rec.Body.String(), `"type"`) || !strings.Contains(rec.Body.String(), `"code":"ABORTED"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), "undeclared error variant") {
		t.Fatalf("expected a warning, got %q", logs.String())
	}
	logs.Reset()
	prod := variantHandler(Production(true), Logger(slog.New(slog.NewTextHandler(&logs, nil))))
	do(prod, http.MethodPost, "/get", `{"id":3}`, nil)
	if strings.Contains(logs.String(), "undeclared") {
		t.Fatal("production must not warn")
	}
}

func TestErrorsPanicsOnNonStruct(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "named struct") {
			t.Fatalf("got %v", r)
		}
	}()
	Errors(codedString("x"))
}

type codedString string

func (codedString) Error() string { return "x" }
func (codedString) Code() Code    { return Unknown }

func TestVariantsDoNotAffectPlainErrors(t *testing.T) {
	h := variantHandler(Logger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	rec := do(h, http.MethodPost, "/get", `{"id":9}`, nil)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
