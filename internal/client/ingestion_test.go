package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
	"github.com/mtgo-labs/mtgo/tg"
)

// TestSavePeerPopulatesCache verifies the peer-cache path used by ingest():
// savePeer stores the access_hash, and ResolvePeer can later find it. This is
// the unblock for sendMessage to a user after they message the bot.
func TestSavePeerPopulatesCache(t *testing.T) {
	c := NewClient(Params{}, "111:secret")
	store, err := storage.Open(t.TempDir(), "111")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	c.store = store

	// Simulate the bot receiving an update from user 42 with access_hash 999.
	c.savePeer(storage.Peer{ID: 42, AccessHash: 999, Type: storage.PeerTypeUser, Username: "alice"})

	got, err := store.GetPeer(context.Background(), 42)
	if err != nil {
		t.Fatalf("peer not cached: %v", err)
	}
	if got.AccessHash != 999 || got.Username != "alice" {
		t.Errorf("cached peer = %+v, want access_hash=999 username=alice", got)
	}
}

// TestPushUpdateEnqueuesAndIDStored verifies the full ingest→getUpdates path
// end-to-end at the queue level: push an update object, read it back, and
// confirm update_id is stored from the EventID.
func TestPushUpdateEnqueuesAndIDStored(t *testing.T) {
	tq := tqueue.New()
	c := NewClient(Params{TQueue: tq}, "222:secret")

	c.pushUpdateObj(map[string]any{"message": map[string]any{"message_id": 7}})

	now := int32(1 << 30)
	events, err := tq.Get(context.Background(), c.queueID(), 0, false, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].UpdateType != "message" {
		t.Fatalf("UpdateType = %q, want message", events[0].UpdateType)
	}
	raw := eventsToRaw(events)
	var obj map[string]any
	if err := json.Unmarshal(raw[0], &obj); err != nil {
		t.Fatal(err)
	}
	if obj["update_id"].(float64) != float64(events[0].ID) {
		t.Errorf("update_id = %v, want %d", obj["update_id"], events[0].ID)
	}
}

func TestEnrichMessageFillsFromForPrivateChatFallback(t *testing.T) {
	msg := &apitypes.Message{
		MessageID: 1,
		Chat:      apitypes.Chat{ID: 1845033319, Type: apitypes.ChatTypePrivate},
		Text:      "k",
	}
	userMap := map[int64]*tg.User{
		1845033319: {
			ID:        1845033319,
			FirstName: "Sadiq",
			Username:  "sadiq",
		},
	}

	enrichMessage(msg, userMap, nil)

	if msg.From == nil {
		t.Fatal("From is nil")
	}
	if msg.From.ID != 1845033319 || msg.From.FirstName != "Sadiq" || msg.From.Username != "sadiq" {
		t.Fatalf("From = %+v", msg.From)
	}
}
