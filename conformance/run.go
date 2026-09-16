package conformance

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bowlinedev/bowline"
)

func Options() []bowline.HandlerOption {
	return []bowline.HandlerOption{bowline.MaxBodySize(bodyLimit)}
}

func Run(t *testing.T, mount func(h http.Handler) http.Handler) {
	t.Helper()
	handler := mount(Router().Handler(Options()...))
	srv := httptest.NewServer(handler)
	defer srv.Close()
	RunURL(t, srv.URL+"/api")
}

func RunURL(t *testing.T, baseURL string) {
	t.Helper()
	base := strings.TrimRight(baseURL, "/")
	client := &http.Client{Timeout: 10 * time.Second}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if err := execute(client, base, tc); err != nil {
				t.Error(err)
			}
		})
	}
}

func execute(client *http.Client, base string, tc testCase) error {
	target := base + "/" + tc.Path
	var req *http.Request
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if tc.Method == http.MethodGet {
		if tc.Body != "" {
			target += "?input=" + url.QueryEscape(tc.Body)
		}
		req, err = http.NewRequestWithContext(ctx, tc.Method, target, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, tc.Method, target, strings.NewReader(tc.Body))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	if err != nil {
		return err
	}
	for k, v := range tc.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", tc.Method, target, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%s %s: reading body: %w", tc.Method, target, err)
	}
	if resp.StatusCode != tc.Status {
		return fmt.Errorf("%s %s: status %d, want %d\nresponse: %s", tc.Method, target, resp.StatusCode, tc.Status, truncate(body))
	}
	if tc.Check != nil {
		if err := tc.Check(resp, body); err != nil {
			return fmt.Errorf("%s %s: %w\nresponse headers: %v", tc.Method, target, err, resp.Header)
		}
	}
	return nil
}

func truncate(b []byte) string {
	if len(b) > 512 {
		return string(b[:512]) + "…"
	}
	return string(b)
}
