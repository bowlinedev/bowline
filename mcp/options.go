package mcp

import (
	"log/slog"
	"slices"
	"time"

	"github.com/bowlinedev/bowline"
)

type Option func(*options)

type options struct {
	scopes   []string
	readOnly bool
	rate     *limiter
	forward  []string
	logger   *slog.Logger
	runtime  []bowline.HandlerOption
	clock    func() time.Time
}

func newOptions(opts []Option) *options {
	o := &options{forward: []string{"Authorization", "Cookie"}, logger: slog.Default(), clock: time.Now}
	for _, opt := range opts {
		opt(o)
	}
	if o.rate != nil {
		o.rate.now = o.clock
	}
	return o
}

func Scopes(names ...string) Option {
	return func(o *options) { o.scopes = append(o.scopes, names...) }
}

func ReadOnly() Option {
	return func(o *options) { o.readOnly = true }
}

func RateLimit(perMinute, burst int) Option {
	return func(o *options) { o.rate = newLimiter(perMinute, burst) }
}

func ForwardHeaders(names ...string) Option {
	return func(o *options) { o.forward = names }
}

func Logger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

func Runtime(opts ...bowline.HandlerOption) Option {
	return func(o *options) { o.runtime = append(o.runtime, opts...) }
}

func withClock(now func() time.Time) Option {
	return func(o *options) { o.clock = now }
}

func (o *options) visible(t Tool) bool {
	if o.readOnly && !t.ReadOnly {
		return false
	}
	if len(o.scopes) == 0 {
		return true
	}
	for _, want := range o.scopes {
		if slices.Contains(t.Scopes, want) {
			return true
		}
	}
	return false
}
