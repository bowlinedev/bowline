package sql_test

import (
	"context"
	dbsql "database/sql"
	"path/filepath"
	"testing"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/idempotencytest"
	store "github.com/bowlinedev/bowline/stores/sql"
	_ "modernc.org/sqlite"
)

func sqliteStore(t *testing.T) bowline.IdempotencyStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "idempotency.db")
	db, err := dbsql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := store.New(db, store.SQLite)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSQLiteStore(t *testing.T) {
	idempotencytest.Verify(t, sqliteStore)
}
