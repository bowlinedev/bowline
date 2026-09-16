package signing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func FuzzVerify(f *testing.F) {
	valid := Sign("POST", "/api/invoices.create", []byte(`{"customerId":1}`), "billing-2026", secrets["billing-2026"], now)
	seeds := []string{
		"",
		" ",
		"v1",
		"v2,t=1,kid=a,sig=AA==",
		"v1,t=1,kid=a,sig=AA==",
		"v1,t=,kid=a,sig=AA==",
		"v1,t=1,kid=,sig=AA==",
		"v1,t=1,kid=a,sig=",
		"v1,t=1,kid=a,sig=AA==,n=",
		"v1,t=1,kid=a,sig=AA==,n=AA",
		"v1,t=1,kid=a,sig=AA==,n=AA,extra=1",
		"v1,t=1,t=2,kid=a,sig=AA==",
		"v1,t=99999999999999999999,kid=a,sig=AA==",
		"v1,t=-9223372036854775808,kid=a,sig=AA==",
		"v1,t=9223372036854775807,kid=a,sig=AA==",
		valid,
		strings.ToUpper(valid),
	}
	for _, seed := range seeds {
		f.Add(seed, "POST", "/api/invoices.create", []byte(`{"customerId":1}`))
	}

	f.Fuzz(func(t *testing.T, header, method, path string, body []byte) {
		err := Verify(context.Background(), secrets, header, method, path, body, now)
		if err == nil {
			return
		}
		known := []error{
			ErrMissingSignature, ErrMalformedSignature, ErrSkew,
			ErrUnknownKey, ErrSignatureMismatch, ErrReplay,
		}
		for _, sentinel := range known {
			if errors.Is(err, sentinel) {
				return
			}
		}
		t.Fatalf("Verify(%q) returned %v, which is not one of the documented sentinels", header, err)
	})
}

func FuzzReplayCache(f *testing.F) {
	f.Add("a", int64(0), 8)
	f.Add("", int64(1), 1)
	f.Fuzz(func(t *testing.T, key string, offset int64, max int) {
		if max < 1 || max > 4096 {
			return
		}
		if offset < -1<<40 || offset > 1<<40 {
			return
		}
		cache := NewReplayCache(max)
		at := now.Add(time.Duration(offset) * time.Second)
		if !cache.observe(key, at) {
			t.Fatalf("the first sighting of %q was reported as a replay", key)
		}
		if cache.observe(key, at) {
			t.Fatalf("an immediate reuse of %q was accepted", key)
		}
		if got := cache.Len(); got > 2*max {
			t.Fatalf("the cache holds %d entries, over the %d bound", got, 2*max)
		}
	})
}
