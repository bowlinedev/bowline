package bowline

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var fuzzMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodHead,
	http.MethodOptions,
}

var fuzzStatuses = func() map[int]bool {
	out := map[int]bool{
		http.StatusOK:                    true,
		http.StatusMethodNotAllowed:      true,
		http.StatusRequestEntityTooLarge: true,
		http.StatusUnsupportedMediaType:  true,
	}
	for _, status := range httpStatus {
		out[status] = true
	}
	return out
}()

func fuzzHandlerUnder(production bool) http.Handler {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	opts := []HandlerOption{Logger(quiet)}
	if production {
		opts = append(opts, Production(true))
	}
	return testHandler(opts...)
}

func FuzzHandler(f *testing.F) {
	seeds := []struct {
		method  string
		path    string
		body    string
		ctype   string
		accept  string
		produce bool
	}{
		{http.MethodGet, "/users.get?input=%7B%22id%22%3A1%7D", "", "", "", false},
		{http.MethodGet, "/users.get?input=%7B%22id%22%3A2%7D", "", "", "", true},
		{http.MethodGet, "/users.get?input=%7B%22id%22%3A3%7D", "", "", "", false},
		{http.MethodGet, "/users.get?input=%7B%22id%22%3A3%7D", "", "", "", true},
		{http.MethodPost, "/users.create", `{"id":1,"name":"a"}`, "application/json", "", false},
		{http.MethodPost, "/users.create", `{`, "application/json", "", false},
		{http.MethodPost, "/users.create", ``, "application/json", "", true},
		{http.MethodPost, "/users.create", `{"id":1}`, "text/plain", "", false},
		{http.MethodGet, "/health", "", "", "", false},
		{http.MethodGet, "/nope", "", "", "", false},
		{http.MethodDelete, "/users.get", "", "", "", false},
		{http.MethodGet, "/users.get", "", "", "text/event-stream", false},
		{http.MethodGet, "/.bowline/contract", "", "", "", false},
		{http.MethodPost, "/users.get", `{"id":1}`, "application/json", "", false},
		{http.MethodGet, "//users.get/", "", "", "", true},
	}
	for _, seed := range seeds {
		f.Add(seed.method, seed.path, seed.body, seed.ctype, seed.accept, seed.produce)
	}

	f.Fuzz(func(t *testing.T, method, path, body, contentType, accept string, production bool) {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		if strings.ContainsAny(path, " \t\r\n") {
			return
		}
		method = fuzzMethods[len(method)%len(fuzzMethods)]

		req, err := http.NewRequest(method, "http://fuzz.invalid"+path, strings.NewReader(body))
		if err != nil {
			return
		}
		if contentType != "" && !strings.ContainsAny(contentType, "\r\n\x00") {
			req.Header.Set("Content-Type", contentType)
		}
		if accept != "" && !strings.ContainsAny(accept, "\r\n\x00") {
			req.Header.Set("Accept", accept)
		}

		rec := httptest.NewRecorder()
		fuzzHandlerUnder(production).ServeHTTP(rec, req)

		if !fuzzStatuses[rec.Code] {
			t.Fatalf("%s %s produced status %d, which is not in the mapping table", method, path, rec.Code)
		}

		raw := rec.Body.Bytes()
		if len(raw) == 0 {
			return
		}
		if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "text/event-stream") {
			return
		}
		if !json.Valid(raw) {
			t.Fatalf("%s %s produced a body that is not JSON: %q", method, path, raw)
		}
		if rec.Code == http.StatusOK {
			return
		}

		var env wireEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("%s %s produced status %d with a body that is not an envelope: %q", method, path, rec.Code, raw)
		}
		if env.Error.Code == "" {
			t.Fatalf("%s %s produced status %d with no error code: %q", method, path, rec.Code, raw)
		}
		if _, ok := httpStatus[env.Error.Code]; !ok {
			t.Fatalf("%s %s produced the unknown code %q", method, path, env.Error.Code)
		}
		if env.Error.Message == "" {
			t.Fatalf("%s %s produced code %q with no message", method, path, env.Error.Code)
		}
		if production && strings.Contains(env.Error.Message, "kaboom") {
			t.Fatalf("%s %s leaked a panic value in production: %q", method, path, env.Error.Message)
		}
		if strings.Contains(env.Error.Message, "goroutine ") {
			t.Fatalf("%s %s leaked a stack trace: %q", method, path, env.Error.Message)
		}
	})
}
