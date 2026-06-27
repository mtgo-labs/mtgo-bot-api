package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func newServiceMsg(action tg.MessageActionClass) *tg.MessageService {
	return &tg.MessageService{
		ID:     42,
		Date:   1000,
		FromID: &tg.PeerUser{UserID: 10},
		PeerID: &tg.PeerChat{ChatID: -5},
		Action: action,
	}
}

func usersMap(ids ...int64) map[int64]*tg.User {
	m := make(map[int64]*tg.User, len(ids))
	for i, id := range ids {
		m[id] = &tg.User{ID: id, FirstName: "User", LastName: string(rune('A' + i))}
	}
	return m
}

func TestMessageServiceEnvelope(t *testing.T) {
	svc := newServiceMsg(&tg.MessageActionEmpty{})
	msg := MessageService(svc, usersMap(10))
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
	if msg.MessageID != 42 || msg.Date != 1000 {
		t.Errorf("envelope: MessageID=%d Date=%d", msg.MessageID, msg.Date)
	}
	if msg.Chat.ID != -5 || msg.Chat.Type != apitypes.ChatTypeGroup {
		t.Errorf("Chat = %+v, want ID=-5 type=group", msg.Chat)
	}
	if msg.From == nil || msg.From.ID != 10 || msg.From.FirstName != "User" {
		t.Errorf("From = %+v, want resolved user 10", msg.From)
	}
}

func TestMessageServiceNil(t *testing.T) {
	if msg := MessageService(nil, nil); msg != nil {
		t.Fatalf("expected nil for nil service, got %+v", msg)
	}
}

func TestMessageServiceNewChatMembers(t *testing.T) {
	svc := newServiceMsg(&tg.MessageActionChatAddUser{Users: []int64{10, 20}})
	msg := MessageService(svc, usersMap(10, 20))
	if len(msg.NewChatMembers) != 2 {
		t.Fatalf("NewChatMembers len = %d, want 2", len(msg.NewChatMembers))
	}
	if msg.NewChatMembers[0].ID != 10 || msg.NewChatMembers[0].FirstName != "User" {
		t.Errorf("NewChatMembers[0] = %+v, want resolved user 10", msg.NewChatMembers[0])
	}
}

func TestMessageServiceNewChatMembersFallsBackToIDOnly(t *testing.T) {
	// User 99 is absent from the map → ID-only stub.
	svc := newServiceMsg(&tg.MessageActionChatAddUser{Users: []int64{99}})
	msg := MessageService(svc, usersMap()) // empty map
	if len(msg.NewChatMembers) != 1 || msg.NewChatMembers[0].ID != 99 {
		t.Errorf("NewChatMembers = %+v, want [ID-only 99]", msg.NewChatMembers)
	}
}

func TestMessageServiceJoinedByLink(t *testing.T) {
	svc := newServiceMsg(&tg.MessageActionChatJoinedByLink{InviterID: 10})
	msg := MessageService(svc, usersMap(10))
	if len(msg.NewChatMembers) != 1 || msg.NewChatMembers[0].ID != 10 {
		t.Errorf("NewChatMembers = %+v, want [sender 10]", msg.NewChatMembers)
	}
}

func TestMessageServiceLeftChatMember(t *testing.T) {
	svc := newServiceMsg(&tg.MessageActionChatDeleteUser{UserID: 20})
	msg := MessageService(svc, usersMap(20))
	if msg.LeftChatMember == nil || msg.LeftChatMember.ID != 20 || msg.LeftChatMember.FirstName != "User" {
		t.Errorf("LeftChatMember = %+v, want resolved user 20", msg.LeftChatMember)
	}
}

func TestMessageServiceNewChatTitle(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionChatEditTitle{Title: "New Title"}), nil)
	if msg.NewChatTitle != "New Title" {
		t.Errorf("NewChatTitle = %q, want %q", msg.NewChatTitle, "New Title")
	}
}

func TestMessageServiceDeleteChatPhoto(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionChatDeletePhoto{}), nil)
	if !msg.DeleteChatPhoto {
		t.Error("DeleteChatPhoto = false, want true")
	}
}

func TestMessageServiceNewChatPhoto(t *testing.T) {
	photo := &tg.Photo{ID: 7, AccessHash: 9, Sizes: []tg.PhotoSizeClass{
		&tg.PhotoSize{Type: "x", W: 100, H: 80, Size: 5000},
	}}
	msg := MessageService(newServiceMsg(&tg.MessageActionChatEditPhoto{Photo: photo}), nil)
	if len(msg.NewChatPhoto) != 1 {
		t.Fatalf("NewChatPhoto len = %d, want 1", len(msg.NewChatPhoto))
	}
	if msg.NewChatPhoto[0].Width != 100 || msg.NewChatPhoto[0].Height != 80 {
		t.Errorf("NewChatPhoto[0] = %+v, want W=100 H=80", msg.NewChatPhoto[0])
	}
}

func TestMessageServiceChatCreated(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionChatCreate{Title: "G", Users: []int64{10}}), nil)
	if !msg.GroupChatCreated {
		t.Error("GroupChatCreated = false, want true")
	}
}

func TestMessageServiceChannelCreated(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionChannelCreate{Title: "C"}), nil)
	if !msg.ChannelChatCreated {
		t.Error("ChannelChatCreated = false, want true")
	}
}

func TestMessageServiceAutoDeleteTimer(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionSetMessagesTTL{Period: 3600}), nil)
	if msg.MessageAutoDeleteTimerChanged == nil || msg.MessageAutoDeleteTimerChanged.MessageAutoDeleteTime != 3600 {
		t.Errorf("MessageAutoDeleteTimerChanged = %+v, want 3600", msg.MessageAutoDeleteTimerChanged)
	}
}

func TestMessageServiceVoiceChatStarted(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionGroupCall{}), nil)
	// Both the video_chat_started and voice_chat_started aliases must be set.
	if msg.VoiceChatStarted == nil || msg.VideoChatStarted == nil {
		t.Errorf("VoiceChatStarted=%v VideoChatStarted=%v, want both non-nil", msg.VoiceChatStarted, msg.VideoChatStarted)
	}
}

func TestMessageServiceVoiceChatScheduled(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionGroupCallScheduled{ScheduleDate: 123}), nil)
	if msg.VoiceChatScheduled == nil || msg.VideoChatScheduled == nil || msg.VoiceChatScheduled.StartDate != 123 {
		t.Errorf("VoiceChatScheduled=%v VideoChatScheduled=%v", msg.VoiceChatScheduled, msg.VideoChatScheduled)
	}
}

func TestMessageServiceInviteToGroupCall(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionInviteToGroupCall{Users: []int64{10, 20}}), usersMap(10, 20))
	if msg.VideoChatParticipantsInvited == nil || msg.VoiceChatParticipantsInvited == nil {
		t.Fatalf("participants: video=%v voice=%v", msg.VideoChatParticipantsInvited, msg.VoiceChatParticipantsInvited)
	}
	if len(msg.VideoChatParticipantsInvited.Users) != 2 || msg.VideoChatParticipantsInvited.Users[0].ID != 10 {
		t.Errorf("Users = %+v, want [10, 20] resolved", msg.VideoChatParticipantsInvited.Users)
	}
}

func TestMessageServiceMigrateTo(t *testing.T) {
	// Reference: migrate_to_chat_id = get_supergroup_chat_id(supergroup_id) = -1e13 - id.
	msg := MessageService(newServiceMsg(&tg.MessageActionChatMigrateTo{ChannelID: 456}), nil)
	if msg.MigrateToChatID != -1000000000456 {
		t.Errorf("MigrateToChatID = %d, want -1000000000456", msg.MigrateToChatID)
	}
}

func TestMessageServiceMigrateFrom(t *testing.T) {
	// Reference: migrate_from_chat_id = get_basic_group_chat_id(basic_group_id) = -id.
	msg := MessageService(newServiceMsg(&tg.MessageActionChannelMigrateFrom{ChatID: 123}), nil)
	if msg.MigrateFromChatID != -123 {
		t.Errorf("MigrateFromChatID = %d, want -123", msg.MigrateFromChatID)
	}
}

func TestMessageServicePinnedMessage(t *testing.T) {
	// Pin carries the pinned message id via reply_to; we emit the inaccessible
	// fallback {message_id, date:0, chat} (reference uncached path).
	svc := newServiceMsg(&tg.MessageActionPinMessage{})
	svc.ReplyTo = &tg.MessageReplyHeader{ReplyToMsgID: 99}
	msg := MessageService(svc, nil)
	if msg.PinnedMessage == nil || msg.PinnedMessage.MessageID != 99 || msg.PinnedMessage.Date != 0 {
		t.Errorf("PinnedMessage = %+v, want {MessageID:99 Date:0}", msg.PinnedMessage)
	}
	if msg.PinnedMessage.Chat.ID != -5 {
		t.Errorf("PinnedMessage.Chat.ID = %d, want -5 (same chat as the pin)", msg.PinnedMessage.Chat.ID)
	}
}

func TestMessageServicePinWithoutReplyTo(t *testing.T) {
	msg := MessageService(newServiceMsg(&tg.MessageActionPinMessage{}), nil)
	if msg.PinnedMessage != nil {
		t.Error("PinnedMessage should be nil when the pin has no reply_to")
	}
}
