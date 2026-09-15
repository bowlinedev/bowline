package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutesServeInvoices(t *testing.T) {
	h := Routes().Handler()
	req := httptest.NewRequest(http.MethodPost, "/invoices.list", strings.NewReader(`{"limit":10}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var page struct {
		Items []struct {
			ID    int64  `json:"id"`
			Total string `json:"total"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].Total != "USD 1500.00" {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestVoidPaidInvoiceFails(t *testing.T) {
	h := Routes().Handler()
	req := httptest.NewRequest(http.MethodPost, "/invoices.void", strings.NewReader(`{"id":4}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 412 || !strings.Contains(rec.Body.String(), "FAILED_PRECONDITION") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestContractMatchesRouter(t *testing.T) {
	if err := Routes().Verify(Contract); err != nil {
		t.Fatal(err)
	}
}
