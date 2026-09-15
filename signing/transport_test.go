package signing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTransportSignsWhatTheServerSees(t *testing.T) {
	var verified error
	var seenBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seenBody = string(body)
		path := r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		verified = Verify(r.Context(), secrets, r.Header.Get(Header), r.Method, path, body, time.Now())
	}))
	defer srv.Close()
	client := &http.Client{Transport: &Transport{KeyID: "billing-2026", Secret: secrets["billing-2026"]}}

	resp, err := client.Post(srv.URL+"/api/invoices.create", "application/json", strings.NewReader(`{"customerId":1}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if verified != nil {
		t.Fatalf("post: %v", verified)
	}
	if seenBody != `{"customerId":1}` {
		t.Fatalf("body %q", seenBody)
	}

	resp, err = client.Get(srv.URL + "/api/invoices.get?input=%7B%22id%22%3A3%7D")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if verified != nil {
		t.Fatalf("get: %v", verified)
	}
	if seenBody != "" {
		t.Fatalf("get body %q", seenBody)
	}
}

func TestTransportLeavesTheBodyReadableAfterARedirect(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		if r.URL.Path == "/first" {
			http.Redirect(w, r, "/second", http.StatusTemporaryRedirect)
		}
	}))
	defer srv.Close()
	client := &http.Client{Transport: &Transport{KeyID: "billing-2026", Secret: secrets["billing-2026"]}}
	resp, err := client.Post(srv.URL+"/first", "application/json", strings.NewReader(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(bodies) != 2 || bodies[0] != `{"a":1}` || bodies[1] != `{"a":1}` {
		t.Fatalf("bodies %q", bodies)
	}
}

func TestTransportUsesItsClock(t *testing.T) {
	var header string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get(Header)
	}))
	defer srv.Close()
	client := &http.Client{Transport: &Transport{
		KeyID:  "billing-2026",
		Secret: secrets["billing-2026"],
		Now:    func() time.Time { return now },
	}}
	resp, err := client.Get(srv.URL + "/api/ping")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !strings.Contains(header, "t=1762084800") {
		t.Fatalf("header %q", header)
	}
	if err := Verify(context.Background(), secrets, header, "GET", "/api/ping", nil, now); err != nil {
		t.Fatal(err)
	}
}
