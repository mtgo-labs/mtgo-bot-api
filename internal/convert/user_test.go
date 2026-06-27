package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

func TestUserNil(t *testing.T) {
	if User(nil) != nil {
		t.Fatal("User(nil) should return nil")
	}
}

func TestUserBasicMapping(t *testing.T) {
	raw := &tg.User{
		ID:        123456789,
		Bot:       true,
		FirstName: "MyBot",
		LastName:  "Surname",
		Username:  "mybot",
		LangCode:  "en",
		Premium:   false,
	}
	u := User(raw)
	if u == nil {
		t.Fatal("User() returned nil")
	}
	if u.ID != 123456789 {
		t.Errorf("ID = %d", u.ID)
	}
	if !u.IsBot {
		t.Error("IsBot should be true")
	}
	if u.FirstName != "MyBot" {
		t.Errorf("FirstName = %q", u.FirstName)
	}
	if u.Username != "mybot" {
		t.Errorf("Username = %q", u.Username)
	}
	// GOTCHA: tg.User.LangCode → types.User.LanguageCode
	if u.LanguageCode != "en" {
		t.Errorf("LanguageCode = %q, want en", u.LanguageCode)
	}
}

func TestUserBotCapabilityFlags(t *testing.T) {
	// A bot that CAN join groups, CAN read all group messages, supports inline
	// queries, connects to business, and has a main web app.
	raw := &tg.User{
		ID:                   1,
		Bot:                  true,
		BotNochats:           false, // !BotNochats → CanJoinGroups = true
		BotChatHistory:       true,  // → CanReadAllGroupMessages = true
		BotInlinePlaceholder: "<get>", // non-empty → SupportsInlineQueries = true
		BotBusiness:          true,  // → CanConnectToBusiness = true
		BotHasMainApp:        true,  // → HasMainWebApp = true
		AttachMenuEnabled:    true,  // → AddedToAttachmentMenu = true
	}
	u := User(raw)
	if !u.CanJoinGroups {
		t.Error("CanJoinGroups should be true when BotNochats is false")
	}
	if !u.CanReadAllGroupMessages {
		t.Error("CanReadAllGroupMessages should be true")
	}
	if !u.SupportsInlineQueries {
		t.Error("SupportsInlineQueries should be true (BotInlinePlaceholder set)")
	}
	if !u.CanConnectToBusiness {
		t.Error("CanConnectToBusiness should be true")
	}
	if !u.HasMainWebApp {
		t.Error("HasMainWebApp should be true")
	}
	if !u.AddedToAttachmentMenu {
		t.Error("AddedToAttachmentMenu should be true")
	}
}

func TestUserBotNochatsDisablesGroups(t *testing.T) {
	raw := &tg.User{ID: 1, Bot: true, BotNochats: true}
	u := User(raw)
	if u.CanJoinGroups {
		t.Error("CanJoinGroups should be false when BotNochats is true")
	}
}
