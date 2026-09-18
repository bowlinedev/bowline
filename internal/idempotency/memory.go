package idempotency

import (
	"context"
	"slices"
	"sync"
	"time"
)

type State int

const (
	New State = iota
	InFlight
	Stored
)

const (
	DefaultMaxKeys = 10000
	sweepInterval  = time.Second
)

type entry struct {
	inFlight bool
	status   int
	body     []byte
	expires  time.Time
}

type Memory struct {
	mu        sync.Mutex
	now       func() time.Time
	entries   map[string]*entry
	maxKeys   int
	nextSweep time.Time
}

func NewMemory(now func() time.Time) *Memory {
	return NewMemoryWithLimit(now, DefaultMaxKeys)
}

func NewMemoryWithLimit(now func() time.Time, maxKeys int) *Memory {
	if now == nil {
		now = time.Now
	}
	if maxKeys <= 0 {
		maxKeys = DefaultMaxKeys
	}
	return &Memory{now: now, entries: map[string]*entry{}, maxKeys: maxKeys}
}

func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

func (m *Memory) Begin(ctx context.Context, key string) (State, int, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(false)
	e, ok := m.entries[key]
	if !ok {
		m.reserve()
		m.entries[key] = &entry{inFlight: true, expires: m.now().Add(time.Minute)}
		return New, 0, nil, nil
	}
	if e.inFlight {
		return InFlight, 0, nil, nil
	}
	return Stored, e.status, e.body, nil
}

func (m *Memory) Complete(ctx context.Context, key string, status int, body []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.entries[key]; !ok {
		m.reserve()
	}
	m.entries[key] = &entry{status: status, body: body, expires: m.now().Add(ttl)}
	return nil
}

func (m *Memory) Abort(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}

func (m *Memory) reserve() {
	if len(m.entries) < m.maxKeys {
		return
	}
	m.sweep(true)
	if len(m.entries) < m.maxKeys {
		return
	}
	batch := max(m.maxKeys/16, 1)
	candidates := make([]eviction, 0, len(m.entries))
	for key, e := range m.entries {
		if e.inFlight {
			continue
		}
		candidates = append(candidates, eviction{key: key, expires: e.expires})
	}
	if len(candidates) == 0 {
		return
	}
	batch = min(batch, len(candidates))
	slices.SortFunc(candidates, func(a, b eviction) int { return a.expires.Compare(b.expires) })
	for _, c := range candidates[:batch] {
		delete(m.entries, c.key)
	}
}

type eviction struct {
	key     string
	expires time.Time
}

func (m *Memory) sweep(force bool) {
	now := m.now()
	if !force && now.Before(m.nextSweep) {
		return
	}
	m.nextSweep = now.Add(sweepInterval)
	for key, e := range m.entries {
		if !e.inFlight && now.After(e.expires) {
			delete(m.entries, key)
		}
	}
}
