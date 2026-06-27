package types

import (
	"encoding/json"
	"testing"
)

func TestUserForFrom_StripsBotCapabilityFields(t *testing.T) {
	// A full User as it would come from getMe (all bot-capability fields set).
	full := &User{
		ID:                        123,
		IsBot:                     false,
		FirstName:                 "Alice",
		LastName:                  "Smith",
		Username:                  "alice",
		LanguageCode:              "en",
		IsPremium:                 true,
		AddedToAttachmentMenu:     true,
		CanJoinGroups:             true,
		CanReadAllGroupMessages:   true,
		SupportsInlineQueries:     true,
		CanConnectToBusiness:      true,
		HasMainWebApp:             true,
		HasTopicsEnabled:          true,
		AllowsUsersToCreateTopics: true,
		CanManageBots:             true,
		SupportsGuestQueries:      true,
		SupportsJoinRequestQueries: true,
	}

	got := UserForFrom(full)
	if got == nil {
		t.Fatal("UserForFrom returned nil")
	}

	// User-info fields preserved.
	if got.ID != 123 || got.FirstName != "Alice" || got.Username != "alice" || !got.IsPremium {
		t.Errorf("user-info fields not preserved: %+v", got)
	}

	// Bot-capability fields MUST be zero (stripped).
	for name, val := range map[string]bool{
		"can_join_groups":              got.CanJoinGroups,
		"can_read_all_group_messages":  got.CanReadAllGroupMessages,
		"supports_inline_queries":      got.SupportsInlineQueries,
		"can_connect_to_business":      got.CanConnectToBusiness,
		"has_main_web_app":             got.HasMainWebApp,
		"has_topics_enabled":           got.HasTopicsEnabled,
		"allows_users_to_create_topics": got.AllowsUsersToCreateTopics,
		"can_manage_bots":              got.CanManageBots,
		"supports_guest_queries":       got.SupportsGuestQueries,
		"supports_join_request_queries": got.SupportsJoinRequestQueries,
	} {
		if val {
			t.Errorf("bot-capability field %s should be stripped (false), was true", name)
		}
	}

	// JSON shape: must NOT contain any bot-capability keys.
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	jsonStr := string(b)
	for _, key := range []string{
		`"can_join_groups"`, `"can_read_all_group_messages"`, `"supports_inline_queries"`,
		`"can_connect_to_business"`, `"has_main_web_app"`, `"has_topics_enabled"`,
		`"allows_users_to_create_topics"`, `"can_manage_bots"`, `"supports_guest_queries"`,
		`"supports_join_request_queries"`,
	} {
		if contains(jsonStr, key) {
			t.Errorf("JSON must not contain %s; got %s", key, jsonStr)
		}
	}
	// Must contain the user-info keys.
	for _, key := range []string{`"id"`, `"first_name"`, `"username"`, `"is_premium"`} {
		if !contains(jsonStr, key) {
			t.Errorf("JSON must contain %s; got %s", key, jsonStr)
		}
	}
}

func TestUserForFrom_Nil(t *testing.T) {
	if UserForFrom(nil) != nil {
		t.Error("UserForFrom(nil) must return nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOf(s, substr) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
