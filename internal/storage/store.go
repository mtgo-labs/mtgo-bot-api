// Package storage implements a custom SQLite-backed persistence layer for the
// Bot API server, modeled on tdlib/telegram-bot-api. It owns:
//   - bot session strings (auth key/DC/user) so connections survive restarts,
//   - a peer cache (id↔access_hash) for raw TL InputPeer construction,
//   - (US3) the TQueue update queue via a StorageCallback,
//   - (US6) webhook configuration.
//
// It deliberately does NOT depend on github.com/mtgo-labs/storage: the mtgo
// client runs with in-memory session storage at runtime, and we persist the
// session via session-string export/restore into this store.
//
// Per-bot file: <dir>/<bot_id>/bot.db  (mirrors tdlib's per-bot working dir).
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

	"modernc.org/sqlite"
)

// Store is the per-bot SQLite persistence handle. It holds two connections
// to the same file: db (writer, serial) and rdb (reader, concurrent WAL).
type Store struct {
	db    *sql.DB // writer (serial)
	rdb   *sql.DB // reader (concurrent WAL reads)
	path  string
	botID string
	mu    sync.Mutex
}

// ValidBotID reports whether s is a safe bot ID for use in filesystem paths:
// a positive integer with no leading zero, path separators, dots, or
// non-digit characters. This prevents path traversal via filepath.Join.
func ValidBotID(s string) bool {
	if s == "" || s[0] == '0' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Open creates/opens the per-bot SQLite database, creating the directory and
// schema if needed. botID is the numeric bot user id (token prefix).
func Open(dir, botID string) (*Store, error) {
	if !ValidBotID(botID) {
		return nil, fmt.Errorf("storage: invalid bot id: %q", botID)
	}
	dbDir := filepath.Join(dir, botID)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create bot dir: %w", err)
	}
	path := filepath.Join(dbDir, "bot.db")
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite serial writers; reads via WAL.

	// Reader connection: read-only, multiple concurrent connections.
	// query_only(1) prevents accidental writes; WAL allows readers to run
	// concurrently with the writer without blocking.
	rdb, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=query_only(1)")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage: open reader %s: %w", path, err)
	}
	rdb.SetMaxOpenConns(4)

	s := &Store{db: db, rdb: rdb, path: path, botID: botID}
	if err := s.initSchema(); err != nil {
		_ = rdb.Close()
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes both the reader and writer database connections.
func (s *Store) Close() error {
	_ = s.rdb.Close()
	return s.db.Close()
}

// Path returns the on-disk database path.
func (s *Store) Path() string { return s.path }

func (s *Store) initSchema() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("storage: begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("storage: init schema: %w", err)
	}
	// Idempotent ADD COLUMN migrations. SQLite has no ADD COLUMN IF NOT EXISTS
	// before 3.35; duplicate-column errors are expected and skipped.
	migrations := []string{
		`ALTER TABLE webhook_config ADD COLUMN certificate BLOB`,
		`ALTER TABLE webhook_config ADD COLUMN ip_address TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE webhook_config ADD COLUMN fix_ip INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE peers ADD COLUMN first_name TEXT NOT NULL DEFAULT ''`,
		// Chat-access pre-flight state (mirrors the official server's check_chat_access):
		// the bot's own membership status + chat-type/liveness flags, populated from
		// updates + getChat. Empty/zero = unknown → the pre-flight falls through.
		`ALTER TABLE peers ADD COLUMN bot_member_status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE peers ADD COLUMN is_megagroup INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE peers ADD COLUMN is_deactivated INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE peers ADD COLUMN migrated_to TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range migrations {
		if _, err := tx.Exec(stmt); err != nil && !isDupColumnErr(err) {
			return fmt.Errorf("storage: migrate: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit migration: %w", err)
	}
	return nil
}

// isDupColumnErr reports whether err is a SQLite "duplicate column name"
// error, which indicates an idempotent ADD COLUMN migration is already applied.
// Uses errors.As to confirm the error originated from SQLite before checking
// the message — a non-SQLite error with the same substring is never swallowed.
func isDupColumnErr(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return strings.Contains(sqliteErr.Error(), "duplicate column name")
}

const schema = `
CREATE TABLE IF NOT EXISTS session_strings (
    bot_id          TEXT PRIMARY KEY,
    session_string  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS peers (
    id           INTEGER PRIMARY KEY,
    access_hash  INTEGER NOT NULL,
    type         TEXT    NOT NULL,
    username     TEXT,
    first_name   TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_peers_username ON peers(username);

CREATE TABLE IF NOT EXISTS webhook_config (
    id                 INTEGER PRIMARY KEY DEFAULT 1,
    url                TEXT    NOT NULL,
    secret_token       TEXT    NOT NULL DEFAULT '',
    ip_address         TEXT    NOT NULL DEFAULT '',
    fix_ip             INTEGER NOT NULL DEFAULT 0,
    max_connections    INTEGER NOT NULL DEFAULT 40,
    allowed_updates    TEXT    NOT NULL DEFAULT '',
    certificate        BLOB,
    last_error_date    INTEGER NOT NULL DEFAULT 0,
    last_error_message TEXT    NOT NULL DEFAULT ''
);
`

// --- Session string persistence (replaces mtgo session storage) ---

func (s *Store) GetSessionString(ctx context.Context) (string, error) {
	var ss string
	err := s.rdb.QueryRowContext(ctx, `SELECT session_string FROM session_strings WHERE bot_id = ?`, s.botID).Scan(&ss)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("storage: get session string: %w", err)
	}
	return ss, nil
}

func (s *Store) SetSessionString(ctx context.Context, ss string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO session_strings (bot_id, session_string) VALUES (?, ?)
		 ON CONFLICT(bot_id) DO UPDATE SET session_string = excluded.session_string`,
		s.botID, ss,
	)
	if err != nil {
		return fmt.Errorf("storage: set session string: %w", err)
	}
	return nil
}

// --- Peer cache (for raw TL InputPeer construction) ---

// PeerType enumerates peer kinds stored in the cache.
type PeerType string

const (
	PeerTypeUser    PeerType = "user"
	PeerTypeChat    PeerType = "chat"
	PeerTypeChannel PeerType = "channel"
)

// Peer is a cached peer (id + access_hash + type) for building InputPeer/InputUser.
type Peer struct {
	ID         int64
	AccessHash int64
	Type       PeerType
	Username   string
	FirstName  string

	// Chat-access pre-flight state (mirrors check_chat_access). Zero/empty = unknown.
	BotMemberStatus string // "" | member | left | kicked | banned | administrator | creator
	IsMegagroup     bool   // channel is a supergroup (selects "supergroup" vs "channel" suffix)
	IsDeactivated   bool   // group deactivated (deleted) / user deactivated
	MigratedTo      string // group upgraded to this supergroup (Bot API chat_id string)
}

// SavePeer upserts a peer into the cache.
func (s *Store) SavePeer(ctx context.Context, p Peer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO peers (id, access_hash, type, username, first_name) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET access_hash=excluded.access_hash, type=excluded.type, username=excluded.username, first_name=CASE WHEN excluded.first_name != '' THEN excluded.first_name ELSE first_name END`,
		p.ID, p.AccessHash, string(p.Type), p.Username, p.FirstName,
	)
	return err
}

// GetPeer returns the cached peer by id, or sql.ErrNoRows.
func (s *Store) GetPeer(ctx context.Context, id int64) (Peer, error) {
	var p Peer
	var typ string
	var isMegagroup, isDeactivated int
	err := s.rdb.QueryRowContext(ctx,
		`SELECT id, access_hash, type, COALESCE(username, ''), COALESCE(first_name, ''), COALESCE(bot_member_status, ''), COALESCE(is_megagroup, 0), COALESCE(is_deactivated, 0), COALESCE(migrated_to, '') FROM peers WHERE id = ?`, id,
	).Scan(&p.ID, &p.AccessHash, &typ, &p.Username, &p.FirstName, &p.BotMemberStatus, &isMegagroup, &isDeactivated, &p.MigratedTo)
	if err == nil {
		p.Type = PeerType(typ)
		p.IsMegagroup = isMegagroup != 0
		p.IsDeactivated = isDeactivated != 0
	}
	return p, err
}

// GetPeerByUsername returns the cached peer by username (without leading @), or sql.ErrNoRows.
func (s *Store) GetPeerByUsername(ctx context.Context, username string) (Peer, error) {
	var p Peer
	var typ string
	var isMegagroup, isDeactivated int
	err := s.rdb.QueryRowContext(ctx,
		`SELECT id, access_hash, type, COALESCE(username, ''), COALESCE(first_name, ''), COALESCE(bot_member_status, ''), COALESCE(is_megagroup, 0), COALESCE(is_deactivated, 0), COALESCE(migrated_to, '') FROM peers WHERE username = ?`, username,
	).Scan(&p.ID, &p.AccessHash, &typ, &p.Username, &p.FirstName, &p.BotMemberStatus, &isMegagroup, &isDeactivated, &p.MigratedTo)
	if err == nil {
		p.Type = PeerType(typ)
		p.IsMegagroup = isMegagroup != 0
		p.IsDeactivated = isDeactivated != 0
	}
	return p, err
}

// SaveChatFlags updates the cached chat-type/liveness flags for a peer (from the
// Chats map in updates/getChat). Does not touch access_hash or bot_member_status.
// No-op if the peer isn't cached (callers cache peers before flagging them).
func (s *Store) SaveChatFlags(ctx context.Context, id int64, isMegagroup, isDeactivated bool, migratedTo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE peers SET is_megagroup=?, is_deactivated=?, migrated_to=? WHERE id=?`,
		isMegagroup, isDeactivated, migratedTo, id)
	return err
}

// SaveBotMemberStatus records the bot's own membership status in a chat (from a
// participant update or getChatMember). No-op when status is empty.
func (s *Store) SaveBotMemberStatus(ctx context.Context, id int64, status string) error {
	if status == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE peers SET bot_member_status=? WHERE id=?`, status, id)
	return err
}

// --- Webhook config ---

// WebhookConfig holds the per-bot webhook configuration (mirrors Bot API
// setWebhook parameters + getWebhookInfo status fields).
type WebhookConfig struct {
	URL              string
	SecretToken      string
	IPAddress        string
	FixIP            bool // deliver to IPAddress without DNS re-resolve
	MaxConnections   int
	AllowedUpdates   string // JSON array
	Certificate      []byte // optional PEM-encoded self-signed cert
	LastErrorDate    int64
	LastErrorMessage string
}

// SetWebhookConfig upserts the webhook config for this bot.
func (s *Store) SetWebhookConfig(ctx context.Context, cfg WebhookConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhook_config (id, url, secret_token, ip_address, fix_ip, max_connections, allowed_updates, certificate, last_error_date, last_error_message)
		 VALUES (1, ?, ?, ?, ?, ?, ?, ?, 0, '')
		 ON CONFLICT(id) DO UPDATE SET url=excluded.url, secret_token=excluded.secret_token,
		     ip_address=excluded.ip_address, fix_ip=excluded.fix_ip, max_connections=excluded.max_connections, allowed_updates=excluded.allowed_updates,
		     certificate=excluded.certificate`,
		cfg.URL, cfg.SecretToken, cfg.IPAddress, cfg.FixIP, cfg.MaxConnections, cfg.AllowedUpdates, cfg.Certificate,
	)
	return err
}

// GetWebhookConfig returns the stored webhook config, or sql.ErrNoRows.
func (s *Store) GetWebhookConfig(ctx context.Context) (WebhookConfig, error) {
	var cfg WebhookConfig
	var cert []byte
	var fixIP int
	err := s.rdb.QueryRowContext(ctx,
		`SELECT COALESCE(url, ''), COALESCE(secret_token, ''), COALESCE(ip_address, ''), COALESCE(fix_ip, 0), COALESCE(max_connections, 40), COALESCE(allowed_updates, ''), COALESCE(certificate, X''), COALESCE(last_error_date, 0), COALESCE(last_error_message, '')
		 FROM webhook_config WHERE id = 1`).Scan(
		&cfg.URL, &cfg.SecretToken, &cfg.IPAddress, &fixIP, &cfg.MaxConnections, &cfg.AllowedUpdates, &cert,
		&cfg.LastErrorDate, &cfg.LastErrorMessage,
	)
	cfg.FixIP = fixIP != 0
	cfg.Certificate = cert
	return cfg, err
}

// DeleteWebhookConfig removes the webhook config for this bot.
func (s *Store) DeleteWebhookConfig(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhook_config WHERE id = 1`)
	return err
}

// SetWebhookError records the last delivery error.
func (s *Store) SetWebhookError(ctx context.Context, date int64, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`UPDATE webhook_config SET last_error_date = ?, last_error_message = ? WHERE id = 1`,
		date, message,
	)
	return err
}
