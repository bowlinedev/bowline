package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bowlinedev/bowline/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, Mount)
}

func TestSmoke(t *testing.T) {
	srv := httptest.NewServer(Mount(conformance.Router().Handler(conformance.Options()...)))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/echo?input=%7B%22message%22%3A%22smoke%22%7D")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	conformance.RunURL(t, srv.URL+"/api")
}
