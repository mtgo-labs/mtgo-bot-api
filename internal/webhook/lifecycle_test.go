package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

func TestDeliverer_Lifecycle(t *testing.T) {
	// Set up a test HTTP server that receives webhook POSTs.
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = body
		received.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// Set up TQueue + Store.
	dir := t.TempDir()
	defer os.RemoveAll(dir)
	store, err := storage.Open(dir, "123")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	tq := tqueue.New()
	qid := tqueue.QueueID(1)

	// Push a test update.
	data := []byte(`{"message":{"text":"webhook test"}}`)
	tq.Push(context.Background(), qid, data, int32(time.Now().Unix())+3600, 0, 0)

	// Create deliverer.
	d := NewDeliverer("test-bot", qid, tq, store)
	d.SetSSRFBypass(true) // tests use httptest (loopback)
	if d == nil {
		t.Fatal("NewDeliverer returned nil")
	}
	if d.IsActive() {
		t.Error("should not be active before Start")
	}

	// Start delivery.
	cfg := storage.WebhookConfig{
		URL:         srv.URL,
		SecretToken: "test-secret",
	}
	d.Start(cfg, false)

	// Verify active.
	if !d.IsActive() {
		t.Error("should be active after Start")
	}

	// Wait for delivery (retry loop with backoff).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if received.Load() > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if received.Load() == 0 {
		t.Error("webhook delivery did not arrive within 5s")
	}

	// Stop.
	d.Stop()
	if d.IsActive() {
		t.Error("should not be active after Stop")
	}
}

func TestDeliverer_StopWithoutStart(t *testing.T) {
	// Stop on a non-started deliverer should not panic.
	dir := t.TempDir()
	defer os.RemoveAll(dir)
	store, _ := storage.Open(dir, "123")
	defer store.Close()
	tq := tqueue.New()
	d := NewDeliverer("test-bot", 1, tq, store)
	d.Stop() // should be a no-op, not panic
	if d.IsActive() {
		t.Error("should not be active")
	}
}
