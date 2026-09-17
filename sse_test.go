package bowline

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type tick struct {
	N    int64     `json:"n"`
	At   time.Time `json:"at"`
	Tags []string  `json:"tags"`
}

type watchInput struct {
	Count int64 `json:"count" validate:"min=1"`
}

func watch(ctx context.Context, in watchInput, stream *Stream[tick]) error {
	for i := int64(1); i <= in.Count; i++ {
		if err := stream.Send(tick{N: i, At: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)}); err != nil {
			return err
		}
	}
	if in.Count == 2 {
		return Errorf(NotFound, "gone after %d", in.Count)
	}
	return nil
}

func subscriptionHandler(opts ...HandlerOption) http.Handler {
	return NewRouter(Subscription("watch", watch, Description("Watch ticks."))).Handler(opts...)
}

func streamRequest(h http.Handler, method, target string, accept string, ctx context.Context) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSubscriptionStreamsEvents(t *testing.T) {
	rec := streamRequest(subscriptionHandler(), http.MethodGet, "/watch?input=%7B%22count%22%3A3%7D", "text/event-stream", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/event-stream" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status %d headers %v", rec.Code, rec.Header())
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, ": open\n\n") || strings.Count(body, "event: message\n") != 3 || !strings.HasSuffix(body, "event: done\ndata: {}\n\n") {
		t.Fatalf("body %q", body)
	}
	if !strings.Contains(body, `data: {"n":1,"at":"2026-09-15T00:00:00Z","tags":[]}`) {
		t.Fatalf("first event not normalized: %q", body)
	}
}

func TestSubscriptionErrorEvent(t *testing.T) {
	rec := streamRequest(subscriptionHandler(), http.MethodGet, "/watch?input=%7B%22count%22%3A2%7D", "text/event-stream", nil)
	body := rec.Body.String()
	if strings.Count(body, "event: message\n") != 2 || !strings.Contains(body, "event: error\ndata: {\"error\":{\"code\":\"NOT_FOUND\",\"message\":\"gone after 2\"}}\n\n") || strings.Contains(body, "event: done") {
		t.Fatalf("body %q", body)
	}
}

func TestSubscriptionRequiresAccept(t *testing.T) {
	rec := streamRequest(subscriptionHandler(), http.MethodGet, "/watch?input=%7B%22count%22%3A1%7D", "", nil)
	if rec.Code != 400 || errorCode(t, rec) != InvalidArgument || !strings.Contains(rec.Body.String(), "text/event-stream") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	rec = streamRequest(subscriptionHandler(), http.MethodGet, "/watch?input=%7B%22count%22%3A1%7D", "*/*", nil)
	if rec.Code != 200 {
		t.Fatalf("wildcard accept: status %d", rec.Code)
	}
}

func TestSubscriptionValidatesInput(t *testing.T) {
	rec := streamRequest(subscriptionHandler(), http.MethodGet, "/watch?input=%7B%22count%22%3A0%7D", "text/event-stream", nil)
	if rec.Code != 400 || errorCode(t, rec) != InvalidArgument {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestSubscriptionStopsOnDisconnect(t *testing.T) {
	sendErrs := make(chan error, 1)
	forever := func(ctx context.Context, in struct{}, stream *Stream[tick]) error {
		for {
			if err := stream.Send(tick{}); err != nil {
				sendErrs <- err
				return err
			}
		}
	}
	h := NewRouter(Subscription("forever", forever)).Handler()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	done := make(chan struct{})
	go func() {
		streamRequest(h, http.MethodGet, "/forever", "text/event-stream", ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("subscription did not stop after cancellation")
	}
	if sendErr := <-sendErrs; !errors.Is(sendErr, context.Canceled) {
		t.Fatalf("Send returned %v, want context.Canceled", sendErr)
	}
}

func TestHeartbeat(t *testing.T) {
	slow := func(ctx context.Context, in struct{}, stream *Stream[tick]) error {
		time.Sleep(70 * time.Millisecond)
		return nil
	}
	h := NewRouter(Subscription("slow", slow)).Handler(Heartbeat(20 * time.Millisecond))
	rec := streamRequest(h, http.MethodGet, "/slow", "text/event-stream", nil)
	if strings.Count(rec.Body.String(), ": ping\n\n") < 2 {
		t.Fatalf("expected heartbeats, got %q", rec.Body.String())
	}
}

func TestSubscriptionIsAProcedure(t *testing.T) {
	procs := NewRouter(Subscription("watch", watch), Subscription("secret", watch, Sensitive())).Procedures()
	if procs[0].Kind != KindSubscription || procs[0].Method() != "GET" || procs[1].Method() != "POST" {
		t.Fatalf("%+v", procs)
	}
	req := httptest.NewRequest(http.MethodPost, "/watch", strings.NewReader(`{"count":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	subscriptionHandler().ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "event: done") {
		t.Fatalf("POST subscription status %d body %s", rec.Code, rec.Body.String())
	}
}
