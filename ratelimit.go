package bowline

import (
	"container/list"
	"context"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type RateLimitOptions struct {
	Key     func(ctx context.Context) string
	Rate    float64
	Burst   int
	MaxKeys int
	Now     func() time.Time
}

func RateLimit(options RateLimitOptions) Middleware {
	if options.Key == nil {
		panic("bowline: RateLimit needs a Key function")
	}
	if options.Rate <= 0 {
		panic("bowline: RateLimit needs a positive Rate")
	}
	l := &limiter{
		key:     options.Key,
		rate:    options.Rate,
		burst:   float64(options.Burst),
		maxKeys: options.MaxKeys,
		now:     options.Now,
		buckets: map[string]*bucket{},
		order:   list.New(),
	}
	if options.Burst <= 0 {
		l.burst = math.Max(1, math.Ceil(options.Rate))
	}
	if l.maxKeys <= 0 {
		l.maxKeys = 10000
	}
	if l.now == nil {
		l.now = time.Now
	}
	return func(next Next) Next {
		return func(ctx context.Context, in any) (any, error) {
			key := l.key(ctx)
			if key == "" {
				return next(ctx, in)
			}
			wait, ok := l.allow(key)
			if ok {
				return next(ctx, in)
			}
			seconds := max(int(math.Ceil(wait.Seconds())), 1)
			if call := CallFrom(ctx); call != nil {
				call.ResponseHeader().Set("Retry-After", strconv.Itoa(seconds))
			}
			return nil, Errorf(ResourceExhausted, "rate limit exceeded; retry in %ds", seconds)
		}
	}
}

type bucket struct {
	key     string
	tokens  float64
	updated time.Time
	elem    *list.Element
}

type limiter struct {
	key     func(context.Context) string
	rate    float64
	burst   float64
	maxKeys int
	now     func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	order   *list.List
}

func (l *limiter) allow(key string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		for len(l.buckets) >= l.maxKeys {
			if !l.evict() {
				break
			}
		}
		b = &bucket{key: key, tokens: l.burst, updated: now}
		b.elem = l.order.PushFront(b)
		l.buckets[key] = b
	} else {
		if elapsed := now.Sub(b.updated); elapsed > 0 {
			b.tokens = math.Min(l.burst, b.tokens+elapsed.Seconds()*l.rate)
			b.updated = now
		}
		l.order.MoveToFront(b.elem)
	}
	if b.tokens < 1 {
		return time.Duration((1 - b.tokens) / l.rate * float64(time.Second)), false
	}
	b.tokens--
	return 0, true
}

func (l *limiter) evict() bool {
	back := l.order.Back()
	if back == nil {
		return false
	}
	l.order.Remove(back)
	delete(l.buckets, back.Value.(*bucket).key)
	return true
}

func applyResponseHeader(w http.ResponseWriter, ctx context.Context) {
	call := CallFrom(ctx)
	if call == nil || call.header == nil {
		return
	}
	header := w.Header()
	for name, values := range call.header {
		for _, value := range values {
			header.Add(name, value)
		}
	}
}
