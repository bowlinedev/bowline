package ledger

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	mu          sync.RWMutex
	now         func() time.Time
	invoices    map[int64]Invoice
	customers   map[int64]Customer
	attachments map[int64]Attachment
	watchers    map[int64]chan Invoice
	nextID      int64
	nextWatcher int64
}

func NewStore(now func() time.Time) *Store {
	s := &Store{now: now, invoices: map[int64]Invoice{}, customers: map[int64]Customer{}, attachments: map[int64]Attachment{}, watchers: map[int64]chan Invoice{}, nextID: 1}
	s.seed()
	return s
}

func (s *Store) Watch() (<-chan Invoice, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextWatcher
	s.nextWatcher++
	ch := make(chan Invoice, 64)
	s.watchers[id] = ch
	return ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.watchers[id]; ok {
			delete(s.watchers, id)
			close(ch)
		}
	}
}

func (s *Store) Attach(invoiceID int64, name, contentType string, size int64) Attachment {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	a := Attachment{ID: s.nextID, InvoiceID: invoiceID, Name: name, ContentType: contentType, Size: size, Audit: Audit{CreatedAt: now, UpdatedAt: now}}
	s.nextID++
	s.attachments[a.ID] = a
	return a
}

func (s *Store) Attachments(invoiceID int64) []Attachment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Attachment
	for _, a := range s.attachments {
		if a.InvoiceID == invoiceID {
			out = append(out, a)
		}
	}
	slices.SortFunc(out, func(a, b Attachment) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

func (s *Store) seed() {
	ada := s.PutCustomer(Customer{Name: "Ada Lovelace", Email: "ada@example.com"})
	grace := s.PutCustomer(Customer{Name: "Grace Hopper", Email: "grace@example.com"})
	note := "net 30"
	s.PutInvoice(Invoice{CustomerID: ada.ID, Status: StatusSent, Lines: []Line{{Description: "Consulting", Quantity: 10, UnitPrice: Money{15000, "USD"}}}, Note: &note})
	s.PutInvoice(Invoice{CustomerID: grace.ID, Status: StatusPaid, Lines: []Line{{Description: "Compiler", Quantity: 1, UnitPrice: Money{999900, "USD"}}}})
}

func (s *Store) PutCustomer(c Customer) Customer {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if c.ID == 0 {
		c.ID = s.nextID
		s.nextID++
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	s.customers[c.ID] = c
	return c
}

func (s *Store) Customer(id int64) (Customer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.customers[id]
	if !ok {
		return Customer{}, ErrNotFound
	}
	return c, nil
}

func (s *Store) SearchCustomers(query string) []Customer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Customer
	for _, c := range s.customers {
		if strings.Contains(strings.ToLower(c.Name), strings.ToLower(query)) || strings.Contains(strings.ToLower(c.Email), strings.ToLower(query)) {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b Customer) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

func (s *Store) PutInvoice(inv Invoice) Invoice {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if inv.ID == 0 {
		inv.ID = s.nextID
		s.nextID++
		inv.CreatedAt = now
	}
	inv.UpdatedAt = now
	total := Money{Currency: "USD"}
	for _, l := range inv.Lines {
		total = total.Add(l.UnitPrice.Times(l.Quantity))
	}
	inv.Total = total
	s.invoices[inv.ID] = inv
	for _, ch := range s.watchers {
		select {
		case ch <- inv:
		default:
		}
	}
	return inv
}

func (s *Store) Invoice(id int64) (Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inv, ok := s.invoices[id]
	if !ok {
		return Invoice{}, ErrNotFound
	}
	return inv, nil
}

func (s *Store) ListInvoices(cursor string, limit int, status *Status) ([]Invoice, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	after, _ := strconv.ParseInt(cursor, 10, 64)
	var all []Invoice
	for _, inv := range s.invoices {
		if inv.ID > after && (status == nil || inv.Status == *status) {
			all = append(all, inv)
		}
	}
	slices.SortFunc(all, func(a, b Invoice) int { return cmp.Compare(a.ID, b.ID) })
	if len(all) <= limit {
		return all, ""
	}
	page := all[:limit]
	return page, strconv.FormatInt(page[len(page)-1].ID, 10)
}
