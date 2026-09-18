package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/examples/ledger/ledger"
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
	req := httptest.NewRequest(http.MethodDelete, "/invoices/4", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 412 || !strings.Contains(rec.Body.String(), `"type":"InvoiceLocked"`) || !strings.Contains(rec.Body.String(), `"status":"paid"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestContractMatchesRouter(t *testing.T) {
	if err := Routes().Verify(Contract); err != nil {
		t.Fatal(err)
	}
}

func TestWatchStreamsChanges(t *testing.T) {
	a := New(ledger.NewStore(time.Now), slog.Default(), "")
	h := a.Router().Handler()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/invoices.watch", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	a.store.PutInvoice(ledger.Invoice{CustomerID: 1, Status: ledger.StatusDraft, Lines: []ledger.Line{{Description: "x", Quantity: 1, UnitPrice: ledger.Money{Cents: 100, Currency: "USD"}}}})
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if !strings.Contains(rec.Body.String(), "event: message") || !strings.Contains(rec.Body.String(), `"status":"draft"`) {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestAttachStoresMetadata(t *testing.T) {
	a := New(ledger.NewStore(time.Now), slog.Default(), "")
	h := a.Router().Handler()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, _ := mw.CreateFormField("input")
	io.WriteString(w, `{"invoiceId":3}`)
	fw, _ := mw.CreateFormFile("file", "receipt.pdf")
	io.WriteString(fw, strings.Repeat("x", 1024))
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/invoices.attach", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"size":1024`) || !strings.Contains(rec.Body.String(), `"name":"receipt.pdf"`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if got := a.store.Attachments(3); len(got) != 1 || got[0].Size != 1024 {
		t.Fatalf("attachments %+v", got)
	}
}
