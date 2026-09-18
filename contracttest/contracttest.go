package contracttest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

func sendsBody(method string) bool {
	switch method {
	case http.MethodGet, http.MethodDelete, http.MethodHead:
		return false
	}
	return true
}

func resolve(proc *contract.Procedure, name, method string, input json.RawMessage) (string, io.Reader, error) {
	if proc == nil || proc.HTTPPath == "" {
		if !sendsBody(method) {
			return "/" + name + "?input=" + url.QueryEscape(string(input)), nil, nil
		}
		return "/" + name, bytes.NewReader(input), nil
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(input, &fields); err != nil {
		return "", nil, fmt.Errorf("input is not a JSON object: %w", err)
	}
	segments, err := contract.ParsePath(proc.HTTPPath)
	if err != nil {
		return "", nil, err
	}
	var path strings.Builder
	for i, seg := range segments {
		if i > 0 {
			path.WriteByte('/')
		}
		if !seg.Param {
			path.WriteString(seg.Text)
			continue
		}
		raw, ok := fields[seg.Text]
		if !ok {
			return "", nil, fmt.Errorf("path parameter %q is missing from the recorded input", seg.Text)
		}
		path.WriteString(url.PathEscape(literal(raw)))
		delete(fields, seg.Text)
	}
	target := "/" + path.String()
	if sendsBody(method) {
		rest, err := json.Marshal(fields)
		if err != nil {
			return "", nil, err
		}
		return target, bytes.NewReader(rest), nil
	}
	if query := values(fields).Encode(); query != "" {
		target += "?" + query
	}
	return target, nil, nil
}

func values(fields map[string]json.RawMessage) url.Values {
	out := make(url.Values, len(fields))
	for key, raw := range fields {
		var list []json.RawMessage
		if json.Unmarshal(raw, &list) == nil {
			for _, item := range list {
				out.Add(key, literal(item))
			}
			continue
		}
		if string(raw) == "null" {
			continue
		}
		out.Set(key, literal(raw))
	}
	return out
}

func literal(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
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
	if proc != nil && proc.Kind == "upload" {
		t.Logf("%s: upload interactions are verified statically only", label)
		return true
	}
	if doc != nil && proc != nil && in.Response.Status < 400 {
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
	target, body, err := resolve(proc, in.Procedure, method, input)
	if err != nil {
		t.Errorf("%s: %v", label, err)
		return false
	}
	req := httptest.NewRequest(method, target, body)
	if body != nil {
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
