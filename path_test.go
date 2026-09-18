package bowline_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
)

type getInvoiceIn struct {
	ID int64 `json:"id" validate:"required"`
}

type listInvoicesIn struct {
	Limit int32 `json:"limit"`
}

type createInvoiceIn struct {
	Total string `json:"total" validate:"required"`
}

type invoiceOut struct {
	ID    int64  `json:"id"`
	Total string `json:"total"`
	Limit int32  `json:"limit"`
}

type lineIn struct {
	InvoiceID int64  `json:"invoiceId"`
	LineID    string `json:"lineId"`
}

func pathRouter() *bowline.Router {
	return bowline.NewRouter(
		bowline.Mount("invoices", bowline.NewRouter(
			bowline.Query("get", func(_ context.Context, in getInvoiceIn) (invoiceOut, error) {
				return invoiceOut{ID: in.ID, Total: "USD 1.00"}, nil
			}, bowline.Path("invoices/{id}")),
			bowline.Query("list", func(_ context.Context, in listInvoicesIn) (invoiceOut, error) {
				return invoiceOut{Limit: in.Limit}, nil
			}, bowline.Path("invoices")),
			bowline.Mutation("create", func(_ context.Context, in createInvoiceIn) (invoiceOut, error) {
				return invoiceOut{ID: 9, Total: in.Total}, nil
			}, bowline.Path("invoices")),
			bowline.Query("line", func(_ context.Context, in lineIn) (lineIn, error) {
				return in, nil
			}, bowline.Path("invoices/{invoiceId}/lines/{lineId}")),
		)),
	)
}

func serve(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	pathRouter().Handler().ServeHTTP(w, r)
	return w
}

func TestRestPathBindsTheParameter(t *testing.T) {
	w := serve(t, http.MethodGet, "/api/invoices/42", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	var out invoiceOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != 42 {
		t.Fatalf("id %d, want 42", out.ID)
	}
}

func TestRestPathWorksUnderAnyMountPrefix(t *testing.T) {
	for _, target := range []string{"/invoices/7", "/api/invoices/7", "/a/b/c/invoices/7"} {
		w := serve(t, http.MethodGet, target, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d body %s", target, w.Code, w.Body)
		}
	}
}

func TestCanonicalPathStillWorks(t *testing.T) {
	w := serve(t, http.MethodGet, `/api/invoices.get?input={"id":5}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	var out invoiceOut
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.ID != 5 {
		t.Fatalf("id %d, want 5", out.ID)
	}
}

func TestSamePathDifferentMethods(t *testing.T) {
	w := serve(t, http.MethodGet, `/api/invoices?input={"limit":20}`, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET status %d body %s", w.Code, w.Body)
	}
	var list invoiceOut
	json.Unmarshal(w.Body.Bytes(), &list)
	if list.Limit != 20 {
		t.Fatalf("limit %d, want 20", list.Limit)
	}

	w = serve(t, http.MethodPost, "/api/invoices", `{"total":"USD 3.00"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("POST status %d body %s", w.Code, w.Body)
	}
	var created invoiceOut
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.ID != 9 || created.Total != "USD 3.00" {
		t.Fatalf("created %+v", created)
	}
}

func TestSeveralParametersBind(t *testing.T) {
	w := serve(t, http.MethodGet, "/api/invoices/3/lines/abc", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	var out lineIn
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.InvoiceID != 3 || out.LineID != "abc" {
		t.Fatalf("got %+v", out)
	}
}

func TestValidationStillRunsAfterBinding(t *testing.T) {
	w := serve(t, http.MethodGet, "/api/invoices/0", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "is required") {
		t.Fatalf("body %s", w.Body)
	}
}

func TestNonNumericParameterIsRejected(t *testing.T) {
	w := serve(t, http.MethodGet, "/api/invoices/abc", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "whole number") {
		t.Fatalf("body %s", w.Body)
	}
}

func TestMethodNotAllowedOnARestPath(t *testing.T) {
	w := serve(t, http.MethodDelete, "/api/invoices/3", "")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	if allow := w.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow %q", allow)
	}
}

func TestUnknownRestPathIsUnimplemented(t *testing.T) {
	w := serve(t, http.MethodGet, "/api/nope/3", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
}

func TestAmbiguousPathsPanic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("two routes with the same shape on one method must panic")
		}
	}()
	bowline.NewRouter(
		bowline.Query("a", func(_ context.Context, in getInvoiceIn) (invoiceOut, error) { return invoiceOut{}, nil }, bowline.Path("things/{id}")),
		bowline.Query("b", func(_ context.Context, in getInvoiceIn) (invoiceOut, error) { return invoiceOut{}, nil }, bowline.Path("things/{key}")),
	).Handler()
}

func TestMalformedPathPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a malformed pattern must panic")
		}
	}()
	bowline.NewRouter(
		bowline.Query("a", func(_ context.Context, in getInvoiceIn) (invoiceOut, error) { return invoiceOut{}, nil }, bowline.Path("/leading")),
	).Handler()
}

type slugIn struct {
	Slug string `json:"slug"`
}

func TestEncodedPathSegment(t *testing.T) {
	r := bowline.NewRouter(
		bowline.Query("get", func(_ context.Context, in slugIn) (slugIn, error) { return in, nil },
			bowline.Path("docs/{slug}")),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/docs/a%2Fb", nil)
	w := httptest.NewRecorder()
	r.Handler().ServeHTTP(w, req)
	t.Logf("status=%d body=%s", w.Code, w.Body)
	if w.Code != http.StatusOK {
		t.Fatalf("a percent-encoded slash in a segment should still match: status %d", w.Code)
	}
	var out slugIn
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Slug != "a/b" {
		t.Fatalf("slug %q, want %q", out.Slug, "a/b")
	}
}
