package idempotencytest_test

import (
	"testing"

	"github.com/bowlinedev/bowline"
	"github.com/bowlinedev/bowline/idempotencytest"
)

func TestMemoryStore(t *testing.T) {
	idempotencytest.Verify(t, func(t *testing.T) bowline.IdempotencyStore {
		return bowline.MemoryIdempotencyStore()
	})
}
