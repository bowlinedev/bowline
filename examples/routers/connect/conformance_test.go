package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, Mount)
}

func TestSmoke(t *testing.T) {
	srv := httptest.NewServer(Mount(conformance.Router().Handler(conformance.Options()...)))
	defer srv.Close()
	conformance.RunURL(t, srv.URL+"/api")
}

func TestConnectCoexists(t *testing.T) {
	srv := httptest.NewServer(Mount(conformance.Router().Handler(conformance.Options()...)))
	defer srv.Close()
	resp, err := http.Post(srv.URL+PingProcedure, "application/json", strings.NewReader(`{"message":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"pong: hi"`) {
		t.Fatalf("connect status %d body %s", resp.StatusCode, body)
	}
	beside, err := http.Get(srv.URL + "/api/nested.deep.get")
	if err != nil {
		t.Fatal(err)
	}
	beside.Body.Close()
	if beside.StatusCode != http.StatusOK {
		t.Fatalf("bowline beside connect: %d", beside.StatusCode)
	}
}
