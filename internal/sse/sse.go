package sse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/bowlinedev/bowline/internal/codec"
)

func NewSink(w http.ResponseWriter, flusher http.Flusher, ctx context.Context, plan *codec.Plan) *Sink {
	return &Sink{w: w, flusher: flusher, ctx: ctx, plan: plan}
}

type Sink struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
	ctx     context.Context
	plan    *codec.Plan
	closed  bool
}

var ErrClosed = errors.New("bowline: stream closed")

func (s *Sink) Send(v any) error {
	data, err := EncodeMessage(s.ctx, s.plan, v)
	if err != nil {
		return err
	}
	return s.Write("message", data)
}

func EncodeMessage(ctx context.Context, plan *codec.Plan, v any) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, context.Canceled
	}
	normalized, err := plan.Normalize(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(normalized)
}

func (s *Sink) Write(event string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
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

func (s *Sink) Comment(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if _, err := s.w.Write([]byte(": " + text + "\n\n")); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *Sink) Close() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}
