package idempotency

import (
	"context"
	"sync"
	"time"
)

type State int

const (
	New State = iota
	InFlight
	Stored
)

type entry struct {
	inFlight bool
	status   int
	body     []byte
	expires  time.Time
}

type Memory struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]*entry
}

func NewMemory(now func() time.Time) *Memory {
	if now == nil {
		now = time.Now
	}
	return &Memory{now: now, entries: map[string]*entry{}}
}

func (m *Memory) Begin(ctx context.Context, key string) (State, int, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep()
	e, ok := m.entries[key]
	if !ok {
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
	m.entries[key] = &entry{status: status, body: body, expires: m.now().Add(ttl)}
	return nil
}

func (m *Memory) Abort(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}

func (m *Memory) sweep() {
	now := m.now()
	for key, e := range m.entries {
		if !e.inFlight && now.After(e.expires) {
			delete(m.entries, key)
		}
	}
}
