package signing

import (
	"bytes"
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
	if strings.Count(header, ",") != 4 {
		t.Fatalf("header %q", header)
	}
	if !strings.Contains(header, ",n=") {
		t.Fatalf("header %q carries no nonce", header)
	}
}

func TestEverySignatureCarriesAFreshNonce(t *testing.T) {
	seen := map[string]bool{}
	for range 64 {
		header := Sign("POST", "/api/ping", nil, "billing-2026", secrets["billing-2026"], now)
		_, nonce, ok := strings.Cut(header, ",n=")
		if !ok || nonce == "" {
			t.Fatalf("header %q carries no nonce", header)
		}
		if seen[nonce] {
			t.Fatalf("nonce %q was reused", nonce)
		}
		seen[nonce] = true
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
	a := canonical("POST", "/api/ping", nil, now.Unix(), "fixed-nonce")
	b := canonical("POST", "/api/ping", []byte{}, now.Unix(), "fixed-nonce")
	if !bytes.Equal(a, b) {
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

func TestCanonicalStringDistinguishesEveryField(t *testing.T) {
	type request struct {
		method string
		path   string
		body   []byte
		ts     int64
		nonce  string
	}
	corpus := []request{
		{"POST", "/api/pay", []byte(`{"amount":1}`), 1000, "n1"},
		{"POST", "/api/pay", []byte(`{"amount":2}`), 1000, "n1"},
		{"GET", "/api/pay", []byte(`{"amount":1}`), 1000, "n1"},
		{"POST", "/api/refund", []byte(`{"amount":1}`), 1000, "n1"},
		{"POST", "/api/pay", []byte(`{"amount":1}`), 1001, "n1"},
		{"POST", "/api/pay", []byte(`{"amount":1}`), 1000, "n2"},
		{"POST", "/api/pay", []byte(`{"amount":1}`), 1000, ""},
		{"POST", "/api/pay\n/api/refund", nil, 1000, "n1"},
		{"POST", "/api/pay", nil, 1000, "n1"},
		{"POST", "/api", nil, 1000, "n1"},
		{"POST", "/api/pay?to=a&amount=1", nil, 1000, "n1"},
		{"POST", "/api/pay?to=a", nil, 1000, "n1"},
		{"POST", "/api/pay?to=a%26amount=1", nil, 1000, "n1"},
		{"POST", "/x", nil, 1000, "n1"},
		{"POS", "T/x", nil, 1000, "n1"},
		{"POST", "/x", nil, 1000, "1n"},
		{"POST", "/x", nil, 10001, "n"},
	}
	seen := map[string]request{}
	for _, r := range corpus {
		signed := string(canonical(r.method, r.path, r.body, r.ts, r.nonce))
		if previous, clash := seen[signed]; clash {
			t.Fatalf("%+v and %+v sign the same bytes; a signature for one would verify the other", previous, r)
		}
		seen[signed] = r
	}
}

func TestCanonicalStringIsCaseInsensitiveOnTheMethodOnly(t *testing.T) {
	upper := string(canonical("POST", "/api/pay", nil, 1000, "n1"))
	lower := string(canonical("post", "/api/pay", nil, 1000, "n1"))
	if upper != lower {
		t.Fatal("the method is uppercased before signing, so its case must not change the signed bytes")
	}
	if string(canonical("POST", "/API/PAY", nil, 1000, "n1")) == upper {
		t.Fatal("the path is case-sensitive and must change the signed bytes")
	}
}
