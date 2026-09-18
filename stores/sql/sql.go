package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bowlinedev/bowline"
)

const DefaultTable = "bowline_idempotency"

type Dialect int

const (
	Postgres Dialect = iota
	SQLite
)

type Option func(*Store)

func WithTable(name string) Option {
	return func(s *Store) { s.table = name }
}

func WithClaimTTL(d time.Duration) Option {
	return func(s *Store) { s.claimTTL = d }
}

func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

type Store struct {
	db       *sql.DB
	dialect  Dialect
	table    string
	claimTTL time.Duration
	now      func() time.Time

	claim   string
	reclaim string
	read    string
	store   string
	release string
}

func New(db *sql.DB, dialect Dialect, opts ...Option) (*Store, error) {
	if db == nil {
		return nil, errors.New("bowline sql store: nil database")
	}
	if dialect != Postgres && dialect != SQLite {
		return nil, fmt.Errorf("bowline sql store: unknown dialect %d", dialect)
	}
	s := &Store{db: db, dialect: dialect, table: DefaultTable, claimTTL: time.Minute, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	if err := validTable(s.table); err != nil {
		return nil, err
	}
	s.prepare()
	return s, nil
}

func validTable(name string) error {
	if name == "" {
		return errors.New("bowline sql store: empty table name")
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return fmt.Errorf("bowline sql store: table %q may use only letters, digits and underscores", name)
		}
	}
	return nil
}

func (s *Store) rewrite(query string) string {
	if s.dialect != Postgres {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (s *Store) prepare() {
	s.claim = s.rewrite("INSERT INTO " + s.table + " (key, in_flight, status, body, expires_at) VALUES (?, TRUE, 0, NULL, ?) ON CONFLICT (key) DO NOTHING")
	s.reclaim = s.rewrite("UPDATE " + s.table + " SET in_flight = TRUE, status = 0, body = NULL, expires_at = ? WHERE key = ? AND expires_at <= ?")
	s.read = s.rewrite("SELECT in_flight, status, body FROM " + s.table + " WHERE key = ?")
	s.store = s.rewrite("INSERT INTO " + s.table + " (key, in_flight, status, body, expires_at) VALUES (?, FALSE, ?, ?, ?) ON CONFLICT (key) DO UPDATE SET in_flight = FALSE, status = excluded.status, body = excluded.body, expires_at = excluded.expires_at")
	s.release = s.rewrite("DELETE FROM " + s.table + " WHERE key = ?")
}

func (s *Store) Migrate(ctx context.Context) error {
	body := "BYTEA"
	if s.dialect == SQLite {
		body = "BLOB"
	}
	schema := "CREATE TABLE IF NOT EXISTS " + s.table + ` (
	key TEXT PRIMARY KEY,
	in_flight BOOLEAN NOT NULL,
	status INTEGER NOT NULL,
	body ` + body + `,
	expires_at TIMESTAMP NOT NULL
)`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("bowline sql store: creating %s: %w", s.table, err)
	}
	index := "CREATE INDEX IF NOT EXISTS " + s.table + "_expires_at ON " + s.table + " (expires_at)"
	if _, err := s.db.ExecContext(ctx, index); err != nil {
		return fmt.Errorf("bowline sql store: indexing %s: %w", s.table, err)
	}
	return nil
}

func (s *Store) Begin(ctx context.Context, key string) (bowline.IdempotencyState, int, []byte, error) {
	now := s.now().UTC()
	claimed, err := s.db.ExecContext(ctx, s.claim, key, now.Add(s.claimTTL))
	if err != nil {
		return bowline.IdempotencyNew, 0, nil, fmt.Errorf("bowline sql store: claiming %s: %w", s.table, err)
	}
	if rows, err := claimed.RowsAffected(); err == nil && rows == 1 {
		return bowline.IdempotencyNew, 0, nil, nil
	}
	taken, err := s.db.ExecContext(ctx, s.reclaim, now.Add(s.claimTTL), key, now)
	if err != nil {
		return bowline.IdempotencyNew, 0, nil, fmt.Errorf("bowline sql store: reclaiming %s: %w", s.table, err)
	}
	if rows, err := taken.RowsAffected(); err == nil && rows == 1 {
		return bowline.IdempotencyNew, 0, nil, nil
	}
	var inFlight bool
	var status int
	var body []byte
	switch err := s.db.QueryRowContext(ctx, s.read, key).Scan(&inFlight, &status, &body); {
	case errors.Is(err, sql.ErrNoRows):
		return bowline.IdempotencyNew, 0, nil, nil
	case err != nil:
		return bowline.IdempotencyNew, 0, nil, fmt.Errorf("bowline sql store: reading %s: %w", s.table, err)
	}
	if inFlight {
		return bowline.IdempotencyInFlight, 0, nil, nil
	}
	if body == nil {
		body = []byte{}
	}
	return bowline.IdempotencyStored, status, body, nil
}

func (s *Store) Complete(ctx context.Context, key string, status int, body []byte, ttl time.Duration) error {
	if body == nil {
		body = []byte{}
	}
	if _, err := s.db.ExecContext(ctx, s.store, key, status, body, s.now().UTC().Add(ttl)); err != nil {
		return fmt.Errorf("bowline sql store: storing %s: %w", s.table, err)
	}
	return nil
}

func (s *Store) Abort(ctx context.Context, key string) error {
	if _, err := s.db.ExecContext(ctx, s.release, key); err != nil {
		return fmt.Errorf("bowline sql store: releasing %s: %w", s.table, err)
	}
	return nil
}

func (s *Store) Sweep(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, s.rewrite("DELETE FROM "+s.table+" WHERE expires_at <= ?"), s.now().UTC())
	if err != nil {
		return 0, fmt.Errorf("bowline sql store: sweeping %s: %w", s.table, err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return removed, nil
}

func (s *Store) Queries() map[string]string {
	return map[string]string{
		"claim":   s.claim,
		"reclaim": s.reclaim,
		"read":    s.read,
		"store":   s.store,
		"release": s.release,
	}
}
