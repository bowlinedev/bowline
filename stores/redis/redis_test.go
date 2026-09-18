package redis_test

import (
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/idempotencytest"
	store "github.com/bowlinedev/bowline/stores/redis"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisStore(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })
	s, err := store.New(client)
	if err != nil {
		t.Fatal(err)
	}
	idempotencytest.Verify(t, func(t *testing.T) bowline.IdempotencyStore { return s })
}

func TestRedisStoreAgainstARealServer(t *testing.T) {
	addr := os.Getenv("BOWLINE_REDIS_ADDR")
	if addr == "" {
		t.Skip("set BOWLINE_REDIS_ADDR to run against a real Redis")
	}
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	t.Cleanup(func() { client.Close() })
	s, err := store.New(client)
	if err != nil {
		t.Fatal(err)
	}
	idempotencytest.Verify(t, func(t *testing.T) bowline.IdempotencyStore { return s })
}

func TestRejectsANilClient(t *testing.T) {
	if _, err := store.New(nil); err == nil {
		t.Fatal("a nil client was accepted")
	}
}
