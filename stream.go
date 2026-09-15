package bowline

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/bowlinedev/bowline/internal/codec"
)

type Stream[Out any] struct {
	emit func(v any) error
}

func (s *Stream[Out]) Send(v Out) error {
	return s.emit(v)
}

type eventSink struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
	ctx     context.Context
	plan    *codec.Plan
	closed  bool
}

var errStreamClosed = errors.New("bowline: stream closed")

func (s *eventSink) send(v any) error {
	data, err := encodeMessage(s.ctx, s.plan, v)
	if err != nil {
		return err
	}
	return s.write("message", data)
}

func encodeMessage(ctx context.Context, plan *codec.Plan, v any) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, context.Canceled
	}
	normalized, err := plan.Normalize(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func (s *eventSink) write(event string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errStreamClosed
	}
	if _, err := s.w.Write([]byte("event: " + event + "\ndata: ")); err != nil {
		return err
	}
	if _, err := s.w.Write(data); err != nil {
		return err
	}
	if _, err := s.w.Write([]byte("\n\n")); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *eventSink) comment(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errStreamClosed
	}
	if _, err := s.w.Write([]byte(": " + text + "\n\n")); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *eventSink) close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}
