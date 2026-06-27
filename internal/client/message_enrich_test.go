package client

import (
	"context"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestExtractChats(t *testing.T) {
	result := &tg.Updates{
		Chats: []tg.ChatClass{
			&tg.Channel{ID: 5, Broadcast: true, Title: "News", Username: "news"},
			&tg.Channel{ID: 7, Megagroup: true, Title: "Discuss", Forum: true},
			&tg.Chat{ID: 9, Title: "Basic Group"},
		},
	}
	chats := extractChats(result)
	if chats == nil {
		t.Fatal("nil chats map")
	}
	// Broadcast channel → type channel, full metadata.
	bc, ok := chats[-1000000000005]
	if !ok {
		t.Fatal("broadcast channel missing")
	}
	if bc.Type != apitypes.ChatTypeChannel {
		t.Errorf("broadcast type=%q, want channel", bc.Type)
	}
	if bc.Title != "News" || bc.Username != "news" {
		t.Errorf("broadcast metadata = %+v", bc)
	}
	// Megagroup → type supergroup (NOT channel), forum flag.
	mg := chats[-1000000000007]
	if mg.Type != apitypes.ChatTypeSupergroup {
		t.Errorf("megagroup type=%q, want supergroup", mg.Type)
	}
	if !mg.IsForum {
		t.Error("megagroup IsForum = false, want true")
	}
	// Basic group.
	g := chats[-9]
	if g.Type != apitypes.ChatTypeGroup || g.Title != "Basic Group" {
		t.Errorf("group = %+v", g)
	}
	// Non-Updates results yield nil.
	if got := extractChats(&tg.UpdateShort{}); got != nil {
		t.Errorf("non-Updates result should yield nil, got %v", got)
	}
}

func TestEnrichSentMessage_ChannelPostSenderChat(t *testing.T) {
	// A broadcast-channel post: From is nil (MTProto omits from_id); the
	// channel itself must become sender_chat, and the chat gains type/title.
	out := &apitypes.Message{
		Chat: apitypes.Chat{ID: -1000000000005, Type: apitypes.ChatTypeSupergroup},
	}
	chats := map[int64]apitypes.Chat{
		-1000000000005: {ID: -1000000000005, Type: apitypes.ChatTypeChannel, Title: "News", Username: "news"},
	}
	(&Client{}).enrichSentMessage(context.Background(), out, chats)

	if out.Chat.Type != apitypes.ChatTypeChannel {
		t.Errorf("Chat.Type = %q, want channel", out.Chat.Type)
	}
	if out.Chat.Title != "News" || out.Chat.Username != "news" {
		t.Errorf("Chat metadata missing: %+v", out.Chat)
	}
	if out.SenderChat == nil {
		t.Fatal("SenderChat = nil, want the channel")
	}
	if out.SenderChat.ID != -1000000000005 || out.SenderChat.Type != apitypes.ChatTypeChannel {
		t.Errorf("SenderChat = %+v", out.SenderChat)
	}
	if out.From != nil {
		t.Errorf("From should stay nil for a channel post, got %+v", out.From)
	}
}

func TestEnrichSentMessage_SupergroupKeepsFrom(t *testing.T) {
	// A supergroup post where the bot posted under its own identity keeps From
	// and gains type/title, but must NOT get sender_chat.
	out := &apitypes.Message{
		From: &apitypes.User{ID: 1, FirstName: "Bot"},
		Chat: apitypes.Chat{ID: -1000000000007, Type: apitypes.ChatTypeSupergroup},
	}
	chats := map[int64]apitypes.Chat{
		-1000000000007: {ID: -1000000000007, Type: apitypes.ChatTypeSupergroup, Title: "Discuss"},
	}
	(&Client{}).enrichSentMessage(context.Background(), out, chats)

	if out.Chat.Title != "Discuss" {
		t.Errorf("Title = %q, want Discuss", out.Chat.Title)
	}
	if out.SenderChat != nil {
		t.Errorf("SenderChat should be nil when From is set, got %+v", out.SenderChat)
	}
	if out.From == nil {
		t.Error("From was unexpectedly cleared")
	}
}
