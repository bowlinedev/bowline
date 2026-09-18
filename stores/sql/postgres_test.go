package sql_test

import (
	"context"
	dbsql "database/sql"
	"os"
	"strings"
	"testing"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/idempotencytest"
	store "github.com/bowlinedev/bowline/stores/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresStore(t *testing.T) {
	url := os.Getenv("BOWLINE_POSTGRES_URL")
	if url == "" {
		t.Skip("set BOWLINE_POSTGRES_URL to run against a real Postgres")
	}
	db, err := dbsql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := store.New(db, store.Postgres)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	idempotencytest.Verify(t, func(t *testing.T) bowline.IdempotencyStore { return s })
}

func TestPostgresUsesNumberedPlaceholders(t *testing.T) {
	db, err := dbsql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := store.New(db, store.Postgres)
	if err != nil {
		t.Fatal(err)
	}
	for name, query := range s.Queries() {
		if strings.Contains(query, "?") {
			t.Errorf("%s still has a positional placeholder, which Postgres rejects: %s", name, query)
		}
		if !strings.Contains(query, "$1") {
			t.Errorf("%s has no numbered placeholder: %s", name, query)
		}
	}
}

func TestSQLiteKeepsPositionalPlaceholders(t *testing.T) {
	db, err := dbsql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := store.New(db, store.SQLite)
	if err != nil {
		t.Fatal(err)
	}
	for name, query := range s.Queries() {
		if strings.Contains(query, "$1") {
			t.Errorf("%s uses a numbered placeholder, which SQLite does not bind: %s", name, query)
		}
	}
}

func TestRejectsAnUnsafeTableName(t *testing.T) {
	db, _ := dbsql.Open("sqlite", ":memory:")
	defer db.Close()
	for _, name := range []string{"", "bowline; DROP TABLE users", "bowline-keys", "1bad", "bowline keys"} {
		if _, err := store.New(db, store.SQLite, store.WithTable(name)); err == nil {
			t.Errorf("table %q was accepted; it is interpolated into SQL and must be rejected", name)
		}
	}
}
