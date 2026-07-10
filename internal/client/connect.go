package client

import (
	"context"

	"github.com/mtgo-labs/mtgo/telegram"
	"github.com/mtgo-labs/mtgo/tg"

	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

// connect creates a telegram.Client backed by in-memory session storage at
// runtime (Storage=nil), restores any persisted session string from our own
// SQLite store, connects/authenticates the bot, then exports the fresh session
// string back to disk. Returns the raw *tg.RPCClient for Bot API calls.
// in-memory, and we own persistence via session-string export/restore.
func connect(ctx context.Context, p Params, token, botID string) (*telegram.Client, *tg.RPCClient, *storage.Store, error) {
	store, err := storage.Open(p.Dir, botID)
	if err != nil {
		return nil, nil, nil, err
	}
	sessStr, err := store.GetSessionString(ctx)
	if err != nil {
		// Non-fatal: an empty session just means fresh auth on next connect.
		botlog.Warn("client %s: failed to load session string: %v", botID, err)
	}

	cfg := &telegram.Config{
		BotToken: token,
		TestMode: p.TestDC,
		// File-based session — matches the echo_bot pattern. The session file
		// is created in the working dir as <botID>.session and persists the
		// auth key across restarts. Removed InMemory+SessionString which caused
		// silent connection failures when the session string was corrupted.
		SessionName: botID,
		SessionString: sessStr, // restore if available (migration from old format)
		SavePeers:     false,   // we maintain our own peer cache
		// Auto-reconnect + health pings. These must be set
		// explicitly because mtgo's mergeConfig blind-copies them (resetting
		// DefaultConfig's true → false). Without them, a dropped MTProto
		// connection never recovers and every RPC fails until restart.
		ReconnectEnabled: true,
		HealthEnabled:    true,
	}
	cl, err := telegram.NewClient(p.APIID, p.APIHash, cfg)
	if err != nil {
		_ = store.Close()
		return nil, nil, nil, err
	}
	if err := cl.Connect(connectTimeout); err != nil {
		_ = store.Close()
		return nil, nil, nil, err
	}
	rpc := cl.RPC()

	// Persist the (possibly updated) session string so the bot authenticates
	// instantly on the next restart.
	if fresh, err := cl.ExportSessionString(); err == nil && fresh != "" && fresh != sessStr {
		if err := store.SetSessionString(ctx, fresh); err != nil {
			botlog.Error("client %s: failed to persist session string: %v", botID, err)
		}
	}
	return cl, rpc, store, nil
}
