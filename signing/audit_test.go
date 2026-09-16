package signing

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func replace(header, field, value string) string {
	parts := strings.Split(header, ",")
	for i, part := range parts {
		if strings.HasPrefix(part, field+"=") {
			parts[i] = field + "=" + value
		}
	}
	return strings.Join(parts, ",")
}

func signatureOf(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		if value, ok := strings.CutPrefix(part, "sig="); ok {
			return value
		}
	}
	return ""
}

func TestSignatureComparisonIsConstantTime(t *testing.T) {
	path := "/api/invoices.create"
	body := []byte(`{"customerId":1}`)
	header := Sign("POST", path, body, "billing-2026", secrets["billing-2026"], now)
	real := signatureOf(header)

	forged := []string{}
	for n := range len(real) {
		candidate := []byte(real)
		if candidate[n] == 'A' {
			candidate[n] = 'B'
		} else {
			candidate[n] = 'A'
		}
		forged = append(forged, string(candidate))
	}

	for _, candidate := range forged {
		want := subtle.ConstantTimeCompare([]byte(real), []byte(candidate)) == 1
		err := Verify(context.Background(), secrets, replace(header, "sig", candidate), "POST", path, body, now)
		got := err == nil
		if got != want {
			t.Fatalf("signature %q: accepted=%v, constant-time compare=%v (%v)", candidate, got, want, err)
		}
		if err != nil && !errors.Is(err, ErrSignatureMismatch) && !errors.Is(err, ErrMalformedSignature) {
			t.Fatalf("signature %q rejected with %v, which reveals more than a mismatch", candidate, err)
		}
	}

	if subtle.ConstantTimeCompare([]byte(real), []byte(real)) != 1 {
		t.Fatal("the real signature does not compare equal to itself")
	}
	if err := Verify(context.Background(), secrets, header, "POST", path, body, now); err != nil {
		t.Fatalf("the real signature was rejected: %v", err)
	}
}

func TestReplayWindowRejectsReuse(t *testing.T) {
	cache := NewReplayCache(16)
	path := "/api/invoices.create"
	body := []byte(`{"customerId":1}`)
	header := Sign("POST", path, body, "billing-2026", secrets["billing-2026"], now)

	if err := Verify(context.Background(), secrets, header, "POST", path, body, now, WithReplayCache(cache)); err != nil {
		t.Fatalf("first call: %v", err)
	}
	err := Verify(context.Background(), secrets, header, "POST", path, body, now.Add(time.Second), WithReplayCache(cache))
	if !errors.Is(err, ErrReplay) {
		t.Fatalf("replay returned %v, want %v", err, ErrReplay)
	}

	fresh := Sign("POST", path, body, "billing-2026", secrets["billing-2026"], now)
	if fresh == header {
		t.Fatal("two signatures of the same request are identical, so the nonce is not doing its job")
	}
	if err := Verify(context.Background(), secrets, fresh, "POST", path, body, now, WithReplayCache(cache)); err != nil {
		t.Fatalf("an identical request signed again was rejected: %v", err)
	}
}

func TestReplayWindowIsSkippedWithoutACache(t *testing.T) {
	path := "/api/ping"
	header := Sign("POST", path, nil, "billing-2026", secrets["billing-2026"], now)
	for i := range 3 {
		if err := Verify(context.Background(), secrets, header, "POST", path, nil, now); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestClockSkewWindow(t *testing.T) {
	path := "/api/ping"
	header := Sign("GET", path, nil, "billing-2026", secrets["billing-2026"], now)
	cases := map[string]struct {
		at       time.Time
		accepted bool
	}{
		"exactly at the signing time": {now, true},
		"at the past edge":            {now.Add(Skew), true},
		"one second past the edge":    {now.Add(Skew + time.Second), false},
		"at the future edge":          {now.Add(-Skew), true},
		"one second before the edge":  {now.Add(-Skew - time.Second), false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := Verify(context.Background(), secrets, header, "GET", path, nil, tc.at)
			if tc.accepted && err != nil {
				t.Fatalf("rejected: %v", err)
			}
			if !tc.accepted && !errors.Is(err, ErrSkew) {
				t.Fatalf("got %v, want %v", err, ErrSkew)
			}
		})
	}
}

func TestClockSkewRejectsTimestampsThatWouldOverflow(t *testing.T) {
	path := "/api/ping"
	header := Sign("GET", path, nil, "billing-2026", secrets["billing-2026"], now)
	for _, stamp := range []int64{-9223372036854775808, 9223372036854775807, -1, 0, 1 << 62} {
		t.Run(fmt.Sprint(stamp), func(t *testing.T) {
			err := Verify(context.Background(), secrets, replace(header, "t", fmt.Sprint(stamp)), "GET", path, nil, now)
			if !errors.Is(err, ErrSkew) {
				t.Fatalf("timestamp %d returned %v, want %v", stamp, err, ErrSkew)
			}
		})
	}
}

func TestReplayCacheIsBounded(t *testing.T) {
	const max = 64
	cache := NewReplayCache(max)
	path := "/api/ping"
	for i := range max * 40 {
		body := fmt.Appendf(nil, `{"n":%d}`, i)
		header := Sign("POST", path, body, "billing-2026", secrets["billing-2026"], now)
		if err := Verify(context.Background(), secrets, header, "POST", path, body, now, WithReplayCache(cache)); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if got := cache.size(); got > 2*max {
			t.Fatalf("after %d calls the cache holds %d entries, which is over the %d bound", i, got, 2*max)
		}
	}
}

func TestReplayCacheIsSafeUnderConcurrency(t *testing.T) {
	cache := NewReplayCache(128)
	path := "/api/ping"
	header := Sign("POST", path, nil, "billing-2026", secrets["billing-2026"], now)

	var mu sync.Mutex
	accepted := 0
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Verify(context.Background(), secrets, header, "POST", path, nil, now, WithReplayCache(cache))
			if err == nil {
				mu.Lock()
				accepted++
				mu.Unlock()
				return
			}
			if !errors.Is(err, ErrReplay) {
				t.Errorf("got %v, want %v", err, ErrReplay)
			}
		}()
	}
	wg.Wait()
	if accepted != 1 {
		t.Fatalf("%d goroutines were accepted, want exactly 1", accepted)
	}
}

func TestReplayCacheForgetsPastTheSkewWindow(t *testing.T) {
	cache := NewReplayCache(64)
	if !cache.observe("first", now) {
		t.Fatal("the first sighting was reported as a replay")
	}
	if cache.observe("first", now.Add(time.Second)) {
		t.Fatal("an immediate reuse was not caught")
	}
	if cache.observe("first", now.Add(Skew)) {
		t.Fatal("a reuse inside the window was not caught")
	}
	if !cache.observe("first", now.Add(2*Skew+time.Second)) {
		t.Fatal("the entry outlived two windows, so the cache never forgets")
	}
	if cache.size() > 2 {
		t.Fatalf("the cache holds %d entries after two rotations", cache.size())
	}
}

func TestAnOlderClientWithoutANonceStillVerifies(t *testing.T) {
	path := "/api/ping"
	body := []byte(`{"customerId":1}`)
	legacy := fmt.Sprintf("v1,t=%d,kid=%s,sig=%s", now.Unix(), "billing-2026",
		mac(secrets["billing-2026"], canonical("POST", path, body, now.Unix(), "")))
	if err := Verify(context.Background(), secrets, legacy, "POST", path, body, now); err != nil {
		t.Fatalf("a signature without a nonce was rejected: %v", err)
	}
}

func TestATamperedNonceIsRejected(t *testing.T) {
	path := "/api/ping"
	header := Sign("POST", path, nil, "billing-2026", secrets["billing-2026"], now)
	err := Verify(context.Background(), secrets, replace(header, "n", "AAAAAAAAAAAAAAAAAAAAAA"), "POST", path, nil, now)
	if !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("got %v, want %v", err, ErrSignatureMismatch)
	}
}

func TestADroppedNonceIsRejected(t *testing.T) {
	path := "/api/ping"
	header := Sign("POST", path, nil, "billing-2026", secrets["billing-2026"], now)
	stripped, _, _ := strings.Cut(header, ",n=")
	err := Verify(context.Background(), secrets, stripped, "POST", path, nil, now)
	if !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("got %v, want %v", err, ErrSignatureMismatch)
	}
}
