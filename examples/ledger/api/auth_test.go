package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/examples/ledger/ledger"
)

func TestRequireTokenGuardsInvoices(t *testing.T) {
	a := New(ledger.NewStore(time.Now), slog.Default(), "secret")
	h := a.Router().Handler()
	call := func(auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/invoices.get", strings.NewReader(`{"id":3}`))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := call(""); rec.Code != 401 || !strings.Contains(rec.Body.String(), `"UNAUTHENTICATED"`) {
		t.Fatalf("no header: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call("Bearer wrong"); rec.Code != 401 {
		t.Fatalf("wrong token: %d", rec.Code)
	}
	if rec := call("Bearer secret"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"id":3`) {
		t.Fatalf("right token: %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/customers.get?input=%7B%22id%22%3A1%7D", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("customers must stay open: %d %s", rec.Code, rec.Body.String())
	}
}

func TestClockUsesFixedTime(t *testing.T) {
	t.Setenv("LEDGER_FIXED_TIME", "2026-01-02T03:04:05Z")
	if got := Clock()(); got.Format(time.RFC3339) != "2026-01-02T03:04:05Z" {
		t.Fatalf("got %s", got)
	}
	t.Setenv("LEDGER_FIXED_TIME", "")
	if Clock()().Sub(time.Now()) > time.Second {
		t.Fatal("expected wall clock")
	}
}
