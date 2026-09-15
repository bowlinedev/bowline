package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/examples/federation/billing/billing"
	ledgerapi "github.com/bowlinedev/bowline/examples/ledger/api"
	"github.com/bowlinedev/bowline/signing"
)

const key = "billing-2026"

var secret = []byte("shared-secret")

func ledgerServer(t *testing.T, signed bool) *httptest.Server {
	t.Helper()
	options := []bowline.HandlerOption{bowline.WithContract(ledgerapi.Contract)}
	if signed {
		options = append(options, bowline.Signed(signing.StaticSecrets{key: secret}))
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", ledgerapi.Routes().Handler(options...)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func billingHandler(t *testing.T, ledgerURL string, keyID string, key []byte) http.Handler {
	t.Helper()
	a := New(billing.NewStore(func() time.Time { return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC) }), LedgerClient(ledgerURL, keyID, key), slog.Default())
	return a.Router().Handler()
}

func call(h http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestChargingAnInvoiceReadsItFromTheLedgerOverASignedCall(t *testing.T) {
	ledger := ledgerServer(t, true)
	h := billingHandler(t, ledger.URL+"/api", key, secret)
	rec := call(h, "charges.create", `{"invoiceId":3}`)
	if rec.Code != 200 {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var charge billing.Charge
	json.Unmarshal(rec.Body.Bytes(), &charge)
	if charge.InvoiceID != 3 || charge.Amount != "USD 1500.00" || charge.Status != billing.StatusOpen {
		t.Fatalf("charge %+v", charge)
	}
	list := call(h, "charges.list", `{"limit":20}`)
	if !strings.Contains(list.Body.String(), `"invoiceId":3`) {
		t.Fatalf("list %s", list.Body.String())
	}
	settled := call(h, "charges.settle", `{"id":1}`)
	if !strings.Contains(settled.Body.String(), `"status":"settled"`) {
		t.Fatalf("settle %s", settled.Body.String())
	}
}

func TestAnUnsignedCallToASignedLedgerIsRejected(t *testing.T) {
	ledger := ledgerServer(t, true)
	h := billingHandler(t, ledger.URL+"/api", "", nil)
	rec := call(h, "charges.create", `{"invoiceId":3}`)
	if rec.Code != 401 || !strings.Contains(rec.Body.String(), "UNAUTHENTICATED") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestAMissingInvoiceBecomesADeclaredVariant(t *testing.T) {
	ledger := ledgerServer(t, false)
	h := billingHandler(t, ledger.URL+"/api", "", nil)
	rec := call(h, "charges.create", `{"invoiceId":999}`)
	if rec.Code != 412 || !strings.Contains(rec.Body.String(), `"type":"UnknownInvoice"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestContractMatchesTheRouter(t *testing.T) {
	if err := Routes().Verify(Contract); err != nil {
		t.Fatal(err)
	}
}
