package signing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var (
	secrets = StaticSecrets{"billing-2026": []byte("shared secret")}
	now     = time.Unix(1762084800, 0)
)

func TestSignProducesTheDocumentedHeader(t *testing.T) {
	header := Sign("GET", "/api/invoices.get?input=%7B%22id%22%3A3%7D", nil, "billing-2026", secrets["billing-2026"], now)
	if !strings.HasPrefix(header, "v1,t=1762084800,kid=billing-2026,sig=") {
		t.Fatalf("header %q", header)
	}
	if strings.Count(header, ",") != 3 {
		t.Fatalf("header %q", header)
	}
}

func TestVerifyAcceptsAGetWithAQueryString(t *testing.T) {
	path := "/api/invoices.get?input=%7B%22id%22%3A3%7D"
	header := Sign("GET", path, nil, "billing-2026", secrets["billing-2026"], now)
	if err := Verify(context.Background(), secrets, header, "GET", path, nil, now.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyAcceptsAPostWithABody(t *testing.T) {
	body := []byte(`{"customerId":1}`)
	header := Sign("POST", "/api/invoices.create", body, "billing-2026", secrets["billing-2026"], now)
	if err := Verify(context.Background(), secrets, header, "POST", "/api/invoices.create", body, now); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRejectsTampering(t *testing.T) {
	body := []byte(`{"customerId":1}`)
	path := "/api/invoices.create"
	header := Sign("POST", path, body, "billing-2026", secrets["billing-2026"], now)
	cases := map[string]struct {
		method string
		path   string
		body   []byte
	}{
		"tampered body":   {"POST", path, []byte(`{"customerId":2}`)},
		"tampered path":   {"POST", "/api/invoices.void", body},
		"tampered method": {"GET", path, body},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := Verify(context.Background(), secrets, header, c.method, c.path, c.body, now)
			if !errors.Is(err, ErrSignatureMismatch) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestVerifyRejectsATamperedTimestamp(t *testing.T) {
	header := Sign("POST", "/api/ping", nil, "billing-2026", secrets["billing-2026"], now)
	moved := strings.Replace(header, "t=1762084800", "t=1762084801", 1)
	if err := Verify(context.Background(), secrets, moved, "POST", "/api/ping", nil, now); !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyRejectsSkewBeyondTheWindow(t *testing.T) {
	header := Sign("POST", "/api/ping", nil, "billing-2026", secrets["billing-2026"], now)
	for _, drift := range []time.Duration{301 * time.Second, -301 * time.Second} {
		if err := Verify(context.Background(), secrets, header, "POST", "/api/ping", nil, now.Add(drift)); !errors.Is(err, ErrSkew) {
			t.Fatalf("drift %s: got %v", drift, err)
		}
	}
	for _, drift := range []time.Duration{300 * time.Second, -300 * time.Second} {
		if err := Verify(context.Background(), secrets, header, "POST", "/api/ping", nil, now.Add(drift)); err != nil {
			t.Fatalf("drift %s: got %v", drift, err)
		}
	}
}

func TestVerifyRejectsAnUnknownKey(t *testing.T) {
	header := Sign("POST", "/api/ping", nil, "ledger-2026", []byte("other"), now)
	err := Verify(context.Background(), secrets, header, "POST", "/api/ping", nil, now)
	if !errors.Is(err, ErrUnknownKey) || !strings.Contains(err.Error(), "ledger-2026") {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyRejectsMissingAndMalformedHeaders(t *testing.T) {
	cases := map[string]struct {
		header string
		want   error
	}{
		"empty":            {"", ErrMissingSignature},
		"blank":            {"   ", ErrMissingSignature},
		"wrong version":    {"v2,t=1762084800,kid=billing-2026,sig=abc", ErrMalformedSignature},
		"too few parts":    {"v1,t=1762084800,kid=billing-2026", ErrMalformedSignature},
		"no timestamp":     {"v1,x=1762084800,kid=billing-2026,sig=abc", ErrMalformedSignature},
		"bad timestamp":    {"v1,t=soon,kid=billing-2026,sig=abc", ErrMalformedSignature},
		"no key":           {"v1,t=1762084800,x=billing-2026,sig=abc", ErrMalformedSignature},
		"no signature":     {"v1,t=1762084800,kid=billing-2026,x=abc", ErrMalformedSignature},
		"not base64":       {"v1,t=1762084800,kid=billing-2026,sig=!!!", ErrMalformedSignature},
		"empty value":      {"v1,t=1762084800,kid=,sig=abc", ErrMalformedSignature},
		"duplicate fields": {"v1,t=1762084800,t=1762084801,sig=abc", ErrMalformedSignature},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Verify(context.Background(), secrets, c.header, "POST", "/api/ping", nil, now); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestVerifyWithoutAProviderRejects(t *testing.T) {
	header := Sign("POST", "/api/ping", nil, "billing-2026", secrets["billing-2026"], now)
	if err := Verify(context.Background(), nil, header, "POST", "/api/ping", nil, now); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("got %v", err)
	}
}

func TestAnEmptyBodyAndANilBodyHashTheSame(t *testing.T) {
	a := Sign("POST", "/api/ping", nil, "billing-2026", secrets["billing-2026"], now)
	b := Sign("POST", "/api/ping", []byte{}, "billing-2026", secrets["billing-2026"], now)
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
}

type failingProvider struct{}

func (failingProvider) Secret(ctx context.Context, keyID string) ([]byte, error) {
	return nil, errors.New("vault unreachable")
}

func TestVerifyPropagatesProviderFailures(t *testing.T) {
	header := Sign("POST", "/api/ping", nil, "billing-2026", secrets["billing-2026"], now)
	err := Verify(context.Background(), failingProvider{}, header, "POST", "/api/ping", nil, now)
	if err == nil || !strings.Contains(err.Error(), "vault unreachable") {
		t.Fatalf("got %v", err)
	}
}
