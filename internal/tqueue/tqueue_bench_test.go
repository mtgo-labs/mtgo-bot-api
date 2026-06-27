package tqueue

import (
	"context"
	"testing"
)

// BenchmarkTQueuePush measures the per-event cost of pushing onto a queue.
func BenchmarkTQueuePush(b *testing.B) {
	tq := New()
	qid := QueueID(1)
	data := []byte(`{"message":{"message_id":1}}`)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := tq.Push(ctx, qid, data, 0, 0, 0); err != nil {
			b.Fatal(err)
		}
		// Prevent unbounded growth from skewing results past maxQueueEvents.
		if i%50000 == 49999 {
			b.StopTimer()
			tq.Clear(ctx, qid, 0)
			b.StartTimer()
		}
	}
}

// BenchmarkTQueueGet measures the per-Get cost of reading events from a
// pre-populated queue with various starting offsets.
func BenchmarkTQueueGet(b *testing.B) {
	tq := New()
	qid := QueueID(1)
	data := []byte(`{"message":{"message_id":1}}`)
	ctx := context.Background()

	// Pre-populate with 1000 events.
	for i := 0; i < 1000; i++ {
		if _, err := tq.Push(ctx, qid, data, 0, 0, 0); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fromID := EventID(i % 1000)
		if _, err := tq.Get(ctx, qid, fromID, false, 1<<30, 100); err != nil {
			b.Fatal(err)
		}
	}
}
