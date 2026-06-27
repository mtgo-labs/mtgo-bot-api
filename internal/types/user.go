// Package types defines the Bot API type structs matching the official Bot API
// spec with snake_case json tags. Booleans never use omitempty (Constitution 1).
package types

import "encoding/json"

// User represents a Telegram user or bot. The bot-capability fields
// (CanJoinGroups … SupportsJoinRequestQueries) have no omitempty so getMe emits
// them. For ALL other contexts (message.from, callback.from, etc.) those fields
// must NOT appear — including when the bot is the sender of its own outgoing
// messages. The `full` flag is set ONLY by getMe (via UserForGetMe).
type User struct {
	ID                         int64  `json:"id"`
	IsBot                      bool   `json:"is_bot"`
	FirstName                  string `json:"first_name"`
	LastName                   string `json:"last_name,omitempty"`
	Username                   string `json:"username,omitempty"`
	LanguageCode               string `json:"language_code,omitempty"`
	IsPremium                  bool   `json:"is_premium,omitempty"`
	AddedToAttachmentMenu      bool   `json:"added_to_attachment_menu,omitempty"`
	CanJoinGroups              bool   `json:"can_join_groups"`
	CanReadAllGroupMessages    bool   `json:"can_read_all_group_messages"`
	SupportsInlineQueries      bool   `json:"supports_inline_queries"`
	SupportsGuestQueries       bool   `json:"supports_guest_queries"`
	CanConnectToBusiness       bool   `json:"can_connect_to_business"`
	HasMainWebApp              bool   `json:"has_main_web_app"`
	HasTopicsEnabled           bool   `json:"has_topics_enabled"`
	AllowsUsersToCreateTopics  bool   `json:"allows_users_to_create_topics"`
	CanManageBots              bool   `json:"can_manage_bots"`
	SupportsJoinRequestQueries bool   `json:"supports_join_request_queries"`
	// full is set by UserForGetMe; only getMe emits the full capability shape.
	full bool `json:"-"`
}

// UserForGetMe marks the User for full serialization (all capability fields).
// Used exclusively by getMe. Returns the same pointer for convenience.
func UserForGetMe(u *User) *User {
	if u != nil {
		u.full = true
	}
	return u
}

// MarshalJSON emits the full User for getMe (full=true) and a minimal user-info
// shape for all other contexts (message.from / senders), matching the official
// Bot API: capability fields only appear on the bot's own getMe profile — never
// on a message sender, even when the sender IS the bot (outgoing messages).
func (u User) MarshalJSON() ([]byte, error) {
	type fullUser User // avoid recursion
	if u.full {
		return json.Marshal(fullUser(u))
	}
	return json.Marshal(struct {
		ID                    int64  `json:"id"`
		IsBot                 bool   `json:"is_bot"`
		FirstName             string `json:"first_name"`
		LastName              string `json:"last_name,omitempty"`
		Username              string `json:"username,omitempty"`
		LanguageCode          string `json:"language_code,omitempty"`
		IsPremium             bool   `json:"is_premium,omitempty"`
		AddedToAttachmentMenu bool   `json:"added_to_attachment_menu,omitempty"`
	}{
		ID: u.ID, IsBot: u.IsBot, FirstName: u.FirstName, LastName: u.LastName,
		Username: u.Username, LanguageCode: u.LanguageCode,
		IsPremium: u.IsPremium, AddedToAttachmentMenu: u.AddedToAttachmentMenu,
	})
}

// UserForFrom returns a User suitable for message.from / sender serialization.
// It carries the user-info fields (id, is_bot, first_name, last_name, username,
// language_code, is_premium, added_to_attachment_menu) but STRIPS the
// bot-capability fields (can_join_groups, supports_inline_queries, etc.) which
// the official Bot API only emits on getMe (the bot's own profile), never on a
// message sender. Without this, those bool fields (which have no omitempty so
// they survive in getMe) would pollute every non-bot User JSON with falses.
func UserForFrom(u *User) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:                    u.ID,
		IsBot:                 u.IsBot,
		FirstName:             u.FirstName,
		LastName:              u.LastName,
		Username:              u.Username,
		LanguageCode:          u.LanguageCode,
		IsPremium:             u.IsPremium,
		AddedToAttachmentMenu: u.AddedToAttachmentMenu,
	}
}

// MessageID identifies a message. Returned by send*/copy*/forward* methods.
type MessageID struct {
	MessageID int64 `json:"message_id"`
}

// ChatID identifies a target chat by integer id (for migrate_to_chat_id and similar).
type ChatID struct {
	ID int64 `json:"id"`
}
