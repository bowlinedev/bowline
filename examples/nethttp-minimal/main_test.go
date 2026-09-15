package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGreet(t *testing.T) {
	rec := httptest.NewRecorder()
	Routes().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/greet?input=%7B%22name%22%3A%22ada%22%7D", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "hello, ada") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}
