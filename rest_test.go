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

type listIn struct {
	Limit  int32    `json:"limit"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
	Deep   bool     `json:"deep"`
}

type delIn struct {
	ID    int64 `json:"id" validate:"required"`
	Force bool  `json:"force"`
}

type echoOut struct {
	Limit  int32    `json:"limit"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
	Deep   bool     `json:"deep"`
	ID     int64    `json:"id"`
	Force  bool     `json:"force"`
}

func restRouter() *bowline.Router {
	return bowline.NewRouter(
		bowline.Query("list", func(_ context.Context, in listIn) (echoOut, error) {
			return echoOut{Limit: in.Limit, Status: in.Status, Tags: in.Tags, Deep: in.Deep}, nil
		}, bowline.Path("invoices")),
		bowline.Mutation("remove", func(_ context.Context, in delIn) (echoOut, error) {
			return echoOut{ID: in.ID, Force: in.Force}, nil
		}, bowline.Path("invoices/{id}"), bowline.Method(http.MethodDelete)),
		bowline.Mutation("replace", func(_ context.Context, in delIn) (echoOut, error) {
			return echoOut{ID: in.ID, Force: in.Force}, nil
		}, bowline.Path("invoices/{id}"), bowline.Method(http.MethodPut)),
	)
}

func do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	restRouter().Handler().ServeHTTP(w, req)
	return w
}

func TestQueryStringFieldsBind(t *testing.T) {
	w := do(t, http.MethodGet, "/api/invoices?limit=20&status=paid&deep=true", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	var out echoOut
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Limit != 20 || out.Status != "paid" || !out.Deep {
		t.Fatalf("got %+v", out)
	}
}

func TestRepeatedQueryParamBecomesASlice(t *testing.T) {
	w := do(t, http.MethodGet, "/api/invoices?tags=a&tags=b", "")
	var out echoOut
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Tags) != 2 || out.Tags[0] != "a" || out.Tags[1] != "b" {
		t.Fatalf("tags %v", out.Tags)
	}
}

func TestUnknownQueryParamsAreIgnored(t *testing.T) {
	w := do(t, http.MethodGet, "/api/invoices?limit=5&utm_source=x", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
}

func TestBadQueryValueIsRejected(t *testing.T) {
	w := do(t, http.MethodGet, "/api/invoices?limit=abc", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
}

func TestDeleteMethodWithPathAndQuery(t *testing.T) {
	w := do(t, http.MethodDelete, "/api/invoices/7?force=true", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	var out echoOut
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.ID != 7 || !out.Force {
		t.Fatalf("got %+v", out)
	}
}

func TestPutMethodTakesABody(t *testing.T) {
	w := do(t, http.MethodPut, "/api/invoices/9", `{"force":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	var out echoOut
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.ID != 9 || !out.Force {
		t.Fatalf("got %+v", out)
	}
}

func TestWrongMethodOnACustomVerb(t *testing.T) {
	w := do(t, http.MethodPost, "/api/invoices/7", `{}`)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
}
