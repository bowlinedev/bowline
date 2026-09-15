package contracttest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/contract"
)

type Option func(*options)

type options struct {
	headers  http.Header
	setup    func(t testing.TB)
	handler  http.Handler
	contract []byte
}

func WithHeaders(h http.Header) Option {
	return func(o *options) { o.headers = h }
}

func WithSetup(fn func(t testing.TB)) Option {
	return func(o *options) { o.setup = fn }
}

func WithHandler(h http.Handler) Option {
	return func(o *options) { o.handler = h }
}

func WithContract(doc []byte) Option {
	return func(o *options) { o.contract = doc }
}

func VerifyConsumers(t testing.TB, r *bowline.Router, dir string, opts ...Option) {
	t.Helper()
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	list, err := loadConsumers(dir)
	if err != nil {
		t.Fatalf("contracttest: loading consumer contracts from %s: %v", dir, err)
		return
	}
	if len(list) == 0 {
		t.Fatalf("contracttest: no consumer contracts under %s", dir)
		return
	}
	handler := o.handler
	if handler == nil {
		if r == nil {
			t.Fatalf("contracttest: a router or WithHandler is required")
			return
		}
		handler = r.Handler()
	}
	var doc *contract.Document
	if o.contract != nil {
		doc, err = contract.Parse(o.contract)
		if err != nil {
			t.Fatalf("contracttest: parsing contract: %v", err)
			return
		}
	} else {
		t.Logf("contracttest: no contract given; checking status and error codes only")
	}
	procs := map[string]*contract.Procedure{}
	if doc != nil {
		for _, p := range doc.Procedures {
			procs[p.Path] = p
		}
	}
	total, broken := 0, 0
	for _, c := range list {
		for _, in := range c.Interactions {
			total++
			if o.setup != nil {
				o.setup(t)
			}
			if !verify(t, handler, procs, doc, o.headers, c.Consumer, in) {
				broken++
			}
		}
	}
	t.Logf("contracttest: verified %d interactions from %d consumers", total, len(list))
	if broken > 0 {
		t.Errorf("contracttest: %d of %d interactions broke", broken, total)
	}
}

func verify(t testing.TB, handler http.Handler, procs map[string]*contract.Procedure, doc *contract.Document, headers http.Header, name string, in interaction) bool {
	t.Helper()
	label := fmt.Sprintf("consumer %s: %s", name, in.Procedure)
	proc := procs[in.Procedure]
	method := in.Method
	if method == "" {
		method = http.MethodPost
		if proc != nil && proc.Method == http.MethodGet {
			method = http.MethodGet
		}
	}
	input := in.Input
	if len(bytes.TrimSpace(input)) == 0 {
		input = json.RawMessage("{}")
	}
	if doc != nil && proc != nil {
		if decoded, err := decode(input); err == nil {
			rejected := false
			for _, issue := range validateInput(doc, proc.Input, decoded) {
				t.Errorf("consumer %s: %s → input.%s: input no longer accepted: %s", name, in.Procedure, strings.Join(issue.Path, "."), issue.Message)
				rejected = true
			}
			if rejected {
				return false
			}
		}
	}
	var req *http.Request
	if method == http.MethodGet {
		req = httptest.NewRequest(http.MethodGet, "/"+in.Procedure+"?input="+url.QueryEscape(string(input)), nil)
	} else {
		req = httptest.NewRequest(http.MethodPost, "/"+in.Procedure, bytes.NewReader(input))
		req.Header.Set("Content-Type", "application/json")
	}
	for k, values := range headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != in.Response.Status {
		t.Errorf("%s: status %d, recorded %d: %s", label, rec.Code, in.Response.Status, strings.TrimSpace(rec.Body.String()))
		return false
	}
	live, err := decode(rec.Body.Bytes())
	if err != nil {
		t.Errorf("%s: live response is not JSON: %v", label, err)
		return false
	}
	if in.Response.Status >= 400 {
		recorded, err := decode(in.Response.Body)
		if err != nil {
			t.Errorf("%s: recorded response is not JSON: %v", label, err)
			return false
		}
		return verifyError(t, label, recorded, live)
	}
	if doc == nil {
		return true
	}
	if proc == nil {
		t.Errorf("%s: procedure is not in the contract", label)
		return false
	}
	ok := true
	for _, m := range checkShape(doc, proc.Output, live) {
		where := in.Procedure
		if len(m.Path) > 0 {
			where += " → " + strings.Join(m.Path, ".")
		}
		t.Errorf("consumer %s: %s: %s", name, where, m.Reason)
		ok = false
	}
	return ok
}

func verifyError(t testing.TB, label string, recorded, live any) bool {
	t.Helper()
	wantCode, wantType := errorFields(recorded)
	gotCode, gotType := errorFields(live)
	ok := true
	if wantCode != gotCode {
		t.Errorf("%s: error code %q, recorded %q", label, gotCode, wantCode)
		ok = false
	}
	if wantType != "" && wantType != gotType {
		t.Errorf("%s: error type %q, recorded %q", label, gotType, wantType)
		ok = false
	}
	return ok
}

func errorFields(body any) (string, string) {
	obj, _ := body.(map[string]any)
	env, _ := obj["error"].(map[string]any)
	code, _ := env["code"].(string)
	typ, _ := env["type"].(string)
	return code, typ
}
