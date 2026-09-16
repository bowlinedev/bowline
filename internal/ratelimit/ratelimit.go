package ratelimit

import (
	"container/list"
	"math"
	"sync"
	"time"
)

const defaultMaxKeys = 10000

type bucket struct {
	key     string
	tokens  float64
	updated time.Time
	elem    *list.Element
}

type Limiter struct {
	rate    float64
	burst   float64
	maxKeys int
	now     func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	order   *list.List
}

func New(rate float64, burst int, maxKeys int, now func() time.Time) *Limiter {
	l := &Limiter{
		rate:    rate,
		burst:   float64(burst),
		maxKeys: maxKeys,
		now:     now,
		buckets: map[string]*bucket{},
		order:   list.New(),
	}
	if burst <= 0 {
		l.burst = math.Max(1, math.Ceil(rate))
	}
	if l.maxKeys <= 0 {
		l.maxKeys = defaultMaxKeys
	}
	if l.now == nil {
		l.now = time.Now
	}
	return l
}

func (l *Limiter) Allow(key string) (time.Duration, bool) {
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

func (l *Limiter) evict() bool {
	back := l.order.Back()
	if back == nil {
		return false
	}
	l.order.Remove(back)
	delete(l.buckets, back.Value.(*bucket).key)
	return true
}
