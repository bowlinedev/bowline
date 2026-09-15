package gateway

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bowlinedev/bowline/contract"
)

func streamingGateway(t *testing.T, upstreamURL string) *httptest.Server {
	t.Helper()
	cfg := &Config{
		Prefix:         "/api",
		Timeout:        150 * time.Millisecond,
		ForwardHeaders: append([]string(nil), DefaultForwardHeaders...),
		Services:       map[string]Upstream{"ledger": {URL: upstreamURL + "/api", Version: "sha256:x", Retries: DefaultRetries}},
	}
	handler := testGateway(t, cfg, map[string]*contract.Document{"ledger": load(t, "ledger.contract.json")})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestStreamDeliversEventsAsTheyArrive(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		controller := http.NewResponseController(w)
		controller.Flush()
		for i := 1; i <= 3; i++ {
			fmt.Fprintf(w, "event: message\ndata: {\"n\":%d}\n\n", i)
			controller.Flush()
			time.Sleep(60 * time.Millisecond)
		}
		fmt.Fprint(w, "event: done\ndata: {}\n\n")
		controller.Flush()
	}))
	defer upstream.Close()
	gw := streamingGateway(t, upstream.URL)

	resp, err := http.Get(gw.URL + "/api/ledger.invoices.watch")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type %q", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	var arrivals []time.Duration
	start := time.Now()
	events := 0
	done := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			arrivals = append(arrivals, time.Since(start))
			events++
		}
		if line == "event: done" {
			done = true
		}
		if done && events >= 4 {
			break
		}
	}
	if events < 4 || !done {
		t.Fatalf("%d data lines, done=%v", events, done)
	}
	if arrivals[2]-arrivals[0] < 80*time.Millisecond {
		t.Fatalf("events arrived together, not streamed: %v", arrivals)
	}
}

func TestStreamIsExemptFromTheCallTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		controller := http.NewResponseController(w)
		for i := 0; i < 4; i++ {
			fmt.Fprintf(w, "event: message\ndata: {\"n\":%d}\n\n", i)
			controller.Flush()
			time.Sleep(90 * time.Millisecond)
		}
	}))
	defer upstream.Close()
	gw := streamingGateway(t, upstream.URL)

	resp, err := http.Get(gw.URL + "/api/ledger.invoices.watch")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), "data: ") != 4 {
		t.Fatalf("stream cut short by the call timeout: %q", body)
	}
}

func TestStreamPropagatesClientCancellation(t *testing.T) {
	var cancelled atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		controller := http.NewResponseController(w)
		fmt.Fprint(w, "event: message\ndata: {}\n\n")
		controller.Flush()
		select {
		case <-r.Context().Done():
			cancelled.Store(true)
		case <-time.After(5 * time.Second):
		}
	}))
	defer upstream.Close()
	gw := streamingGateway(t, upstream.URL)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, gw.URL+"/api/ledger.invoices.watch", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatal(err)
	}
	cancel()
	resp.Body.Close()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cancelled.Load() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the upstream request was not cancelled within a second")
}

func TestUploadStreamsWithoutBuffering(t *testing.T) {
	const size = 10 << 20
	var received atomic.Int64
	var contentLength atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentLength.Store(r.ContentLength)
		n, _ := io.Copy(io.Discard, r.Body)
		received.Store(n)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := &Config{
		Prefix:         "/api",
		Timeout:        30 * time.Second,
		ForwardHeaders: append([]string(nil), DefaultForwardHeaders...),
		Services:       map[string]Upstream{"ledger": {URL: upstream.URL + "/api", Version: "sha256:x"}},
	}
	handler := testGateway(t, cfg, map[string]*contract.Document{"ledger": load(t, "ledger.contract.json")})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	body := io.LimitReader(byteStream{}, size)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/ledger.invoices.attach", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if received.Load() != size {
		t.Fatalf("upstream received %d of %d bytes", received.Load(), size)
	}
	if contentLength.Load() != -1 {
		t.Fatalf("gateway buffered the upload and set Content-Length %d", contentLength.Load())
	}
}

type byteStream struct{}

func (byteStream) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}
