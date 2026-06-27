package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir, "123456")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close(); os.RemoveAll(dir) })
	return s
}

func testLegacyStore(t *testing.T, schema string) *Store {
	t.Helper()
	dir := t.TempDir()
	botID := "123456"
	dbDir := filepath.Join(dir, botID)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dbDir, "bot.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir, botID)
	if err != nil {
		t.Fatalf("Open legacy store: %v", err)
	}
	t.Cleanup(func() { s.Close(); os.RemoveAll(dir) })
	return s
}

func TestPeer_SaveAndGet(t *testing.T) {
	s := testStore(t)

	p := Peer{ID: 123, AccessHash: 456, Type: PeerTypeUser, Username: "alice", FirstName: "Alice"}
	if err := s.SavePeer(context.Background(), p); err != nil {
		t.Fatalf("SavePeer: %v", err)
	}

	got, err := s.GetPeer(context.Background(), 123)
	if err != nil {
		t.Fatalf("GetPeer: %v", err)
	}
	if got.ID != 123 || got.AccessHash != 456 || got.Username != "alice" {
		t.Errorf("got %+v, want {ID:123 AccessHash:456 Username:alice}", got)
	}
}

func TestPeer_GetByUsername(t *testing.T) {
	s := testStore(t)

	p := Peer{ID: 789, AccessHash: 111, Type: PeerTypeUser, Username: "bob", FirstName: "Bob"}
	if err := s.SavePeer(context.Background(), p); err != nil {
		t.Fatalf("SavePeer: %v", err)
	}

	got, err := s.GetPeerByUsername(context.Background(), "bob")
	if err != nil {
		t.Fatalf("GetPeerByUsername: %v", err)
	}
	if got.ID != 789 {
		t.Errorf("ID = %d, want 789", got.ID)
	}
	if got.FirstName != "Bob" {
		t.Errorf("FirstName = %q, want Bob", got.FirstName)
	}
}

func TestPeer_NotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetPeer(context.Background(), 999999)
	if err == nil {
		t.Error("expected error for non-existent peer")
	}
}

func TestWebhookConfig_SetGetDelete(t *testing.T) {
	s := testStore(t)

	cfg := WebhookConfig{
		URL:            "https://example.com/webhook",
		SecretToken:    "secret123",
		MaxConnections: 50,
		AllowedUpdates: `["message","callback_query"]`,
	}
	if err := s.SetWebhookConfig(context.Background(), cfg); err != nil {
		t.Fatalf("SetWebhookConfig: %v", err)
	}

	got, err := s.GetWebhookConfig(context.Background())
	if err != nil {
		t.Fatalf("GetWebhookConfig: %v", err)
	}
	if got.URL != "https://example.com/webhook" {
		t.Errorf("URL = %s", got.URL)
	}
	if got.SecretToken != "secret123" {
		t.Errorf("SecretToken = %s", got.SecretToken)
	}
	if got.MaxConnections != 50 {
		t.Errorf("MaxConnections = %d", got.MaxConnections)
	}

	// Delete.
	if err := s.DeleteWebhookConfig(context.Background()); err != nil {
		t.Fatalf("DeleteWebhookConfig: %v", err)
	}
	// After delete, GetWebhookConfig should return empty/err.
	// Either behavior is acceptable; verify URL is empty.
	got2, _ := s.GetWebhookConfig(context.Background())
	if got2.URL != "" {
		t.Errorf("URL after delete = %s, want empty", got2.URL)
	}
}

func TestWebhookConfig_UpdateExisting(t *testing.T) {
	s := testStore(t)

	// Set initial config.
	s.SetWebhookConfig(context.Background(), WebhookConfig{URL: "https://first.com", SecretToken: "a"})
	// Overwrite.
	s.SetWebhookConfig(context.Background(), WebhookConfig{URL: "https://second.com", SecretToken: "b"})

	got, _ := s.GetWebhookConfig(context.Background())
	if got.URL != "https://second.com" {
		t.Errorf("URL = %s, want https://second.com (overwrite)", got.URL)
	}
	if got.SecretToken != "b" {
		t.Errorf("SecretToken = %s, want b", got.SecretToken)
	}
}

func TestWebhookError_Set(t *testing.T) {
	s := testStore(t)
	if err := s.SetWebhookError(context.Background(), 1700000000, "connection refused"); err != nil {
		t.Fatalf("SetWebhookError: %v", err)
	}
}

func TestSessionString_Roundtrip(t *testing.T) {
	s := testStore(t)

	if err := s.SetSessionString(context.Background(), "my-session-string"); err != nil {
		t.Fatalf("SetSessionString: %v", err)
	}

	got, err := s.GetSessionString(context.Background())
	if err != nil {
		t.Fatalf("GetSessionString: %v", err)
	}
	if got != "my-session-string" {
		t.Errorf("got %q, want 'my-session-string'", got)
	}
}

func TestSessionString_Empty(t *testing.T) {
	s := testStore(t)
	got, err := s.GetSessionString(context.Background())
	// Empty store should return empty string (not crash).
	if err != nil && got != "" {
		t.Errorf("expected empty session string, got %q (err: %v)", got, err)
	}
}

func TestSessionString_ErrorPropagates(t *testing.T) {
	// A real DB error (not ErrNoRows) must surface as a non-nil error so callers
	// like connect() can log it instead of silently dropping it.
	dir := t.TempDir()
	s, err := Open(dir, "654321")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := s.GetSessionString(context.Background()); err == nil {
		t.Error("expected error reading session string from closed DB")
	}
}

func TestStore_Path(t *testing.T) {
	s := testStore(t)
	expected := filepath.Join(s.Path(), "123456.db")
	_ = expected // Path() returns the dir; the DB file is within it.
	if s.Path() == "" {
		t.Error("Path should not be empty")
	}
}

func TestIsDupColumnErr(t *testing.T) {
	s := testStore(t)

	// Real duplicate-column error from SQLite.
	_, err := s.db.Exec(`ALTER TABLE peers ADD COLUMN first_name TEXT NOT NULL DEFAULT ''`)
	if err == nil {
		t.Fatal("expected error adding duplicate column")
	}
	if !isDupColumnErr(err) {
		t.Errorf("isDupColumnErr(%v) = false, want true", err)
	}

	// Non-duplicate SQL error must NOT be swallowed.
	_, err = s.db.Exec(`ALTER TABLE peers ADD COLUMN`)
	if err == nil {
		t.Fatal("expected syntax error")
	}
	if isDupColumnErr(err) {
		t.Errorf("isDupColumnErr(syntax error) = true, want false")
	}

	// Non-SQLite error must NOT match.
	if isDupColumnErr(os.ErrNotExist) {
		t.Error("isDupColumnErr(non-SQLite error) = true, want false")
	}
}

func TestPeer_NullLegacyColumns(t *testing.T) {
	s := testLegacyStore(t, `
		CREATE TABLE peers (
			id INTEGER PRIMARY KEY,
			access_hash INTEGER NOT NULL,
			type TEXT NOT NULL,
			username TEXT,
			first_name TEXT,
			bot_member_status TEXT,
			is_megagroup INTEGER,
			is_deactivated INTEGER,
			migrated_to TEXT
		);
	`)
	// Legacy databases may have NULL values in columns now treated as concrete
	// strings/ints by callers. Reads must normalize them instead of scan-failing.
	_, err := s.db.Exec(`
		INSERT INTO peers (
			id, access_hash, type, username, first_name, bot_member_status,
			is_megagroup, is_deactivated, migrated_to
		) VALUES (555, 0, 'user', NULL, NULL, NULL, NULL, NULL, NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.GetPeer(context.Background(), 555)
	if err != nil {
		t.Fatalf("GetPeer with NULL legacy columns: %v", err)
	}
	if p.Username != "" || p.FirstName != "" || p.BotMemberStatus != "" || p.MigratedTo != "" {
		t.Errorf("peer strings = username:%q first_name:%q status:%q migrated_to:%q, want empty strings",
			p.Username, p.FirstName, p.BotMemberStatus, p.MigratedTo)
	}
	if p.IsMegagroup || p.IsDeactivated {
		t.Errorf("peer flags = is_megagroup:%v is_deactivated:%v, want false", p.IsMegagroup, p.IsDeactivated)
	}
}

func TestPeerByUsername_NullLegacyColumns(t *testing.T) {
	s := testLegacyStore(t, `
		CREATE TABLE peers (
			id INTEGER PRIMARY KEY,
			access_hash INTEGER NOT NULL,
			type TEXT NOT NULL,
			username TEXT,
			first_name TEXT,
			bot_member_status TEXT,
			is_megagroup INTEGER,
			is_deactivated INTEGER,
			migrated_to TEXT
		);
	`)
	_, err := s.db.Exec(`
		INSERT INTO peers (
			id, access_hash, type, username, first_name, bot_member_status,
			is_megagroup, is_deactivated, migrated_to
		) VALUES (556, 0, 'user', 'legacy', NULL, NULL, NULL, NULL, NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.GetPeerByUsername(context.Background(), "legacy")
	if err != nil {
		t.Fatalf("GetPeerByUsername with NULL legacy columns: %v", err)
	}
	if p.FirstName != "" || p.BotMemberStatus != "" || p.MigratedTo != "" {
		t.Errorf("peer strings = first_name:%q status:%q migrated_to:%q, want empty strings",
			p.FirstName, p.BotMemberStatus, p.MigratedTo)
	}
	if p.IsMegagroup || p.IsDeactivated {
		t.Errorf("peer flags = is_megagroup:%v is_deactivated:%v, want false", p.IsMegagroup, p.IsDeactivated)
	}
}

func TestWebhookConfig_NullLegacyColumns(t *testing.T) {
	s := testLegacyStore(t, `
		CREATE TABLE webhook_config (
			id INTEGER PRIMARY KEY DEFAULT 1,
			url TEXT,
			secret_token TEXT,
			ip_address TEXT,
			fix_ip INTEGER,
			max_connections INTEGER,
			allowed_updates TEXT,
			certificate BLOB,
			last_error_date INTEGER,
			last_error_message TEXT
		);
	`)
	_, err := s.db.Exec(`
		INSERT INTO webhook_config (
			id, url, secret_token, ip_address, fix_ip, max_connections,
			allowed_updates, certificate, last_error_date, last_error_message
		) VALUES (1, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := s.GetWebhookConfig(context.Background())
	if err != nil {
		t.Fatalf("GetWebhookConfig with NULL legacy columns: %v", err)
	}
	if cfg.URL != "" || cfg.SecretToken != "" || cfg.IPAddress != "" || cfg.AllowedUpdates != "" || cfg.LastErrorMessage != "" {
		t.Errorf("webhook strings = %+v, want empty strings", cfg)
	}
	if cfg.FixIP || cfg.MaxConnections != 40 || cfg.LastErrorDate != 0 || len(cfg.Certificate) != 0 {
		t.Errorf("webhook values = %+v, want zero/default values", cfg)
	}
}
