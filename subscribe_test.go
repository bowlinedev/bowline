package bowline

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSubscribeDrivesAProcedureWithoutHTTP(t *testing.T) {
	r := NewRouter(Subscription("watch", watch))
	var frames []string
	err := r.Subscribe(context.Background(), "watch", []byte(`{"count":2}`), func(data []byte) error {
		frames = append(frames, string(data))
		return nil
	})
	var failure *StreamFailure
	if !errors.As(err, &failure) || failure.Status != 404 || !strings.Contains(string(failure.Body), `"NOT_FOUND"`) {
		t.Fatalf("got %v", err)
	}
	if len(frames) != 2 || !strings.Contains(frames[0], `"tags":[]`) {
		t.Fatalf("frames %v", frames)
	}
}

func TestSubscribeRejectsBadRequests(t *testing.T) {
	r := NewRouter(Subscription("watch", watch), Query("get", getUser))
	cases := map[string]struct {
		path   string
		input  string
		status int
	}{
		"unknown":       {"nope", `{}`, 404},
		"not a stream":  {"get", `{}`, 400},
		"invalid json":  {"watch", `{`, 400},
		"invalid input": {"watch", `{"count":0}`, 400},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := r.Subscribe(context.Background(), tc.path, []byte(tc.input), func([]byte) error { return nil })
			var failure *StreamFailure
			if !errors.As(err, &failure) || failure.Status != tc.status {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestSubscribeStopsWhenSendFails(t *testing.T) {
	r := NewRouter(Subscription("watch", watch))
	stop := errors.New("socket closed")
	err := r.Subscribe(context.Background(), "watch", []byte(`{"count":3}`), func([]byte) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("got %v", err)
	}
}
