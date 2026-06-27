// Package client implements the per-bot Bot API method handlers. Each handler
// receives a parsed Query, invokes the raw tg RPC layer, and returns a Bot API
// JSON response. This mirrors telegram-bot-api's Client.cpp: one large package
// where all handlers share unexported Client state (rpc, store, mu, me).
// The deliberate god-package structure mirrors the reference implementation.
package client

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mtgo-labs/mtgo/telegram"

	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
	"github.com/mtgo-labs/mtgo-bot-api/internal/webhook"
)

// Params holds the per-bot connection configuration threaded from the Manager.
type Params struct {
	APIID     int32
	APIHash   string
	Dir       string // working directory for per-bot bot.db
	TempDir   string // temp directory for file downloads/uploads
	TestDC    bool
	LocalMode bool           // --local: absolute file_path, no 20 MB download cap
	TQueue    *tqueue.TQueue // shared global update queue (owned by Manager)
	// MsgCacheCap bounds the per-bot message cache. Zero uses the default.
	MsgCacheCap int
	// StartTime is the server process start time (parameters_->start_time_ in
	// the reference). It is shared across all clients and used by the close
	// anti-abuse gate (Client.cpp:13301-13308).
	StartTime time.Time
}

// Client holds the per-bot state for one token: a live telegram.Client (owning
// the connection/session/bot auth), its rpcInvoker (a *tg.RPCClient in
// production, a fakeRPC in tests), the on-disk Store, and the cached bot
// identity. All Telegram calls go through rpc (raw tg).
type Client struct {
	Token  string
	TestDC bool
	botID  string
	params Params

	startTime time.Time // when this client was constructed (Client::start_time_)

	mu                        sync.Mutex
	conn                      *telegram.Client
	rpc                       rpcInvoker
	store                     *storage.Store
	me                        *apitypes.User
	msgs                      *msgCache
	ready                     bool
	connErr                   error
	deliverer                 *webhook.Deliverer
	allowedUpdates            map[string]bool // nil=default exclusions; non-nil=explicit allowlist
	webhookSetBusy            bool
	nextSetWebhook            time.Time
	nextBotUpdatesWarningTime time.Time
	wasBotUpdatesWarning      bool
	// Long-poll coordination (A3): only one long poll per bot; a new getUpdates
	// with a timeout aborts the prior (Client.cpp abort_long_poll 16959,17121).
	longPollMu                 sync.Mutex
	longPollCancel             context.CancelFunc
	longPollConflict           chan longPollConflict
	previousGetUpdatesOffset   int64
	previousGetUpdatesStart    time.Time
	previousGetUpdatesFinish   time.Time
	nextGetUpdatesConflictTime time.Time
	// floodBuckets is the per-total-file-size token bucket for the upload flood
	// limiter (Client.cpp:13327-13350). Keyed by total uploaded bytes; value is
	// the monotonic "next allowed send" time in seconds. Guarded by mu.
	floodBuckets map[int64]float64

	// Lifecycle state (Client.cpp logging_out_ / closing_ / clear_tqueue_),
	// read on every dispatch so set atomically to keep the hot path lock-free.
	// See closingError / fail_query_closing (Client.cpp:16987-17032).
	closing    atomic.Bool // td_api::close processed
	loggingOut atomic.Bool // auth.logOut sent
	loggedOut  atomic.Bool // clear_tqueue_: queue cleared post-logout
}

// NewClient builds an unconnected Client for the given token.
func NewClient(params Params, token string) *Client {
	return &Client{
		Token:        token,
		params:       params,
		TestDC:       params.TestDC,
		botID:        tokenID(token),
		msgs:         newMsgCache(params.MsgCacheCap),
		startTime:    time.Now(),
		floodBuckets: make(map[int64]float64),
	}
}

// Stop gracefully shuts down the client: stops the webhook deliverer (if any)
// and closes the Telegram connection. Safe to call multiple times.
func (c *Client) Stop() {
	_ = c.StopContext(context.Background())
}

// StopContext gracefully shuts down the client, returning ctx.Err if webhook
// delivery doesn't stop before the context is done. Telegram connection stop is
// launched asynchronously because mtgo exposes it as a blocking call without a
// context-aware variant.
func (c *Client) StopContext(ctx context.Context) error {
	c.mu.Lock()
	d := c.deliverer
	conn := c.conn
	c.mu.Unlock()
	var err error
	if d != nil {
		err = d.StopContext(ctx)
	}
	if conn != nil {
		go conn.Stop()
	}
	return err
}

// ConnFailed reports whether the client attempted to connect and failed.
// Used by Manager for eviction of dead entries.
func (c *Client) ConnFailed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ready && c.connErr != nil
}

// Store returns the per-bot store, opening the connection if needed.
func (c *Client) Store(ctx context.Context) (*storage.Store, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}
	return c.store, nil
}

// ensureConnected lazily establishes the Telegram connection once. Subsequent
// calls are no-ops (success) or return the cached connect error.
func (c *Client) ensureConnected(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ready {
		return c.connErr
	}
	c.ready = true
	c.conn, c.rpc, c.store, c.connErr = connect(ctx, c.params, c.Token, c.botID)
	if c.connErr == nil && c.conn != nil {
		// Register the update-ingestion handler so incoming updates populate
		// the peer cache and feed getUpdates (mirrors how TDLib pushes updates
		// to the Client actor in telegram-bot-api).
		c.registerIngestion()
		// Preload the bot's own identity so the From field of the very first
		// outgoing message is fully populated (mirrors the reference, which
		// knows the bot from auth). Best-effort: getMe retries on failure.
		if err := c.loadMeLocked(ctx); err != nil {
			botlog.Warn("client %s: failed to preload bot identity: %v", c.botID, err)
		}
		// Warm the peer cache from the bot's dialogs so a cold start can resolve
		// chats the bot is a member of but has no cached access hash for (H1).
		// Best-effort; live updates + on-demand resolution still populate it.
		c.warmPeerCache(ctx)
		// Restore a previously-configured webhook so delivery survives restarts
		// (mirrors ClientManager loading webhooks_db.binlog on boot).
		c.restoreWebhookLocked()
	}
	return c.connErr
}

// tokenID returns the bot user id (token prefix before ":").
func tokenID(token string) string {
	if before, _, ok := strings.Cut(token, ":"); ok {
		return before
	}
	return token
}

// queueID returns the deterministic per-bot queue id. Mirrors
// ClientManager::get_tqueue_id = user_id + (is_test_dc << 54).
func (c *Client) queueID() tqueue.QueueID {
	uid, _ := strconv.ParseInt(c.botID, 10, 64)
	return tqueue.QueueIDFor(uid, c.TestDC)
}

// connectTimeout is the max time to establish the MTProto connection.
const connectTimeout = 60 * time.Second
