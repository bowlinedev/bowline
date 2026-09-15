package signing

import (
	"sync"
	"time"
)

const DefaultReplayCacheSize = 8192

type ReplayCache struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	curr     map[string]struct{}
	prev     map[string]struct{}
	rotateAt time.Time
}

func NewReplayCache(max int) *ReplayCache {
	if max <= 0 {
		max = DefaultReplayCacheSize
	}
	return &ReplayCache{
		max:    max,
		window: Skew,
		curr:   map[string]struct{}{},
		prev:   map[string]struct{}{},
	}
}

func (c *ReplayCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.curr) + len(c.prev)
}

func (c *ReplayCache) observe(key string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rotateAt.IsZero() {
		c.rotateAt = now.Add(c.window)
	}
	if !now.Before(c.rotateAt) {
		c.rotate(now)
	}
	if _, ok := c.curr[key]; ok {
		return false
	}
	if _, ok := c.prev[key]; ok {
		return false
	}
	if len(c.curr) >= c.max {
		c.rotate(now)
	}
	c.curr[key] = struct{}{}
	return true
}

func (c *ReplayCache) rotate(now time.Time) {
	c.prev = c.curr
	c.curr = map[string]struct{}{}
	c.rotateAt = now.Add(c.window)
}
