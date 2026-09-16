package bowline

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/bowlinedev/bowline/internal/ratelimit"
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
	l := ratelimit.New(options.Rate, options.Burst, options.MaxKeys, options.Now)
	return func(next Next) Next {
		return func(ctx context.Context, in any) (any, error) {
			key := options.Key(ctx)
			if key == "" {
				return next(ctx, in)
			}
			wait, ok := l.Allow(key)
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
