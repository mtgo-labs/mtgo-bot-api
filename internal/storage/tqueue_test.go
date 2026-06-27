package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

// TestTQueueRestartPersistence is the core durability contract (quickstart S5):
// push an update into a TQueue backed by the SQLite store, simulate a restart
// by creating a fresh in-memory TQueue and replaying LoadAll(), and assert the
// update is still retrievable with no loss or duplication.
func TestTQueueRestartPersistence(t *testing.T) {
	dir := t.TempDir()

	// --- "first run": open store + queue, push one event ---
	store1, err := OpenTQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	q1 := tqueue.New()
	q1.SetCallback(store1)
	qid := tqueue.QueueIDFor(111, false)
	id1, err := q1.PushWithData(context.Background(), qid, 999999999, 0, 0, func(id tqueue.EventID) ([]byte, string, error) {
		return []byte(`{"update_id":1,"message":{"text":"hi"}}`), "message", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if id1 == 0 {
		t.Fatal("expected non-zero event id")
	}
	if err := store1.Close(); err != nil {
		t.Fatal(err)
	}

	// --- "restart": brand-new TQueue + store, replay persisted events ---
	store2, err := OpenTQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store2.Close() }()
	q2 := tqueue.New()
	events, err := store2.LoadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("LoadAll returned %d events, want 1 (lost on restart)", len(events))
	}
	q2.Replay(events)
	q2.SetCallback(store2)

	// getUpdates-equivalent: get all events from the head.
	got, err := q2.Get(context.Background(), qid, 0, false, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != id1 {
		t.Fatalf("after restart got %+v, want single id %d", got, id1)
	}
	if string(got[0].Data) != `{"update_id":1,"message":{"text":"hi"}}` {
		t.Errorf("data mismatch after restart: %s", got[0].Data)
	}
	if got[0].UpdateType != "message" {
		t.Errorf("update type after restart = %q, want message", got[0].UpdateType)
	}

	// Offset-confirm semantics survive too: confirm up to id1 forgets it.
	if _, err := q2.Get(context.Background(), qid, id1, true, 0, 0); err != nil {
		t.Fatal(err)
	}
	if q2.Size(qid) != 0 {
		t.Errorf("after confirm, size=%d, want 0", q2.Size(qid))
	}
	// And it must be gone from storage too.
	store3, err := OpenTQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store3.Close() }()
	remain, err := store3.LoadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(remain) != 0 {
		t.Errorf("storage still has %d events after confirm", len(remain))
	}
}

// TestTQueueStoreGC verifies expired events are purged from storage.
func TestTQueueStoreGC(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenTQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	q := tqueue.New()
	q.SetCallback(store)
	qid := tqueue.QueueIDFor(222, false)
	_, _ = q.Push(context.Background(), qid, []byte("old"), 100, 0, 0)
	_, _ = q.Push(context.Background(), qid, []byte("new"), 999999999, 0, 0)

	n, err := store.GC(context.Background(), 200)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("GC deleted %d, want 1", n)
	}
	all, _ := store.LoadAll(context.Background())
	if len(all) != 1 {
		t.Errorf("after GC storage has %d, want 1", len(all))
	}
	_ = filepath.Base(store.path)
}
