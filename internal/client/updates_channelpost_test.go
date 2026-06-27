package client

import (
	"testing"

	"github.com/mtgo-labs/mtgo/telegram"
	mtgotypes "github.com/mtgo-labs/mtgo/telegram/types"
	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// channelMsg builds an update carrying a channel message whose chat is the
// given channel.
func channelMsg(raw *tg.Channel, edited bool) *telegram.Update {
	msg := &mtgotypes.Message{Raw: &tg.Message{
		ID: 1, Date: 100, Message: "hi", PeerID: &tg.PeerChannel{ChannelID: raw.ID},
	}}
	upd := &telegram.Update{
		Chats: map[int64]*mtgotypes.Chat{raw.ID: {Raw: raw}},
	}
	if edited {
		upd.EditedMessage = msg
	} else {
		upd.Message = msg
	}
	return upd
}

func TestBuildUpdate_ChannelPost(t *testing.T) {
	// Broadcast channel → "channel_post", chat type "channel".
	upd := channelMsg(&tg.Channel{ID: 5, Broadcast: true}, false)
	obj := buildUpdateObject(upd, 0, nil)
	msg, ok := obj["channel_post"].(*apitypes.Message)
	if !ok {
		t.Fatalf("want channel_post *Message, got keys: %v", obj)
	}
	if msg.Chat.Type != apitypes.ChatTypeChannel {
		t.Errorf("Chat.Type = %q, want channel", msg.Chat.Type)
	}
}

func TestBuildUpdate_SupergroupIsMessage(t *testing.T) {
	// Supergroup (Broadcast=false) → "message", NOT "channel_post".
	upd := channelMsg(&tg.Channel{ID: 5}, false) // Broadcast unset → false
	obj := buildUpdateObject(upd, 0, nil)
	if _, ok := obj["message"]; !ok {
		t.Fatalf("want message key for supergroup, got %v", obj)
	}
	if _, ok := obj["channel_post"]; ok {
		t.Error("supergroup must not be delivered as channel_post")
	}
}

func TestBuildUpdate_EditedChannelPost(t *testing.T) {
	upd := channelMsg(&tg.Channel{ID: 5, Broadcast: true}, true)
	obj := buildUpdateObject(upd, 0, nil)
	if _, ok := obj["edited_channel_post"]; !ok {
		t.Fatalf("want edited_channel_post key, got %v", obj)
	}
}

func TestBuildUpdate_ChannelUnknownDefaultsToMessage(t *testing.T) {
	// Channel not present in upd.Chats (can't determine broadcast-ness) →
	// graceful fallback to "message".
	upd := &telegram.Update{
		Message: &mtgotypes.Message{Raw: &tg.Message{
			ID: 1, Date: 100, Message: "hi", PeerID: &tg.PeerChannel{ChannelID: 99},
		}},
		Chats: map[int64]*mtgotypes.Chat{}, // empty: channel 99 absent
	}
	obj := buildUpdateObject(upd, 0, nil)
	if _, ok := obj["message"]; !ok {
		t.Fatalf("absent channel should fall back to message, got %v", obj)
	}
}

func TestBuildUpdate_ChannelPostChatEnriched(t *testing.T) {
	// A channel post's chat must carry title/username/is_forum from the update's
	// Chats data — peerToChat only sets id+type. Mirrors api.telegram.org.
	upd := channelMsg(&tg.Channel{
		ID: 5, Broadcast: true, Title: "My Channel", Username: "mychannel",
	}, false)
	obj := buildUpdateObject(upd, 0, nil)
	msg, ok := obj["channel_post"].(*apitypes.Message)
	if !ok {
		t.Fatalf("want channel_post, got keys %v", obj)
	}
	if msg.Chat.ID != -1000000000005 {
		t.Errorf("Chat.ID = %d, want -1000000000005", msg.Chat.ID)
	}
	if msg.Chat.Title != "My Channel" {
		t.Errorf("Chat.Title = %q, want %q", msg.Chat.Title, "My Channel")
	}
	if msg.Chat.Username != "mychannel" {
		t.Errorf("Chat.Username = %q, want %q", msg.Chat.Username, "mychannel")
	}
}

func TestBuildUpdate_ForumSupergroupChatEnriched(t *testing.T) {
	// A forum supergroup message keeps type "supergroup" + is_forum + title.
	upd := channelMsg(&tg.Channel{ID: 7, Title: "Forum SG", Forum: true}, false)
	obj := buildUpdateObject(upd, 0, nil)
	msg, ok := obj["message"].(*apitypes.Message)
	if !ok {
		t.Fatalf("forum supergroup should be 'message', got keys %v", obj)
	}
	if msg.Chat.Title != "Forum SG" {
		t.Errorf("Chat.Title = %q, want %q", msg.Chat.Title, "Forum SG")
	}
	if !msg.Chat.IsForum {
		t.Error("Chat.IsForum = false, want true")
	}
}
