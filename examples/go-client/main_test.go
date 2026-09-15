package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/ledger/api"
	"github.com/bowlinedev/bowline/examples/ledger/ledgerclient"
)

func ledgerServer(t *testing.T) *httptest.Server {
	t.Helper()
	t.Setenv("LEDGER_TOKEN", "")
	srv := httptest.NewServer(http.StripPrefix("/api", api.Routes().Handler()))
	t.Cleanup(srv.Close)
	return srv
}

func call(h http.Handler, path, input string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/"+path, strings.NewReader(input))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestOutstandingSumsUnpaidInvoices(t *testing.T) {
	ledger := ledgerServer(t)
	reports := Routes(NewLedgerClient(ledger.URL+"/api", "")).Handler()
	rec := call(reports, "reports.outstanding", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var out Outstanding
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Count != 1 || out.Total != "1500.00" || out.Currency != "USD" {
		t.Fatalf("outstanding %+v", out)
	}
	filtered := call(reports, "reports.outstanding", `{"customerId":2}`)
	if !strings.Contains(filtered.Body.String(), `"count":0`) {
		t.Fatalf("filtered %s", filtered.Body.String())
	}
}

func TestCustomerRemapsNotFound(t *testing.T) {
	ledger := ledgerServer(t)
	reports := Routes(NewLedgerClient(ledger.URL+"/api", "")).Handler()
	rec := call(reports, "reports.customer", `{"id":999}`)
	if rec.Code != http.StatusPreconditionFailed || !strings.Contains(rec.Body.String(), `"FAILED_PRECONDITION"`) || !strings.Contains(rec.Body.String(), "customer 999") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	ok := call(reports, "reports.customer", `{"id":1}`)
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "Ada Lovelace") {
		t.Fatalf("status %d body %s", ok.Code, ok.Body.String())
	}
}

func TestGeneratedClientAgainstLedger(t *testing.T) {
	ledger := ledgerServer(t)
	client := ledgerclient.New(ledger.URL + "/api")
	ctx := context.Background()
	invoice, err := client.Invoices.Get(ctx, ledgerclient.GetInvoiceInput{ID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if invoice.ID != 3 || invoice.Status != ledgerclient.StatusSent || invoice.CreatedAt.IsZero() || invoice.Total != "USD 1500.00" {
		t.Fatalf("invoice %+v", invoice)
	}
	_, err = client.Invoices.Get(ctx, ledgerclient.GetInvoiceInput{ID: 999})
	var be *bowline.Error
	if !errors.As(err, &be) || be.Code != bowline.NotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
	_, err = client.Invoices.Void(ctx, ledgerclient.VoidInvoiceInput{ID: 4})
	if ledgerclient.VariantOf(err) != "InvoiceLocked" {
		t.Fatalf("expected InvoiceLocked, got %v", err)
	}
	details, ok := ledgerclient.DetailsAs[ledgerclient.InvoiceLocked](err)
	if !ok || details.ID != 4 || details.Status != ledgerclient.StatusPaid {
		t.Fatalf("details %+v %v", details, ok)
	}
}

func TestParseMoney(t *testing.T) {
	for in, want := range map[string]int64{"USD 1500.00": 150000, "EUR 0.05": 5, "USD 7": 700, "USD -1.50": -150} {
		currency, cents, err := parseMoney(in)
		if err != nil || cents != want || currency == "" {
			t.Fatalf("%s: %s %d %v", in, currency, cents, err)
		}
	}
	if _, _, err := parseMoney("1500"); err == nil {
		t.Fatal("expected an error")
	}
}
