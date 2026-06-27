package client

import (
	"testing"

	"github.com/mtgo-labs/mtgo/telegram"
	mtgotypes "github.com/mtgo-labs/mtgo/telegram/types"
	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// privateMsg builds a private-chat (PeerUser) update carrying the given raw message.
func privateMsg(raw *tg.Message) *telegram.Update {
	return &telegram.Update{
		Message: &mtgotypes.Message{Raw: raw},
	}
}

// TestBuildUpdate_ReplyResolvedFromCache: a message seen earlier is served as
// reply_to_message when a later message references it.
func TestBuildUpdate_ReplyResolvedFromCache(t *testing.T) {
	cache := newMsgCache(8)
	const peer = int64(10)

	// Message 1 (origin) — ingesting it caches (peer, 1).
	buildUpdateObject(privateMsg(&tg.Message{
		ID: 1, Date: 100, Message: "origin", PeerID: &tg.PeerUser{UserID: peer},
	}), 0, cache)

	// Message 2 replies to 1.
	obj := buildUpdateObject(privateMsg(&tg.Message{
		ID: 2, Date: 101, Message: "reply", PeerID: &tg.PeerUser{UserID: peer},
		ReplyTo: &tg.MessageReplyHeader{ReplyToMsgID: 1},
	}), 0, cache)

	msg, ok := obj["message"].(*apitypes.Message)
	if !ok {
		t.Fatalf("want message, got %v", obj)
	}
	if msg.ReplyToMessage == nil {
		t.Fatal("reply_to_message not populated from cache")
	}
	if msg.ReplyToMessage.MessageID != 1 {
		t.Errorf("reply_to_message.MessageID = %d, want 1", msg.ReplyToMessage.MessageID)
	}
	if msg.ReplyToMessage.Text != "origin" {
		t.Errorf("reply_to_message.Text = %q, want %q", msg.ReplyToMessage.Text, "origin")
	}
}

// TestBuildUpdate_ReplyUnresolvedWhenUncached: a reply to an unseen message
// leaves reply_to_message unset (no false population).
func TestBuildUpdate_ReplyUnresolvedWhenUncached(t *testing.T) {
	cache := newMsgCache(8)
	obj := buildUpdateObject(privateMsg(&tg.Message{
		ID: 2, Date: 101, Message: "reply", PeerID: &tg.PeerUser{UserID: 10},
		ReplyTo: &tg.MessageReplyHeader{ReplyToMsgID: 99},
	}), 0, cache)
	msg := obj["message"].(*apitypes.Message)
	if msg.ReplyToMessage != nil {
		t.Fatalf("reply_to_message should be nil for uncached reply, got %+v", msg.ReplyToMessage)
	}
}

// TestBuildUpdate_PinnedResolvedFromCache: a pin service message referencing a
// seen message replaces the inaccessible fallback with the full cached message.
func TestBuildUpdate_PinnedResolvedFromCache(t *testing.T) {
	cache := newMsgCache(8)
	const peer = int64(10)

	buildUpdateObject(privateMsg(&tg.Message{
		ID: 1, Date: 100, Message: "pinned-content", PeerID: &tg.PeerUser{UserID: peer},
	}), 0, cache)

	pinUpd := &telegram.Update{
		Message: &mtgotypes.Message{Raw: &tg.MessageService{
			ID: 2, Date: 102, PeerID: &tg.PeerUser{UserID: peer},
			Action:  &tg.MessageActionPinMessage{},
			ReplyTo: &tg.MessageReplyHeader{ReplyToMsgID: 1},
		}},
	}
	obj := buildUpdateObject(pinUpd, 0, cache)
	msg, ok := obj["message"].(*apitypes.Message)
	if !ok {
		t.Fatalf("want message (pin service), got %v", obj)
	}
	if msg.PinnedMessage == nil {
		t.Fatal("pinned_message not set")
	}
	if msg.PinnedMessage.Text != "pinned-content" {
		t.Errorf("pinned_message.Text = %q (inaccessible fallback?), want %q",
			msg.PinnedMessage.Text, "pinned-content")
	}
	if msg.PinnedMessage.MessageID != 1 {
		t.Errorf("pinned_message.MessageID = %d, want 1", msg.PinnedMessage.MessageID)
	}
}

// TestBuildUpdate_PinnedStaysInaccessibleWhenUncached: a pin of an unseen
// message keeps the inaccessible fallback (Date == 0).
func TestBuildUpdate_PinnedStaysInaccessibleWhenUncached(t *testing.T) {
	cache := newMsgCache(8)
	pinUpd := &telegram.Update{
		Message: &mtgotypes.Message{Raw: &tg.MessageService{
			ID: 2, Date: 102, PeerID: &tg.PeerUser{UserID: 10},
			Action:  &tg.MessageActionPinMessage{},
			ReplyTo: &tg.MessageReplyHeader{ReplyToMsgID: 99},
		}},
	}
	obj := buildUpdateObject(pinUpd, 0, cache)
	msg := obj["message"].(*apitypes.Message)
	if msg.PinnedMessage == nil {
		t.Fatal("pinned_message fallback should still be present")
	}
	if msg.PinnedMessage.Date != 0 {
		t.Errorf("uncached pin should keep inaccessible fallback (Date 0), got Date %d",
			msg.PinnedMessage.Date)
	}
}
