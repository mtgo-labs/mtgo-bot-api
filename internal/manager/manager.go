package manager

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/client"
	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

// Parameters holds process-wide configuration shared by all bots. Mirrors
// telegram-bot-api ClientParameters.h.
type Parameters struct {
	APIID                  int32
	APIHash                string
	LocalMode              bool
	WorkingDir             string
	TempDir                string
	DefaultMaxWebhookConns int
	MaxClients             int // max concurrent bot clients (0 = unlimited)
	MsgCacheCap            int // per-bot message cache cap (0 = client default)
}

// Manager routes parsed queries to per-bot Clients by token, creating a Client
// on first use. It owns the global update queue (TQueue), shared across all
// bots and keyed by queue_id, mirroring telegram-bot-api/ClientManager where
// parameters_->shared_data_->tqueue_ is a single TQueue for the whole process.
type Manager struct {
	params  *Parameters
	tq      *tqueue.TQueue
	tqStore *storage.TQueueStore

	startTime time.Time // process start, threaded into each client.Params.StartTime

	mu      sync.Mutex
	clients map[string]*client.Client

	gcStarted chan struct{} // closed by StartGC
	gcStop    chan struct{} // closed by Close to stop the GC loop
	gcDone    chan struct{} // closed when the GC loop has exited
	closeOnce sync.Once
	closeErr  error
}

// New builds a Manager and opens the global update queue, replaying any
// persisted events so getUpdates resumes across restarts (mirrors
// ClientManager.cpp:317-356).
func New(params *Parameters) (*Manager, error) {
	store, err := storage.OpenTQueue(params.WorkingDir)
	if err != nil {
		return nil, err
	}
	tq := tqueue.New()

	events, err := store.LoadAll(context.Background())
	if err != nil {
		botlog.Error("manager: load tqueue events: %v", err)
	}
	if len(events) > 0 {
		tq.Replay(events)
		botlog.Info("manager: replayed %d tqueue events", len(events))
	}
	tq.SetCallback(store)

	return &Manager{
		params:    params,
		tq:        tq,
		tqStore:   store,
		startTime: time.Now(),
		clients:   make(map[string]*client.Client),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}, nil
}

// TQueue returns the global update queue.
func (m *Manager) TQueue() *tqueue.TQueue { return m.tq }

// tqueueGCInterval is how often expired update events are dropped from the
// in-memory queue and the tqueue store. The official server runs GC on a
// scheduler; this is a conservative cadence.
const tqueueGCInterval = 60 * time.Second

// StartGC launches the periodic TQueue garbage collector (drops events whose
// expires_at has passed from memory and the tqueue store). Without it, expired
// events are filtered at read but persist in tqueue.db forever and reload on
// every restart (mirrors TQueue::run_gc, called by the tdlib scheduler).
func (m *Manager) StartGC() {
	select {
	case <-m.gcStarted:
		return // already started
	default:
	}
	close(m.gcStarted)
	go m.gcLoop()
}

func (m *Manager) gcLoop() {
	defer close(m.gcDone)
	ticker := time.NewTicker(tqueueGCInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.gcStop:
			return
		case now := <-ticker.C:
			unix := int32(now.Unix())
			if _, complete := m.tq.RunGC(unix); !complete {
				botlog.Error("manager: tqueue GC did not complete")
			}
			if m.tqStore != nil {
				if _, err := m.tqStore.GC(context.Background(), unix); err != nil {
					botlog.Error("manager: tqueue store GC: %v", err)
				}
			}
		}
	}
}

// Close stops the GC loop (if started) and closes the global queue store.
func (m *Manager) Close() error { return m.CloseContext(context.Background()) }

// CloseContext stops the GC loop and closes the global queue store, returning
// ctx.Err if shutdown waits longer than the caller's deadline.
func (m *Manager) CloseContext(ctx context.Context) error {
	waitErr := make(chan error, 1)
	m.closeOnce.Do(func() {
		select {
		case <-m.gcStarted:
			close(m.gcStop)
			select {
			case <-m.gcDone:
			case <-ctx.Done():
				waitErr <- ctx.Err()
				return
			}
		default:
		}
		if m.tqStore != nil {
			m.closeErr = m.tqStore.Close()
		}
	})
	select {
	case err := <-waitErr:
		return err
	default:
	}
	return m.closeErr
}

// Shutdown stops all bot clients (deliverers + connections), then calls Close.
// Use this instead of Close for graceful process exit.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	clients := make([]*client.Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.mu.Unlock()
	var errs []error
	for _, c := range clients {
		if err := c.StopContext(ctx); err != nil {
			errs = append(errs, err)
			break
		}
	}
	if err := m.CloseContext(ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Handle implements server.Handler: route to the bot Client and dispatch.
func (m *Manager) Handle(ctx context.Context, q *server.Query) (status int, body []byte) {
	if code, desc, ok := validateToken(q.Token); !ok {
		return code, response.Fail(code, desc, nil)
	}
	c := m.clientFor(q.Token, q.TestDC)
	if c == nil {
		return 429, response.Fail(429, "Too Many Requests: too many concurrent bots", &response.Parameters{RetryAfter: 10})
	}
	return c.Dispatch(ctx, q)
}

const (
	invalidTokenDescription         = "Unauthorized: invalid token specified"
	unallowedTokenDescription       = "Misdirected Request: unallowed token specified"
	maxBotUserIDExclusive     int64 = 1 << 54
)

func validateToken(token string) (code int, description string, ok bool) {
	if token == "" || token[0] == '0' || len(token) > 80 || strings.Contains(token, "/") ||
		!strings.Contains(token, ":") {
		return 401, invalidTokenDescription, false
	}
	prefix, _, _ := strings.Cut(token, ":")
	// Defense-in-depth: reject path separators and traversal sequences in
	// the bot ID prefix. ParseInt below already rejects non-numeric input,
	// but this makes the intent explicit and guards against future changes.
	if strings.ContainsAny(prefix, `/\`) || prefix == "." || prefix == ".." {
		return 401, invalidTokenDescription, false
	}
	userID, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil {
		return 421, unallowedTokenDescription, false
	}
	if userID <= 0 || userID >= maxBotUserIDExclusive {
		return 401, invalidTokenDescription, false
	}
	return 0, "", true
}

// defaultMaxClients is the fallback when Parameters.MaxClients is 0.
const defaultMaxClients = 1000

// clientFor returns the existing or newly-created Client for a token.
// If the client map exceeds MaxClients, dead entries (failed connections)
// are evicted first; if still full, nil is returned and the caller rejects
// with 429.
func (m *Manager) clientFor(token string, testDC bool) *client.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := token
	if testDC {
		key += "/test"
	}
	if c, ok := m.clients[key]; ok {
		return c
	}
	// Evict dead entries (failed-to-connect clients) before checking the cap.
	max := m.params.MaxClients
	if max == 0 {
		max = defaultMaxClients
	}
	if len(m.clients) >= max {
		for k, c := range m.clients {
			if c.ConnFailed() {
				c.Stop()
				delete(m.clients, k)
			}
		}
	}
	if len(m.clients) >= max {
		return nil // caller rejects with 429
	}
	c := client.NewClient(client.Params{
		APIID:       m.params.APIID,
		APIHash:     m.params.APIHash,
		Dir:         m.params.WorkingDir,
		TempDir:     m.params.TempDir,
		TestDC:      testDC,
		LocalMode:   m.params.LocalMode,
		TQueue:      m.tq,
		MsgCacheCap: m.params.MsgCacheCap,
		StartTime:   m.startTime,
	}, token)
	m.clients[key] = c
	return c
}
