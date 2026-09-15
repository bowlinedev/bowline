package billing

import (
	"sync"
	"time"
)

type Store struct {
	mu      sync.Mutex
	now     func() time.Time
	charges map[int64]Charge
	nextID  int64
}

func NewStore(now func() time.Time) *Store {
	return &Store{now: now, charges: map[int64]Charge{}, nextID: 1}
}

func (s *Store) Create(invoiceID int64, amount string) Charge {
	s.mu.Lock()
	defer s.mu.Unlock()
	charge := Charge{ID: s.nextID, InvoiceID: invoiceID, Amount: amount, Status: StatusOpen, CreatedAt: s.now().UTC()}
	s.charges[charge.ID] = charge
	s.nextID++
	return charge
}

func (s *Store) List() []Charge {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Charge, 0, len(s.charges))
	for id := int64(1); id < s.nextID; id++ {
		if charge, ok := s.charges[id]; ok {
			out = append(out, charge)
		}
	}
	return out
}

func (s *Store) Settle(id int64) (Charge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	charge, ok := s.charges[id]
	if !ok {
		return Charge{}, false
	}
	charge.Status = StatusSettled
	s.charges[id] = charge
	return charge, true
}
