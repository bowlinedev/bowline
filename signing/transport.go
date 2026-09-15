package signing

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

type Transport struct {
	Base   http.RoundTripper
	KeyID  string
	Secret []byte
	Now    func() time.Time
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		read, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
		body = read
	}
	signed := req.Clone(req.Context())
	if body != nil {
		signed.Body = io.NopCloser(bytes.NewReader(body))
		signed.ContentLength = int64(len(body))
		signed.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}
	now := time.Now
	if t.Now != nil {
		now = t.Now
	}
	signed.Header.Set(Header, Sign(req.Method, pathWithQuery(req), body, t.KeyID, t.Secret, now()))
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(signed)
}

func pathWithQuery(req *http.Request) string {
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if req.URL.RawQuery != "" {
		return path + "?" + req.URL.RawQuery
	}
	return path
}
