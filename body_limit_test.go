package bowline

import (
	"net/http"
	"strings"
	"testing"
)

func TestPerProcedureBodyLimitOverridesDefault(t *testing.T) {
	h := NewRouter(
		Mutation("small", createUser, MaxBody(48)),
		Mutation("large", createUser, MaxBody(4096)),
		Mutation("inherits", createUser),
	).Handler(MaxBodySize(256), Logger(discardLogger()))
	body := func(n int) string { return `{"name":"` + strings.Repeat("a", n) + `"}` }
	cases := []struct {
		path   string
		body   string
		status int
	}{
		{"small", body(4), 200},
		{"small", body(128), 413},
		{"large", body(128), 200},
		{"large", body(8192), 413},
		{"inherits", body(128), 200},
		{"inherits", body(1024), 413},
	}
	for _, tc := range cases {
		t.Run(tc.path+"/"+string(rune('0'+len(tc.body)%10)), func(t *testing.T) {
			rec := do(h, http.MethodPost, "/api/"+tc.path, tc.body, nil)
			if rec.Code != tc.status {
				t.Fatalf("%s with %d bytes: status %d, want %d, body %s", tc.path, len(tc.body), rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestPerProcedureBodyLimitNamesItsOwnLimit(t *testing.T) {
	h := NewRouter(Mutation("small", createUser, MaxBody(48))).Handler(MaxBodySize(1<<20), Logger(discardLogger()))
	rec := do(h, http.MethodPost, "/api/small", `{"name":"`+strings.Repeat("a", 128)+`"}`, nil)
	if got := envelopeOf(t, rec).Message; got != "request body exceeds 48 bytes" {
		t.Fatalf("message %q", got)
	}
}

func TestZeroMaxBodyInheritsTheHandlerDefault(t *testing.T) {
	p := &Procedure{}
	h := &handler{maxBody: 512}
	if got := h.bodyLimit(p); got != 512 {
		t.Fatalf("limit %d, want 512", got)
	}
	p.MaxBody = 64
	if got := h.bodyLimit(p); got != 64 {
		t.Fatalf("limit %d, want 64", got)
	}
}

func TestUploadLimitAppliesToMultipart(t *testing.T) {
	h := NewRouter(Mount("invoices", NewRouter(
		Upload("attach", attach, MaxBody(512)),
		Upload("bulk", attach),
	))).Handler(MaxUploadSize(1<<20), Logger(discardLogger()))

	small, body := multipartBody(t, [][3]string{{"input", "", `{"invoiceId":1}`}, {"file", "a.txt", "hi"}})
	if rec := do(h, http.MethodPost, "/api/invoices.attach", body.String(), map[string]string{"Content-Type": small}); rec.Code != 200 {
		t.Fatalf("a small upload was rejected: %d %s", rec.Code, rec.Body.String())
	}

	ct, big := multipartBody(t, [][3]string{{"input", "", `{"invoiceId":1}`}, {"file", "a.txt", strings.Repeat("a", 4096)}})
	rec := do(h, http.MethodPost, "/api/invoices.attach", big.String(), map[string]string{"Content-Type": ct})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413, body %s", rec.Code, rec.Body.String())
	}
	if got := envelopeOf(t, rec).Message; got != "upload exceeds 512 bytes" {
		t.Fatalf("message %q", got)
	}

	ct, big = multipartBody(t, [][3]string{{"input", "", `{"invoiceId":1}`}, {"file", "a.txt", strings.Repeat("a", 4096)}})
	if rec := do(h, http.MethodPost, "/api/invoices.bulk", big.String(), map[string]string{"Content-Type": ct}); rec.Code != 200 {
		t.Fatalf("a procedure without MaxBody lost the handler default: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMaxBodyIsRecordedOnTheProcedure(t *testing.T) {
	r := NewRouter(Mutation("create", createUser, MaxBody(4096)))
	procs := r.Procedures()
	if len(procs) != 1 {
		t.Fatalf("procedures %d, want 1", len(procs))
	}
	if procs[0].MaxBody != 4096 {
		t.Fatalf("MaxBody %d, want 4096", procs[0].MaxBody)
	}
}

func TestSignedBodyLimitCoversProcedureLimits(t *testing.T) {
	h := NewRouter(Mutation("create", createUser, MaxBody(4<<20))).Handler(MaxBodySize(1024), Logger(discardLogger())).(*handler)
	if h.signedBody < 4<<20 {
		t.Fatalf("signing limit %d, want at least %d", h.signedBody, 4<<20)
	}
}
