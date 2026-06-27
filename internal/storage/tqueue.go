package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"

	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
)

// TQueueStore is a global SQLite-backed implementation of tqueue.StorageCallback.
// It persists the update queue for ALL bots in a single database at
// <dir>/tqueue.db, keyed by queue_id, mirroring telegram-bot-api's single
type TQueueStore struct {
	db       *sql.DB
	rdb      *sql.DB
	path     string
	mu       sync.Mutex
	pushStmt *sql.Stmt
	popStmt  *sql.Stmt
	gcStmt   *sql.Stmt
}

// OpenTQueue opens/creates the global tqueue.db and its schema.
func OpenTQueue(dir string) (*TQueueStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("storage: tqueue dir: %w", err)
	}
	path := filepath.Join(dir, "tqueue.db")
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("storage: open tqueue %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	rdb, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=query_only(1)")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage: open tqueue reader %s: %w", path, err)
	}
	rdb.SetMaxOpenConns(4)
	s := &TQueueStore{db: db, rdb: rdb, path: path}
	if _, err := db.Exec(tqueueSchema); err != nil {
		_ = rdb.Close()
		_ = db.Close()
		return nil, fmt.Errorf("storage: tqueue schema: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE updates ADD COLUMN update_type TEXT NOT NULL DEFAULT ''`); err != nil {
		if !isTQueueDupColumnErr(err) {
			_ = rdb.Close()
			_ = db.Close()
			return nil, fmt.Errorf("storage: tqueue migrate update_type: %w", err)
		}
	}
	// Prepare hot-path statements once — eliminates parse/plan overhead per call.
	s.pushStmt, err = db.Prepare(`INSERT INTO updates (queue_id, event_id, expires_at, data, extra, update_type) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = rdb.Close()
		_ = db.Close()
		return nil, fmt.Errorf("storage: prepare push: %w", err)
	}
	s.popStmt, err = db.Prepare(`DELETE FROM updates WHERE log_id = ?`)
	if err != nil {
		_ = s.pushStmt.Close()
		_ = rdb.Close()
		_ = db.Close()
		return nil, fmt.Errorf("storage: prepare pop: %w", err)
	}
	s.gcStmt, err = db.Prepare(`DELETE FROM updates WHERE expires_at != 0 AND expires_at < ?`)
	if err != nil {
		_ = s.pushStmt.Close()
		_ = s.popStmt.Close()
		_ = rdb.Close()
		_ = db.Close()
		return nil, fmt.Errorf("storage: prepare gc: %w", err)
	}
	return s, nil
}

func isTQueueDupColumnErr(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return strings.Contains(sqliteErr.Error(), "duplicate column name")
}

const tqueueSchema = `
CREATE TABLE IF NOT EXISTS updates (
    log_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    queue_id    INTEGER NOT NULL,
    event_id    INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL,
    data        BLOB    NOT NULL,
    extra       INTEGER NOT NULL,
    update_type TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_updates_queue_event ON updates(queue_id, event_id);
`

// Close closes the prepared statements and the database.
func (s *TQueueStore) Close() error {
	if s.pushStmt != nil {
		_ = s.pushStmt.Close()
	}
	if s.popStmt != nil {
		_ = s.popStmt.Close()
	}
	if s.gcStmt != nil {
		_ = s.gcStmt.Close()
	}
	if s.rdb != nil {
		_ = s.rdb.Close()
	}
	return s.db.Close()
}

// Push persists an event and returns its log_id. Implements tqueue.StorageCallback.
func (s *TQueueStore) Push(ctx context.Context, qid tqueue.QueueID, e tqueue.RawEvent) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.pushStmt.ExecContext(ctx, int64(qid), int64(e.ID), e.ExpiresAt, e.Data, e.Extra, e.UpdateType)
	if err != nil {
		return 0
	}
	id, _ := res.LastInsertId()
	return uint64(id)
}

// Pop deletes the persisted event with the given log id. Implements tqueue.StorageCallback.
func (s *TQueueStore) Pop(ctx context.Context, logID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.popStmt.ExecContext(ctx, int64(logID))
}

// LoadAll returns all persisted events ordered by (queue_id, event_id) for
// replay into an in-memory TQueue on startup. Mirrors binlog replay
// (ClientManager.cpp:326-335).
func (s *TQueueStore) LoadAll(ctx context.Context) ([]tqueue.RawEvent, error) {
	const batchSize = 10_000
	var out []tqueue.RawEvent
	var afterLogID uint64
	for {
		rows, err := s.rdb.QueryContext(ctx,
			`SELECT log_id, queue_id, event_id, expires_at, data, extra, update_type
				 FROM updates WHERE log_id > ? ORDER BY log_id LIMIT ?`,
			afterLogID, batchSize)
		if err != nil {
			return nil, err
		}
		var n int
		for rows.Next() {
			var re tqueue.RawEvent
			var qid, eid int64
			if err := rows.Scan(&re.LogID, &qid, &eid, &re.ExpiresAt, &re.Data, &re.Extra, &re.UpdateType); err != nil {
				rows.Close()
				return nil, err
			}
			re.QueueID = tqueue.QueueID(qid)
			re.ID = tqueue.EventID(eid)
			out = append(out, re)
			afterLogID = re.LogID
			n++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if n < batchSize {
			break // final batch
		}
	}
	return out, nil
}

// GC deletes expired events (expires_at != 0 AND expires_at < now) and returns
// the number deleted. Mirrors TQueue::run_gc persistence side.
func (s *TQueueStore) GC(ctx context.Context, now int32) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.gcStmt.ExecContext(ctx, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
