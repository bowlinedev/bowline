package mcp

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

type limiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
	now     func() time.Time
}

func newLimiter(perMinute, burst int) *limiter {
	if burst < 1 {
		burst = 1
	}
	return &limiter{rate: float64(perMinute) / 60, burst: float64(burst), buckets: map[string]*bucket{}, now: time.Now}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= 4096 {
			l.evict(now)
		}
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *limiter) evict(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.last) > 10*time.Minute {
			delete(l.buckets, key)
		}
	}
}

func clientKey(authorization string) string {
	if authorization == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(authorization))
	return hex.EncodeToString(sum[:])
}
