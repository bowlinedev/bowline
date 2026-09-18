package idempotency

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func BenchmarkMemoryUniqueKeysAtCapacity(b *testing.B) {
	store := NewMemoryWithLimit(time.Now, 10000)
	ctx := context.Background()
	for i := range 10000 {
		key := strconv.Itoa(i)
		store.Begin(ctx, key)
		store.Complete(ctx, key, 200, []byte(`{}`), time.Hour)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench" + strconv.Itoa(i)
		store.Begin(ctx, key)
		store.Complete(ctx, key, 200, []byte(`{}`), time.Hour)
	}
}
