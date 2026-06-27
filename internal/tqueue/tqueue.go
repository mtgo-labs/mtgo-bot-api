package tqueue

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"
)

// QueueID identifies a per-bot update queue. In telegram-bot-api this is
// get_tqueue_id(bot_user_id, is_test_dc) = user_id + (is_test_dc << 54).
// See ClientManager.cpp:313.
type QueueID int64

// QueueIDFor returns the deterministic queue id for a bot.
func QueueIDFor(botUserID int64, isTestDC bool) QueueID {
	if isTestDC {
		return QueueID(botUserID + (1 << 54))
	}
	return QueueID(botUserID)
}

// Event is an in-memory queue event (mirrors TQueue::Event).
type Event struct {
	ID         EventID
	ExpiresAt  int32 // unix timestamp
	Data       []byte
	Extra      int64
	UpdateType string
}

// RawEvent is a persisted event (mirrors TQueue::RawEvent), carrying the
// storage log id assigned by the StorageCallback.
type RawEvent struct {
	LogID      uint64
	QueueID    QueueID // set when reading back from storage (Replay)
	ID         EventID
	ExpiresAt  int32
	Data       []byte
	Extra      int64
	UpdateType string
}

// StorageCallback is the persistence seam mirroring TQueue::StorageCallback.
// Implementations persist events (push) and delete them (pop). The TQueue
type StorageCallback interface {
	// Push persists an event and returns the storage log id (0 if no-op).
	Push(ctx context.Context, qid QueueID, e RawEvent) uint64
	// Pop deletes the persisted event with the given log id.
	Pop(ctx context.Context, logID uint64)
	// Close flushes/closes the storage.
	Close() error
}

// ErrQueueFull is returned when a queue cannot accept more events (id overflow).
var ErrQueueFull = errors.New("tqueue: queue full (EventId overflow)")

// TQueue is a monotonic, per-queue event store with a StorageCallback for
// durability. It mirrors tdlib TQueue: one in-memory structure holds many
// queues keyed by QueueID; push assigns a strictly-increasing EventId; get
// returns events strictly after a given id; forget/clear/gc remove them.
//
// Locking is split: TQueue.mu (an RWMutex) guards only the queues map and the
// callback field. Each per-bot queue has its own RWMutex (queue.mu) so that
// operations on bot A's queue never block bot B. All StorageCallback I/O
// (Push/Pop) is performed outside both locks to avoid serialising SQLite
// access across bots. Lock ordering is always TQueue.mu before queue.mu.
type TQueue struct {
	mu       sync.RWMutex
	queues   map[QueueID]*queue
	callback StorageCallback
	now      func() int32
}

type queue struct {
	mu     sync.RWMutex
	events map[EventID]*RawEvent
	head   EventID // smallest live id (0 if empty)
	tail   EventID // last assigned id (0 if never pushed)
}

// New creates an empty TQueue.
func New() *TQueue {
	return &TQueue{
		queues: make(map[QueueID]*queue),
		now:    func() int32 { return int32(time.Now().Unix()) },
	}
}

// SetCallback installs the persistence callback. Mirrors set_callback.
func (t *TQueue) SetCallback(cb StorageCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callback = cb
}

// Callback returns the installed callback (may be nil).
func (t *TQueue) Callback() StorageCallback {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.callback
}

// NowFunc replaces the clock (for tests).
func (t *TQueue) NowFunc(f func() int32) { t.now = f }

// maxQueueEvents caps the number of live events per queue. Events beyond this
// limit are dropped oldest-first. This prevents unbounded memory growth from
// never-expiring events (ExpiresAt == 0) when a bot never calls getUpdates.
// Mirrors TQueue's size guard. 100k is conservative; the official server uses
// a similar order of magnitude.
const maxQueueEvents = 100_000

// Push appends data to the queue, assigning a monotonic EventId strictly
// greater than the current tail (or hint, whichever is larger). It persists
// via the StorageCallback. Mirrors TQueue::push.
//
// The callback's Push (synchronous SQLite INSERT) is called outside all locks
// so concurrent queues are not serialised on storage I/O. The returned log id
// is recorded in a brief second critical section; if the event was removed in
// the gap the orphaned storage row is popped immediately.
func (t *TQueue) Push(ctx context.Context, qid QueueID, data []byte, expiresAt int32, extra int64, hint EventID) (EventID, error) {
	return t.PushWithData(ctx, qid, expiresAt, extra, hint, func(EventID) ([]byte, string, error) {
		return data, "", nil
	})
}

// PushWithData appends an event whose data is derived from the assigned EventID.
// This lets callers include update_id in the stored JSON at push time without
// reparsing or rewriting the payload in getUpdates/webhook delivery.
func (t *TQueue) PushWithData(ctx context.Context, qid QueueID, expiresAt int32, extra int64, hint EventID, build func(EventID) ([]byte, string, error)) (EventID, error) {
	// Phase 0: get/create queue under the map lock.
	t.mu.Lock()
	q := t.queues[qid]
	if q == nil {
		q = &queue{mu: sync.RWMutex{}, events: make(map[EventID]*RawEvent)}
		t.queues[qid] = q
	}
	cb := t.callback
	t.mu.Unlock()

	// Phase 1: assign ID and store event under the per-queue lock.
	q.mu.Lock()
	nextID := max(hint, q.tail)
	if nextID == 0 {
		nextID = 1 // ids start at 1
	} else {
		n, err := nextID.Next()
		if err != nil {
			q.mu.Unlock()
			return 0, ErrQueueFull
		}
		nextID = n
	}

	data, updateType, err := build(nextID)
	if err != nil {
		q.mu.Unlock()
		return 0, err
	}
	re := &RawEvent{ID: nextID, ExpiresAt: expiresAt, Data: data, Extra: extra, UpdateType: updateType}
	q.events[nextID] = re
	if q.head == 0 {
		q.head = nextID
	}
	q.tail = nextID
	// Enforce per-queue cap: drop oldest events if exceeded.
	var trimmedPops []uint64
	if len(q.events) > maxQueueEvents {
		trimmedPops = t.trimOldestLocked(q, len(q.events)-maxQueueEvents)
	}
	q.mu.Unlock()

	// Persist any cap-induced trims outside all locks.
	popOutsideLock(ctx, cb, trimmedPops)

	// Persist outside all locks to avoid serialising all bots on SQLite I/O.
	if cb != nil {
		logID := cb.Push(ctx, qid, *re)
		if logID != 0 {
			q.mu.Lock()
			if current := q.events[nextID]; current == re {
				re.LogID = logID
			} else {
				// Event was removed before we recorded the log id;
				// undo the storage write to avoid a leak.
				cb.Pop(ctx, logID)
			}
			q.mu.Unlock()
		}
	}

	return nextID, nil
}

// Get returns up to limit events with id strictly greater than fromID, in
// ascending order. If forgetPrevious is set, all events with id <= fromID are
// dropped (persistence Pop included). Mirrors TQueue::get.
//
// Without forgetPrevious this is a pure read and takes only queue read locks.
func (t *TQueue) Get(ctx context.Context, qid QueueID, fromID EventID, forgetPrevious bool, now int32, limit int) ([]Event, error) {
	// Look up the queue and callback under the map read lock.
	t.mu.RLock()
	q := t.queues[qid]
	cb := t.callback
	t.mu.RUnlock()

	if q == nil {
		return nil, nil
	}

	if !forgetPrevious {
		q.mu.RLock()
		defer q.mu.RUnlock()
		return t.getLocked(q, fromID, now, limit), nil
	}

	q.mu.Lock()
	pops := t.forgetUpToLocked(q, fromID)
	events := t.getLocked(q, fromID, now, limit)
	q.mu.Unlock()

	popOutsideLock(ctx, cb, pops)
	return events, nil
}

// getLocked collects up to limit events with id strictly greater than fromID,
// skipping expired entries. Caller must hold q.mu (read or write).
func (t *TQueue) getLocked(q *queue, fromID EventID, now int32, limit int) []Event {
	ids := make([]EventID, 0, len(q.events))
	for id, re := range q.events {
		if id > fromID && (re.ExpiresAt == 0 || re.ExpiresAt >= now) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}

	out := make([]Event, 0, len(ids))
	for _, id := range ids {
		re := q.events[id]
		out = append(out, Event{ID: id, ExpiresAt: re.ExpiresAt, Data: re.Data, Extra: re.Extra, UpdateType: re.UpdateType})
	}
	return out
}

// Forget removes a single event from the queue and storage.
func (t *TQueue) Forget(ctx context.Context, qid QueueID, id EventID) {
	var logID uint64

	// Look up queue and callback under the map read lock.
	t.mu.RLock()
	q := t.queues[qid]
	cb := t.callback
	t.mu.RUnlock()

	if q == nil {
		return
	}

	q.mu.Lock()
	if re, ok := q.events[id]; ok {
		logID = re.LogID
		delete(q.events, id)
		t.recomputeHead(q)
	}
	q.mu.Unlock()

	if cb != nil && logID != 0 {
		cb.Pop(ctx, logID)
	}
}

// forgetUpToLocked removes all events with id <= upto from the queue and
// returns their storage log ids for deferred popping. Caller must hold q.mu
// (write) and call popOutsideLock after releasing it.
func (t *TQueue) forgetUpToLocked(q *queue, upto EventID) []uint64 {
	var pops []uint64
	for id, re := range q.events {
		if id <= upto {
			if re.LogID != 0 {
				pops = append(pops, re.LogID)
			}
			delete(q.events, id)
		}
	}
	t.recomputeHead(q)
	return pops
}

// recomputeHead recalculates the smallest live id. Caller must hold q.mu.
func (t *TQueue) recomputeHead(q *queue) {
	q.head = 0
	for id := range q.events {
		if q.head == 0 || id < q.head {
			q.head = id
		}
	}
}

// trimOldestLocked removes the count oldest events from the queue and returns
// their storage log ids for deferred popping. Caller must hold q.mu (write).
func (t *TQueue) trimOldestLocked(q *queue, count int) []uint64 {
	ids := make([]EventID, 0, len(q.events))
	for id := range q.events {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	if count > len(ids) {
		count = len(ids)
	}
	var pops []uint64
	for _, id := range ids[:count] {
		if re := q.events[id]; re != nil {
			if re.LogID != 0 {
				pops = append(pops, re.LogID)
			}
			delete(q.events, id)
		}
	}
	t.recomputeHead(q)
	return pops
}

// Clear removes all but the last keepCount events. Mirrors TQueue::clear.
func (t *TQueue) Clear(ctx context.Context, qid QueueID, keepCount int) []RawEvent {
	// Look up queue and callback under the map read lock.
	t.mu.RLock()
	q := t.queues[qid]
	cb := t.callback
	t.mu.RUnlock()

	if q == nil {
		return nil
	}

	q.mu.Lock()
	ids := make([]EventID, 0, len(q.events))
	for id := range q.events {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	var dropped []RawEvent
	var pops []uint64
	keepFrom := max(len(ids)-keepCount, 0)
	for _, id := range ids[:keepFrom] {
		re := q.events[id]
		if re.LogID != 0 {
			pops = append(pops, re.LogID)
		}
		dropped = append(dropped, *re)
		delete(q.events, id)
	}
	t.recomputeHead(q)
	q.mu.Unlock()

	popOutsideLock(ctx, cb, pops)
	return dropped
}

// Head returns the smallest live EventId for the queue (0 if empty).
func (t *TQueue) Head(qid QueueID) EventID {
	t.mu.RLock()
	q := t.queues[qid]
	t.mu.RUnlock()
	if q == nil {
		return 0
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.head
}

// Tail returns the last assigned EventId for the queue (0 if never pushed).
func (t *TQueue) Tail(qid QueueID) EventID {
	t.mu.RLock()
	q := t.queues[qid]
	t.mu.RUnlock()
	if q == nil {
		return 0
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.tail
}

// Size returns the number of live events in the queue.
func (t *TQueue) Size(qid QueueID) int {
	t.mu.RLock()
	q := t.queues[qid]
	t.mu.RUnlock()
	if q == nil {
		return 0
	}
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.events)
}

// RunGC deletes all expired events across all queues. Returns the number
// deleted and whether the GC completed. Mirrors TQueue::run_gc.
func (t *TQueue) RunGC(now int32) (deleted int64, complete bool) {
	// Snapshot all queue pointers and callback under the map lock.
	t.mu.Lock()
	queues := make([]*queue, 0, len(t.queues))
	for _, q := range t.queues {
		queues = append(queues, q)
	}
	cb := t.callback
	t.mu.Unlock()

	// Scan each queue under its own lock; different queues run independently.
	var allPops []uint64
	for _, q := range queues {
		q.mu.Lock()
		for id, re := range q.events {
			if re.ExpiresAt != 0 && re.ExpiresAt < now {
				if re.LogID != 0 {
					allPops = append(allPops, re.LogID)
				}
				delete(q.events, id)
				deleted++
			}
		}
		t.recomputeHead(q)
		q.mu.Unlock()
	}

	popOutsideLock(context.Background(), cb, allPops)
	return deleted, true
}

// Replay loads persisted events back into memory after a restart. For each
// event, it restores the in-memory map and advances head/tail. Events are
// expected in ascending id order per queue (the SQLite callback orders them).
//
// This is startup-only: it runs under the single map lock with no per-queue
// locking because no concurrent access is possible during initialisation.
func (t *TQueue) Replay(events []RawEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, e := range events {
		q := t.queues[e.QueueID]
		if q == nil {
			q = &queue{mu: sync.RWMutex{}, events: make(map[EventID]*RawEvent)}
			t.queues[e.QueueID] = q
		}
		ec := e // copy to avoid aliasing the loop var
		q.events[ec.ID] = &ec
		if q.head == 0 || ec.ID < q.head {
			q.head = ec.ID
		}
		if ec.ID > q.tail {
			q.tail = ec.ID
		}
	}
}

// popOutsideLock calls cb.Pop for each log id. The callback may be nil.
func popOutsideLock(ctx context.Context, cb StorageCallback, pops []uint64) {
	if cb == nil {
		return
	}
	for _, logID := range pops {
		cb.Pop(ctx, logID)
	}
}
