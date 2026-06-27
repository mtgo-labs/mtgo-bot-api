package tqueue

import (
	"context"
	"runtime"
	"sync"
	"testing"
)

// TestTQueueConcurrentMultiQueue pushes and gets events concurrently across
// multiple queues (bots), verifying per-queue isolation: bot A's events never
// leak to bot B. Each event's data is tagged with the owning queue's marker
// byte so any cross-queue contamination is detected. Run with -race.
func TestTQueueConcurrentMultiQueue(t *testing.T) {
	tq := New()
	const numQueues = 5
	const eventsPerQueue = 100

	var wg sync.WaitGroup
	for q := 0; q < numQueues; q++ {
		qid := QueueID(q + 1)
		marker := byte(q + 1) // unique single-byte tag per queue

		// Pusher goroutine.
		wg.Add(1)
		go func(qid QueueID, marker byte) {
			defer wg.Done()
			for i := 0; i < eventsPerQueue; i++ {
				data := []byte{marker, byte(i + 1)}
				if _, err := tq.Push(context.Background(), qid, data, 0, 0, 0); err != nil {
					t.Errorf("Push(%d): %v", qid, err)
					return
				}
			}
		}(qid, marker)

		// Getter goroutine — drain events for this queue, verifying tags.
		wg.Add(1)
		go func(qid QueueID, marker byte) {
			defer wg.Done()
			var fromID EventID
			drained := 0
			for drained < eventsPerQueue {
				events, err := tq.Get(context.Background(), qid, fromID, false, 1<<30, 50)
				if err != nil {
					t.Errorf("Get(%d): %v", qid, err)
					return
				}
				for _, e := range events {
					if len(e.Data) == 0 {
						t.Errorf("queue %d: empty event data", qid)
					} else if e.Data[0] != marker {
						t.Errorf("queue %d: data leak — marker %d, want %d", qid, e.Data[0], marker)
					}
					fromID = e.ID
					drained++
				}
				if len(events) == 0 {
					runtime.Gosched()
				}
			}
		}(qid, marker)
	}
	wg.Wait()

	// Final verification: each queue has exactly eventsPerQueue events, all tagged correctly.
	for q := 0; q < numQueues; q++ {
		qid := QueueID(q + 1)
		events, err := tq.Get(context.Background(), qid, 0, false, 1<<30, eventsPerQueue+10)
		if err != nil {
			t.Fatalf("final Get(%d): %v", qid, err)
		}
		if len(events) != eventsPerQueue {
			t.Errorf("queue %d: got %d events, want %d", qid, len(events), eventsPerQueue)
		}
		marker := byte(q + 1)
		for _, e := range events {
			if len(e.Data) == 0 {
				t.Errorf("queue %d: empty event data", qid)
			} else if e.Data[0] != marker {
				t.Errorf("queue %d: cross-queue leak — marker %d, want %d", qid, e.Data[0], marker)
			}
		}
	}
}
