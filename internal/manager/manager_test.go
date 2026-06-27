package manager

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/client"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

// ---------------------------------------------------------------------------
// Existing tests (GC / Close lifecycle, Handle rejection of invalid tokens)
// ---------------------------------------------------------------------------

// TestManagerGCStartStop validates the A1 wiring: StartGC launches the GC loop
// (and is idempotent), and Close stops it cleanly (gcDone closes) without
// blocking — even with a 60s ticker, the gcStop channel lets the loop exit
// immediately. The GC logic itself is covered by tqueue/storage tests.
func TestManagerGCStartStop(t *testing.T) {
	m := &Manager{
		tq:        tqueue.New(),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}

	m.StartGC()
	select {
	case <-m.gcStarted:
	default:
		t.Fatal("StartGC did not close gcStarted")
	}
	// Idempotent: a second StartGC must not panic on the double close.
	m.StartGC()

	if err := m.Close(); err != nil {
		t.Errorf("Close returned %v", err)
	}
	select {
	case <-m.gcDone:
	default:
		t.Fatal("gcDone not closed after Close (loop did not stop)")
	}
}

// TestManagerCloseWithoutGC ensures Close doesn't block when StartGC was never
// called (the gcStarted-default branch).
func TestManagerCloseWithoutGC(t *testing.T) {
	m := &Manager{
		tq:        tqueue.New(),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}
	if err := m.Close(); err != nil {
		t.Errorf("Close (no StartGC) returned %v", err)
	}
}

func TestManagerCloseIsIdempotent(t *testing.T) {
	m := &Manager{
		tq:        tqueue.New(),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}
	if err := m.Close(); err != nil {
		t.Fatalf("first Close returned %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("second Close returned %v", err)
	}
}

func TestManagerCloseContextHonorsDeadline(t *testing.T) {
	m := &Manager{
		tq:        tqueue.New(),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}
	close(m.gcStarted)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := m.CloseContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CloseContext err = %v, want context.Canceled", err)
	}
}

func TestHandleRejectsInvalidTokensBeforeClientCreation(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		status      int
		description string
	}{
		{
			name:        "empty",
			token:       "",
			status:      401,
			description: invalidTokenDescription,
		},
		{
			name:        "leading zero",
			token:       "0:abc",
			status:      401,
			description: invalidTokenDescription,
		},
		{
			name:        "too long",
			token:       "123:" + strings.Repeat("a", 77),
			status:      401,
			description: invalidTokenDescription,
		},
		{
			name:        "slash",
			token:       "123:a/b",
			status:      401,
			description: invalidTokenDescription,
		},
		{
			name:        "missing colon",
			token:       "123abc",
			status:      401,
			description: invalidTokenDescription,
		},
		{
			name:        "non integer prefix",
			token:       "abc:def",
			status:      421,
			description: unallowedTokenDescription,
		},
		{
			name:        "negative prefix",
			token:       "-1:def",
			status:      401,
			description: invalidTokenDescription,
		},
		{
			name:        "too large prefix",
			token:       "18014398509481984:def",
			status:      401,
			description: invalidTokenDescription,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testManager()
			status, body := m.Handle(context.Background(), &server.Query{
				Token:  tt.token,
				Method: "getme",
			})
			if status != tt.status {
				t.Fatalf("status = %d, want %d; body=%s", status, tt.status, body)
			}
			var got struct {
				Ok          bool   `json:"ok"`
				ErrorCode   int    `json:"error_code"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("invalid JSON: %v: %s", err, body)
			}
			if got.Ok || got.ErrorCode != tt.status || got.Description != tt.description {
				t.Fatalf("envelope = %+v, want code=%d desc=%q", got, tt.status, tt.description)
			}
			if len(m.clients) != 0 {
				t.Fatalf("invalid token created %d clients", len(m.clients))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateToken — comprehensive table-driven test
// ---------------------------------------------------------------------------

func TestValidateToken(t *testing.T) {
	maxStr := strconv.FormatInt(maxBotUserIDExclusive, 10)      // "18014398509481984"
	maxMinus1 := strconv.FormatInt(maxBotUserIDExclusive-1, 10) // "18014398509481983"

	tests := []struct {
		name     string
		token    string
		wantCode int
		wantDesc string
		wantOK   bool
	}{
		// --- Valid tokens ---
		{"valid simple", "123456:ABC-def_123", 0, "", true},
		{"valid single digit id", "1:Aa", 0, "", true},
		{"valid large id under max", maxMinus1 + ":token", 0, "", true},
		{"valid exactly 80 chars", "123:" + strings.Repeat("a", 76), 0, "", true},

		// --- Empty ---
		{"empty", "", 401, invalidTokenDescription, false},

		// --- Leading zero ---
		{"leading zero single", "0:abc", 401, invalidTokenDescription, false},
		{"leading zero multi", "01:abc", 401, invalidTokenDescription, false},

		// --- No colon ---
		{"no colon", "123abc", 401, invalidTokenDescription, false},
		{"no colon numeric only", "123", 401, invalidTokenDescription, false},

		// --- Path separators ---
		{"forward slash in suffix", "123:a/b", 401, invalidTokenDescription, false},
		{"forward slash in prefix", "1/2:abc", 401, invalidTokenDescription, false},
		{"backslash in prefix", `123\:abc`, 401, invalidTokenDescription, false},
		{"backslash and slash in prefix", `1/\2:abc`, 401, invalidTokenDescription, false},

		// --- Dot traversal ---
		{"single dot prefix", ".:abc", 401, invalidTokenDescription, false},
		{"double dot prefix", "..:abc", 401, invalidTokenDescription, false},

		// --- Non-numeric prefix ---
		{"alpha prefix", "abc:def", 421, unallowedTokenDescription, false},
		{"mixed prefix", "12a:def", 421, unallowedTokenDescription, false},

		// --- Too long (> 80 chars) ---
		{"too long 81 chars", "123:" + strings.Repeat("a", 77), 401, invalidTokenDescription, false},
		{"too long huge prefix", maxStr + "0:abc", 401, invalidTokenDescription, false},

		// --- userID <= 0 ---
		{"negative userID", "-1:def", 401, invalidTokenDescription, false},
		{"negative multi digit", "-999:def", 401, invalidTokenDescription, false},

		// --- userID >= maxBotUserIDExclusive ---
		{"userID at max", maxStr + ":abc", 401, invalidTokenDescription, false},
		{"userID above max", maxStr + "0:abc", 401, invalidTokenDescription, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, desc, ok := validateToken(tt.token)
			if ok != tt.wantOK {
				t.Fatalf("validateToken(%q) ok=%v, want %v (code=%d desc=%q)",
					tt.token, ok, tt.wantOK, code, desc)
			}
			if code != tt.wantCode {
				t.Fatalf("validateToken(%q) code=%d, want %d", tt.token, code, tt.wantCode)
			}
			if !tt.wantOK && desc != tt.wantDesc {
				t.Fatalf("validateToken(%q) desc=%q, want %q", tt.token, desc, tt.wantDesc)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// New — creates manager with tqueue store, replays events
// ---------------------------------------------------------------------------

func TestNewCreatesManager(t *testing.T) {
	dir := t.TempDir()
	m, err := New(&Parameters{WorkingDir: dir})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer m.Close()

	if m.tq == nil {
		t.Fatal("tq is nil after New")
	}
	if m.tqStore == nil {
		t.Fatal("tqStore is nil after New")
	}
	if m.clients == nil {
		t.Fatal("clients map is nil after New")
	}
	if m.TQueue() == nil {
		t.Fatal("TQueue() returned nil")
	}
	if m.startTime.IsZero() {
		t.Fatal("startTime is zero after New")
	}
	// Channels initialised.
	select {
	case <-m.gcStarted:
		t.Fatal("gcStarted should be open (not closed) after New")
	default:
	}
}

func TestNewReplaysEvents(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate the tqueue store with one event for bot 123.
	store, err := storage.OpenTQueue(dir)
	if err != nil {
		t.Fatalf("OpenTQueue: %v", err)
	}
	qid := tqueue.QueueIDFor(123, false)
	future := int32(time.Now().Add(time.Hour).Unix())
	_ = store.Push(context.Background(), qid, tqueue.RawEvent{
		ID:        tqueue.EventID(1),
		ExpiresAt: future,
		Data:      []byte(`{"update_id":1}`),
	})
	if err := store.Close(); err != nil {
		t.Fatalf("store close: %v", err)
	}

	// New should open the store, load the event, and replay it into the TQueue.
	m, err := New(&Parameters{WorkingDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()

	if got := m.TQueue().Size(qid); got != 1 {
		t.Fatalf("Size(qid) = %d, want 1 (event not replayed)", got)
	}
}

func TestNewEmptyStore(t *testing.T) {
	dir := t.TempDir()
	m, err := New(&Parameters{WorkingDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Close()
	// Empty store → no events replayed, no error.
	if got := m.TQueue().Size(tqueue.QueueIDFor(1, false)); got != 0 {
		t.Fatalf("Size = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// clientFor — new token, same token, MaxClients cap, eviction
// ---------------------------------------------------------------------------

func TestClientForNewToken(t *testing.T) {
	m := testManager()
	c := m.clientFor("100:tokenA", false)
	if c == nil {
		t.Fatal("clientFor returned nil for a new token")
	}
	if c.Token != "100:tokenA" {
		t.Fatalf("client Token = %q, want %q", c.Token, "100:tokenA")
	}
	if len(m.clients) != 1 {
		t.Fatalf("clients map has %d entries, want 1", len(m.clients))
	}
}

func TestClientForSameTokenReturnsSameClient(t *testing.T) {
	m := testManager()
	first := m.clientFor("200:tokenB", false)
	if first == nil {
		t.Fatal("first clientFor returned nil")
	}
	second := m.clientFor("200:tokenB", false)
	if second != first {
		t.Fatal("second clientFor returned a different client pointer")
	}
	if len(m.clients) != 1 {
		t.Fatalf("clients map has %d entries, want 1", len(m.clients))
	}
}

func TestClientForMaxClientsCap(t *testing.T) {
	m := &Manager{
		params:    &Parameters{MaxClients: 2},
		tq:        tqueue.New(),
		startTime: time.Now(),
		clients:   make(map[string]*client.Client),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}
	// Fill both slots.
	c1 := m.clientFor("100:aaa", false)
	c2 := m.clientFor("200:bbb", false)
	if c1 == nil || c2 == nil {
		t.Fatal("expected two clients to be created")
	}
	// Third token: map is full, no dead entries to evict → nil.
	c3 := m.clientFor("300:ccc", false)
	if c3 != nil {
		t.Fatal("clientFor should return nil when map is full")
	}
	if len(m.clients) != 2 {
		t.Fatalf("clients map has %d entries, want 2", len(m.clients))
	}
}

func TestClientForMaxClientsDefaultWhenZero(t *testing.T) {
	// MaxClients == 0 uses defaultMaxClients (1000). A single new token
	// should succeed because 1 < 1000.
	m := testManager() // MaxClients: 0
	c := m.clientFor("100:xxx", false)
	if c == nil {
		t.Fatal("clientFor returned nil with default MaxClients")
	}
}

func TestClientForEvictsDeadEntries(t *testing.T) {
	dir := t.TempDir()

	// Create a path that is a regular file (not a directory) so storage.Open
	// fails immediately without touching the network.
	badFile := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(badFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Manager{
		params:    &Parameters{MaxClients: 1, WorkingDir: dir},
		tq:        tqueue.New(),
		startTime: time.Now(),
		clients:   make(map[string]*client.Client),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}

	// Build a client that failed to connect: NewClient with an invalid dir,
	// then trigger ensureConnected via Store() which fails at storage.Open.
	dead := client.NewClient(client.Params{
		Dir:       badFile,
		TQueue:    m.tq,
		StartTime: m.startTime,
	}, "111:dead")
	if _, err := dead.Store(context.Background()); err == nil {
		t.Fatal("expected Store to fail with bad dir")
	}
	if !dead.ConnFailed() {
		t.Fatal("dead client should report ConnFailed")
	}

	// Inject the dead client to fill the single slot.
	m.clients["111:dead"] = dead

	// clientFor with a new token should evict the dead entry and succeed.
	c := m.clientFor("222:alive", false)
	if c == nil {
		t.Fatal("clientFor returned nil despite evictable dead entry")
	}
	if _, ok := m.clients["111:dead"]; ok {
		t.Fatal("dead client was not evicted from the map")
	}
	if _, ok := m.clients["222:alive"]; !ok {
		t.Fatal("new client was not stored in the map")
	}
	if len(m.clients) != 1 {
		t.Fatalf("clients map has %d entries, want 1 after eviction", len(m.clients))
	}
}

// ---------------------------------------------------------------------------
// Handle — routes valid token to Dispatch, 429 when full
// ---------------------------------------------------------------------------

func TestHandleRoutesToDispatch(t *testing.T) {
	m := testManager()
	// Use an unregistered method name so Dispatch returns 404 without
	// touching the network. A 404 proves the request reached Dispatch.
	status, body := m.Handle(context.Background(), &server.Query{
		Token:  "123456:ABC-DEF",
		Method: "nonexistentmethod42",
	})
	if status != 404 {
		t.Fatalf("status = %d, want 404; body=%s", status, body)
	}
	var got struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, body)
	}
	if got.Ok || got.ErrorCode != 404 {
		t.Fatalf("envelope = %+v, want error_code=404", got)
	}
	// A client must have been created.
	if len(m.clients) != 1 {
		t.Fatalf("expected 1 client after Handle, got %d", len(m.clients))
	}
}

func TestHandleRejectsWhenClientMapFull(t *testing.T) {
	m := &Manager{
		params:    &Parameters{MaxClients: 1},
		tq:        tqueue.New(),
		startTime: time.Now(),
		clients:   make(map[string]*client.Client),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}

	// Pre-fill the single allowed slot with a live client.
	m.clients["111:abc"] = client.NewClient(client.Params{
		TQueue:    m.tq,
		StartTime: m.startTime,
	}, "111:abc")

	// A different valid token should be rejected with 429 because the map
	// is full and the existing client is not dead.
	status, body := m.Handle(context.Background(), &server.Query{
		Token:  "222:def",
		Method: "getme",
	})
	if status != 429 {
		t.Fatalf("status = %d, want 429; body=%s", status, body)
	}
	var got struct {
		Ok          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, body)
	}
	if got.Ok || got.ErrorCode != 429 {
		t.Fatalf("envelope = %+v, want error_code=429", got)
	}
}

// ---------------------------------------------------------------------------
// DC isolation — token with /test suffix creates separate client
// ---------------------------------------------------------------------------

func TestClientForTestDCIsolation(t *testing.T) {
	m := testManager()
	normal := m.clientFor("100:token", false)
	testDC := m.clientFor("100:token", true)

	if normal == nil || testDC == nil {
		t.Fatal("expected non-nil clients")
	}
	if normal == testDC {
		t.Fatal("normal and testDC clients should be different instances")
	}
	if !testDC.TestDC {
		t.Fatal("testDC client should have TestDC=true")
	}
	if normal.TestDC {
		t.Fatal("normal client should have TestDC=false")
	}
	// Two separate map entries (token and token/test).
	if len(m.clients) != 2 {
		t.Fatalf("clients map has %d entries, want 2", len(m.clients))
	}
	if _, ok := m.clients["100:token"]; !ok {
		t.Fatal("normal client not stored under bare token key")
	}
	if _, ok := m.clients["100:token/test"]; !ok {
		t.Fatal("testDC client not stored under token/test key")
	}
}

func TestHandleTestDCIsolation(t *testing.T) {
	m := testManager()
	// First request: normal DC.
	m.Handle(context.Background(), &server.Query{
		Token:  "100:token",
		Method: "nonexistent42",
		TestDC: false,
	})
	// Second request: test DC — should create a separate client.
	m.Handle(context.Background(), &server.Query{
		Token:  "100:token",
		Method: "nonexistent42",
		TestDC: true,
	})
	if len(m.clients) != 2 {
		t.Fatalf("expected 2 clients (normal + test), got %d", len(m.clients))
	}
}

// ---------------------------------------------------------------------------
// Close — stops GC loop, closes store
// ---------------------------------------------------------------------------

func TestNewCloseStopsGCAndClosesStore(t *testing.T) {
	dir := t.TempDir()
	m, err := New(&Parameters{WorkingDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	m.StartGC()

	if err := m.Close(); err != nil {
		t.Fatalf("Close returned %v", err)
	}

	// GC loop exited.
	select {
	case <-m.gcDone:
	default:
		t.Fatal("gcDone not closed after Close (GC loop still running)")
	}

	// Store is closed: subsequent LoadAll must fail.
	if _, err := m.tqStore.LoadAll(context.Background()); err == nil {
		t.Fatal("LoadAll should fail after Close (store closed)")
	}
}

func TestShutdownWithNoClients(t *testing.T) {
	dir := t.TempDir()
	m, err := New(&Parameters{WorkingDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown returned %v", err)
	}
	// Store should be closed after Shutdown (Shutdown calls CloseContext).
	if _, err := m.tqStore.LoadAll(context.Background()); err == nil {
		t.Fatal("LoadAll should fail after Shutdown (store closed)")
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func testManager() *Manager {
	return &Manager{
		params:    &Parameters{},
		tq:        tqueue.New(),
		startTime: time.Now(),
		clients:   make(map[string]*client.Client),
		gcStarted: make(chan struct{}),
		gcStop:    make(chan struct{}),
		gcDone:    make(chan struct{}),
	}
}
