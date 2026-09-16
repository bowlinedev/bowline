package conformance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/bowlinedev/bowline"
)

const bodyLimit = 1 << 16

type testCase struct {
	Name    string
	Method  string
	Path    string
	Body    string
	Headers map[string]string
	Status  int
	Check   func(resp *http.Response, body []byte) error
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Issues  []struct {
			Path    []string `json:"path"`
			Rule    string   `json:"rule"`
			Message string   `json:"message"`
		} `json:"issues"`
	} `json:"error"`
}

func expectContentType(resp *http.Response, want string) error {
	if got := resp.Header.Get("Content-Type"); got != want {
		return fmt.Errorf("content type %q, want %q", got, want)
	}
	return nil
}

func expectError(body []byte, code string) (*errorEnvelope, error) {
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("error body is not an envelope: %s", body)
	}
	if env.Error.Code != code {
		return nil, fmt.Errorf("error code %q, want %q (body %s)", env.Error.Code, code, body)
	}
	return &env, nil
}

func expectFields(body []byte, want map[string]any) error {
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		return fmt.Errorf("body is not an object: %s", body)
	}
	for k, v := range want {
		if fmt.Sprint(got[k]) != fmt.Sprint(v) {
			return fmt.Errorf("field %s = %v, want %v (body %s)", k, got[k], v, body)
		}
	}
	return nil
}

var cases = buildCases()

func buildCases() []testCase {
	list := []testCase{
		{
			Name: "success-body", Method: http.MethodGet, Path: "echo", Body: `{"message":"hi","count":2}`, Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				if err := expectContentType(resp, "application/json; charset=utf-8"); err != nil {
					return err
				}
				return expectFields(body, map[string]any{"message": "hi", "count": 2})
			},
		},
		{
			Name: "get-without-input", Method: http.MethodGet, Path: "echo", Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				return expectFields(body, map[string]any{"message": "", "count": 0})
			},
		},
		{
			Name: "post-on-query", Method: http.MethodPost, Path: "echo", Body: `{"message":"posted"}`, Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				return expectFields(body, map[string]any{"message": "posted"})
			},
		},
		{
			Name: "get-on-mutation", Method: http.MethodGet, Path: "create", Body: `{"name":"x"}`, Status: 405,
			Check: func(resp *http.Response, body []byte) error {
				if resp.Header.Get("Allow") != "POST" {
					return fmt.Errorf("allow header %q, want POST", resp.Header.Get("Allow"))
				}
				_, err := expectError(body, "INVALID_ARGUMENT")
				return err
			},
		},
		{
			Name: "get-on-sensitive", Method: http.MethodGet, Path: "sensitive", Body: `{"message":"secret"}`, Status: 405,
			Check: func(resp *http.Response, body []byte) error {
				if resp.Header.Get("Allow") != "POST" {
					return fmt.Errorf("allow header %q, want POST", resp.Header.Get("Allow"))
				}
				return nil
			},
		},
		{
			Name: "post-on-sensitive", Method: http.MethodPost, Path: "sensitive", Body: `{"message":"secret"}`, Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				return expectFields(body, map[string]any{"message": "secret"})
			},
		},
		{
			Name: "unsupported-media-type", Method: http.MethodPost, Path: "create", Body: `{"name":"x"}`, Headers: map[string]string{"Content-Type": "text/plain"}, Status: 415,
			Check: func(resp *http.Response, body []byte) error {
				_, err := expectError(body, "INVALID_ARGUMENT")
				return err
			},
		},
		{
			Name: "malformed-json", Method: http.MethodPost, Path: "create", Body: `{"name":`, Status: 400,
			Check: func(resp *http.Response, body []byte) error {
				_, err := expectError(body, "INVALID_ARGUMENT")
				return err
			},
		},
		{
			Name: "unknown-procedure", Method: http.MethodGet, Path: "nope", Status: 404,
			Check: func(resp *http.Response, body []byte) error {
				_, err := expectError(body, "UNIMPLEMENTED")
				return err
			},
		},
		{
			Name: "trailing-slash", Method: http.MethodGet, Path: "echo/", Body: `{"message":"slash"}`, Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				return expectFields(body, map[string]any{"message": "slash"})
			},
		},
		{
			Name: "validation-issues", Method: http.MethodPost, Path: "validate", Body: `{"name":"","age":12,"tags":["a"],"kind":"other","email":"nope","website":"nope","token":"nope"}`, Status: 400,
			Check: func(resp *http.Response, body []byte) error {
				env, err := expectError(body, "INVALID_ARGUMENT")
				if err != nil {
					return err
				}
				want := []string{"name required", "age min", "tags len", "kind oneof", "email email", "website url", "token uuid"}
				if len(env.Error.Issues) != len(want) {
					return fmt.Errorf("%d issues, want %d: %s", len(env.Error.Issues), len(want), body)
				}
				for i, issue := range env.Error.Issues {
					got := strings.Join(issue.Path, ".") + " " + issue.Rule
					if got != want[i] || issue.Message == "" {
						return fmt.Errorf("issue %d is %q (%q), want %q", i, got, issue.Message, want[i])
					}
				}
				return nil
			},
		},
		{
			Name: "body-limit", Method: http.MethodPost, Path: "create", Body: `{"name":"` + strings.Repeat("x", bodyLimit) + `"}`, Status: 413,
			Check: func(resp *http.Response, body []byte) error {
				_, err := expectError(body, "INVALID_ARGUMENT")
				return err
			},
		},
		{
			Name: "panic-is-internal", Method: http.MethodGet, Path: "panic", Status: 500,
			Check: func(resp *http.Response, body []byte) error {
				_, err := expectError(body, "INTERNAL")
				return err
			},
		},
		{
			Name: "survives-panic", Method: http.MethodGet, Path: "echo", Body: `{"message":"after"}`, Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				return expectFields(body, map[string]any{"message": "after"})
			},
		},
		{
			Name: "deprecation-header", Method: http.MethodGet, Path: "old", Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				if resp.Header.Get("Deprecation") != "true" {
					return fmt.Errorf("deprecation header %q, want true", resp.Header.Get("Deprecation"))
				}
				return nil
			},
		},
		{
			Name: "nested-path", Method: http.MethodGet, Path: "nested.deep.get", Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				return expectFields(body, map[string]any{"depth": 2})
			},
		},
		{
			Name: "wire-encodings", Method: http.MethodGet, Path: "echo", Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				for _, want := range []string{`"tags":[]`, `"Attrs":{}`, `"at":"2026-01-02T03:04:05Z"`, `"big":"9007199254740993"`} {
					if !bytes.Contains(body, []byte(want)) {
						return fmt.Errorf("body lacks %s: %s", want, body)
					}
				}
				return nil
			},
		},
		{
			Name: "subscription-stream", Method: http.MethodGet, Path: "ticks", Body: `{"count":3}`, Headers: map[string]string{"Accept": "text/event-stream"}, Status: 200,
			Check: func(resp *http.Response, body []byte) error {
				if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
					return fmt.Errorf("content type %q, want text/event-stream", ct)
				}
				events, err := parseEvents(body)
				if err != nil {
					return err
				}
				messages := 0
				done := false
				for _, e := range events {
					switch e.name {
					case "message":
						if err := expectFields([]byte(e.data), map[string]any{"n": messages + 1}); err != nil {
							return err
						}
						messages++
					case "done":
						done = true
					}
				}
				if messages != 3 || !done {
					return fmt.Errorf("stream had %d messages and done=%v: %s", messages, done, body)
				}
				return nil
			},
		},
		{
			Name: "subscription-needs-event-stream", Method: http.MethodGet, Path: "ticks", Status: 400,
			Check: func(resp *http.Response, body []byte) error {
				_, err := expectError(body, "INVALID_ARGUMENT")
				return err
			},
		},
	}
	uploadBody, uploadType := multipartBody(`{"label":"receipt"}`, "receipt.bin", []byte("hello"))
	list = append(list, testCase{
		Name: "upload", Method: http.MethodPost, Path: "upload", Body: uploadBody, Headers: map[string]string{"Content-Type": uploadType}, Status: 200,
		Check: func(resp *http.Response, body []byte) error {
			return expectFields(body, map[string]any{"label": "receipt", "name": "receipt.bin", "size": 5})
		},
	})
	missingBody, missingType := multipartBody(`{"label":"receipt"}`, "", nil)
	list = append(list, testCase{
		Name: "upload-missing-file", Method: http.MethodPost, Path: "upload", Body: missingBody, Headers: map[string]string{"Content-Type": missingType}, Status: 400,
		Check: func(resp *http.Response, body []byte) error {
			_, err := expectError(body, "INVALID_ARGUMENT")
			return err
		},
	})
	for _, code := range allCodes {
		list = append(list, testCase{
			Name: "code-" + strings.ToLower(string(code)), Method: http.MethodGet, Path: "fail", Body: fmt.Sprintf(`{"code":%q}`, code), Status: code.HTTPStatus(),
			Check: func(resp *http.Response, body []byte) error {
				_, err := expectError(body, string(code))
				return err
			},
		})
	}
	return list
}

var allCodes = []bowline.Code{
	bowline.Canceled, bowline.Unknown, bowline.InvalidArgument, bowline.DeadlineExceeded, bowline.NotFound,
	bowline.AlreadyExists, bowline.PermissionDenied, bowline.ResourceExhausted, bowline.FailedPrecondition,
	bowline.Aborted, bowline.OutOfRange, bowline.Unimplemented, bowline.Internal, bowline.Unavailable,
	bowline.DataLoss, bowline.Unauthenticated,
}

func multipartBody(input, fileName string, file []byte) (string, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreatePart(map[string][]string{"Content-Disposition": {`form-data; name="input"`}, "Content-Type": {"application/json"}})
	part.Write([]byte(input))
	if fileName != "" {
		fw, _ := w.CreateFormFile("file", fileName)
		fw.Write(file)
	}
	w.Close()
	return buf.String(), w.FormDataContentType()
}

type event struct {
	name string
	data string
}

func parseEvents(body []byte) ([]event, error) {
	var out []event
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	current := event{}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if current.name != "" || current.data != "" {
				out = append(out, current)
			}
			current = event{}
		case strings.HasPrefix(line, "event: "):
			current.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			current.data = strings.TrimPrefix(line, "data: ")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading the event stream: %w", err)
	}
	if current.name != "" || current.data != "" {
		out = append(out, current)
	}
	return out, nil
}
