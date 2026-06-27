package tqueue

import (
	"context"
	"sync"
	"testing"
)

func TestEventIDNextAndAdvance(t *testing.T) {
	id, err := EventIDFromInt32(5)
	if err != nil {
		t.Fatal(err)
	}
	next, err := id.Next()
	if err != nil || next != 6 {
		t.Errorf("Next = %d err=%v, want 6", next, err)
	}
	adv, err := id.Advance(10)
	if err != nil || adv != 15 {
		t.Errorf("Advance(10) = %d err=%v, want 15", adv, err)
	}
	if _, err := EventID(MaxID).Next(); err == nil {
		t.Error("Next past MaxID should error")
	}
	if _, err := EventID(MaxID).Advance(1); err == nil {
		t.Error("Advance past MaxID should error")
	}
}

func TestPushGetMonotonic(t *testing.T) {
	tq := New()
	qid := QueueID(1)

	id1, err := tq.Push(context.Background(), qid, []byte("a"), 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := tq.Push(context.Background(), qid, []byte("b"), 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if id2 <= id1 {
		t.Fatalf("ids not monotonic: %d then %d", id1, id2)
	}
	if tq.Head(qid) != id1 || tq.Tail(qid) != id2 {
		t.Errorf("head=%d tail=%d, want head=%d tail=%d", tq.Head(qid), tq.Tail(qid), id1, id2)
	}

	// Get strictly after id1 → only id2.
	evs, err := tq.Get(context.Background(), qid, id1, false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].ID != id2 {
		t.Errorf("Get(>%d) = %+v, want only %d", id1, evs, id2)
	}
}

func TestGetOffsetConfirmForgets(t *testing.T) {
	tq := New()
	qid := QueueID(1)
	a, _ := tq.Push(context.Background(), qid, []byte("a"), 0, 0, 0)
	b, _ := tq.Push(context.Background(), qid, []byte("b"), 0, 0, 0)

	// Confirm up to b: forgetPrevious with fromID=b drops a and b.
	_, err := tq.Get(context.Background(), qid, b, true, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if tq.Size(qid) != 0 {
		t.Errorf("after confirm, size=%d, want 0", tq.Size(qid))
	}
	_ = a
}

func TestRunGC(t *testing.T) {
	tq := New()
	qid := QueueID(1)
	_, _ = tq.Push(context.Background(), qid, []byte("old"), 100, 0, 0) // expired (100 < now)
	_, _ = tq.Push(context.Background(), qid, []byte("new"), 999999999, 0, 0)

	deleted, complete := tq.RunGC(200)
	if deleted != 1 || !complete {
		t.Errorf("RunGC deleted=%d complete=%v, want 1/true", deleted, complete)
	}
	if tq.Size(qid) != 1 {
		t.Errorf("size after GC=%d, want 1", tq.Size(qid))
	}
}

func TestCallbackWired(t *testing.T) {
	tq := New()
	cb := &memCB{}
	tq.SetCallback(cb)

	_, _ = tq.Push(context.Background(), QueueID(7), []byte("x"), 0, 0, 0)
	if cb.pushed != 1 {
		t.Errorf("pushed=%d, want 1", cb.pushed)
	}
	// Forgetting the event should trigger a Pop.
	tq.Forget(context.Background(), QueueID(7), tq.Head(QueueID(7)))
	if cb.popped != 1 {
		t.Errorf("popped=%d, want 1", cb.popped)
	}
}

func TestQueueIDFor(t *testing.T) {
	if got := QueueIDFor(42, false); got != QueueID(42) {
		t.Errorf("QueueIDFor(42,false)=%d, want 42", got)
	}
	if got := QueueIDFor(42, true); got == QueueID(42) {
		t.Error("QueueIDFor(42,true) must differ from non-test")
	}
}

func TestConcurrentPushDifferentQueues(t *testing.T) {
	tq := New()
	const (
		queues   = 8
		perQueue = 500
	)
	var wg sync.WaitGroup
	for q := 0; q < queues; q++ {
		qid := QueueID(q + 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perQueue; i++ {
				if _, err := tq.Push(context.Background(), qid, []byte("x"), 0, 0, 0); err != nil {
					t.Errorf("Push(%d): %v", qid, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	for q := 0; q < queues; q++ {
		qid := QueueID(q + 1)
		if got := tq.Size(qid); got != perQueue {
			t.Fatalf("Size(%d) = %d, want %d", qid, got, perQueue)
		}
		if got := tq.Tail(qid); got != EventID(perQueue) {
			t.Fatalf("Tail(%d) = %d, want %d", qid, got, perQueue)
		}
	}
}

type memCB struct {
	pushed, popped int
	nextLog        uint64
}

func (m *memCB) Push(ctx context.Context, qid QueueID, e RawEvent) uint64 {
	m.nextLog++
	m.pushed++
	return m.nextLog
}
func (m *memCB) Pop(ctx context.Context, logID uint64) { m.popped++ }
func (m *memCB) Close() error                          { return nil }
