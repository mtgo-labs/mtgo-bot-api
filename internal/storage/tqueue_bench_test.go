package storage

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func BenchmarkTQueueStoreSynchronous(b *testing.B) {
	for _, tc := range []struct {
		name        string
		synchronous string
	}{
		{name: "FULL", synchronous: "FULL"},
		{name: "NORMAL", synchronous: "NORMAL"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			db := openBenchTQueueDB(b, tc.synchronous)
			defer func() { _ = db.Close() }()
			stmt, err := db.Prepare(`INSERT INTO updates (queue_id, event_id, expires_at, data, extra, update_type) VALUES (?, ?, ?, ?, ?, ?)`)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = stmt.Close() }()

			data := []byte(`{"update_id":1,"message":{"message_id":1}}`)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := stmt.Exec(1, i+1, 0, data, 0, "message"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkTQueueStoreInsertPrepared(b *testing.B) {
	for _, tc := range []struct {
		name     string
		prepared bool
	}{
		{name: "Exec", prepared: false},
		{name: "Prepared", prepared: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			db := openBenchTQueueDB(b, "NORMAL")
			defer func() { _ = db.Close() }()

			data := []byte(`{"update_id":1,"message":{"message_id":1}}`)
			const query = `INSERT INTO updates (queue_id, event_id, expires_at, data, extra, update_type) VALUES (?, ?, ?, ?, ?, ?)`
			b.ReportAllocs()
			if tc.prepared {
				stmt, err := db.Prepare(query)
				if err != nil {
					b.Fatal(err)
				}
				defer func() { _ = stmt.Close() }()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := stmt.Exec(1, i+1, 0, data, 0, "message"); err != nil {
						b.Fatal(err)
					}
				}
				return
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := db.Exec(query, 1, i+1, 0, data, 0, "message"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func openBenchTQueueDB(b *testing.B, synchronous string) *sql.DB {
	b.Helper()
	path := filepath.Join(b.TempDir(), "tqueue-bench.db")
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous("+synchronous+")")
	if err != nil {
		b.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(tqueueSchema); err != nil {
		_ = db.Close()
		b.Fatal(err)
	}
	return db
}
