package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// assertOrder verifies that the JSON output contains the given keys (quoted,
// e.g. `"vcard"`) in the specified order. Each key is expected to appear once.
func assertOrder(t *testing.T, got string, keys ...string) {
	t.Helper()
	prev := -1
	for i, k := range keys {
		idx := strings.Index(got, k)
		if idx < 0 {
			t.Errorf("[%d] key %q missing from JSON: %s", i, k, got)
			return
		}
		if idx < prev {
			t.Errorf("key %q (idx %d) appears before previous key (idx %d); JSON: %s", k, idx, prev, got)
			return
		}
		prev = idx
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// Regression: Contact field order is phone_number, first_name, last_name, vcard,
// user_id (Client.cpp JsonContact::store:2550). vcard must precede user_id.
func TestContactFieldOrder(t *testing.T) {
	got := mustMarshal(t, Contact{PhoneNumber: "+1", FirstName: "A", VCard: "v", UserID: 7})
	assertOrder(t, got, `"phone_number"`, `"first_name"`, `"vcard"`, `"user_id"`)
}

// Regression: Game order is title, text, text_entities, description, photo,
// animation (Client.cpp JsonGame::store:2588).
func TestGameFieldOrder(t *testing.T) {
	got := mustMarshal(t, Game{Title: "T", Text: "x", TextEntities: []MessageEntity{{Type: "bold"}}, Description: "D", Photo: []PhotoSize{{FileID: "f", FileUniqueID: "u", Width: 1, Height: 1}}, Animation: &Animation{FileID: "a", FileUniqueID: "au"}})
	assertOrder(t, got, `"title"`, `"text"`, `"text_entities"`, `"description"`, `"photo"`, `"animation"`)
}

// Regression: Poll order — open_period/close_date follow total_voter_count,
// type follows country_codes (Client.cpp JsonPoll::store:2729).
func TestPollFieldOrder(t *testing.T) {
	got := mustMarshal(t, Poll{
		ID: "1", Question: "q", QuestionEntities: []MessageEntity{{Type: "bold"}},
		Options: []PollOption{{PersistentID: "o", Text: "a", VoterCount: 3}}, TotalVoterCount: 4,
		OpenPeriod: 60, CloseDate: 99, CountryCodes: []string{"US"}, Type: "quiz",
		CorrectOptionID: 0, CorrectOptionIDs: []int{0}, Explanation: "e",
	})
	assertOrder(t, got, `"id"`, `"question"`, `"options"`, `"total_voter_count"`,
		`"open_period"`, `"close_date"`, `"is_closed"`, `"country_codes"`, `"type":"quiz"`, `"explanation"`)
}

// Regression: plain Location emits only lat/long/horizontal_accuracy, never the
// live fields (Client.cpp JsonLocation::store:1132).
func TestLocationPlain(t *testing.T) {
	got := mustMarshal(t, Location{Latitude: 1, Longitude: 2, HorizontalAccuracy: 3})
	assertOrder(t, got, `"latitude"`, `"longitude"`, `"horizontal_accuracy"`)
	if strings.Contains(got, "live_period") {
		t.Errorf("plain location must not emit live_period: %s", got)
	}
}

// Regression: live Location emits lat, long, live_period, heading,
// proximity_alert_radius, horizontal_accuracy (Client.cpp JsonLiveLocation:1105).
func TestLocationLive(t *testing.T) {
	got := mustMarshal(t, Location{Latitude: 1, Longitude: 2, LivePeriod: 60, Heading: 5, ProximityAlertRadius: 10, HorizontalAccuracy: 3})
	assertOrder(t, got, `"latitude"`, `"longitude"`, `"live_period"`, `"heading"`, `"proximity_alert_radius"`, `"horizontal_accuracy"`)
}

// Regression: UsersShared order user_ids, users, request_id (Client.cpp:3832);
// legacy UserShared emits user_id (singular), request_id (Client.cpp:3793).
func TestUsersSharedAndLegacyUserShared(t *testing.T) {
	got := mustMarshal(t, UsersShared{UserIDs: []int64{1, 2}, Users: []SharedUser{{UserID: 1}}, RequestID: 9})
	assertOrder(t, got, `"user_ids"`, `"users"`, `"request_id"`)

	legacy := mustMarshal(t, UserShared{UserID: 5, RequestID: 9})
	assertOrder(t, legacy, `"user_id"`, `"request_id"`)
	if strings.Contains(legacy, "user_ids") {
		t.Errorf("legacy UserShared must emit user_id not user_ids: %s", legacy)
	}
}

// Regression: ChatShared order chat_id, title, username, photo, request_id.
func TestChatSharedFieldOrder(t *testing.T) {
	got := mustMarshal(t, ChatShared{ChatID: 1, Title: "T", Username: "u", RequestID: 3})
	assertOrder(t, got, `"chat_id"`, `"title"`, `"username"`, `"request_id"`)
}

// Regression: InlineQuery order id, from, location, chat_type, query, offset
// (Client.cpp JsonInlineQuery::store:5369).
func TestInlineQueryFieldOrder(t *testing.T) {
	got := mustMarshal(t, InlineQuery{ID: "x", From: &User{}, Location: &Location{Latitude: 1, Longitude: 2}, ChatType: "sender", Query: "q", Offset: "0"})
	assertOrder(t, got, `"id"`, `"from"`, `"location"`, `"chat_type"`, `"query"`, `"offset"`)
}

// Regression: ChosenInlineResult order from, location, inline_message_id, query,
// result_id (Client.cpp JsonChosenInlineResult::store:5431).
func TestChosenInlineResultFieldOrder(t *testing.T) {
	got := mustMarshal(t, ChosenInlineResult{From: &User{}, Location: &Location{Latitude: 1, Longitude: 2}, InlineMessageID: "im", Query: "q", ResultID: "r"})
	assertOrder(t, got, `"from"`, `"location"`, `"inline_message_id"`, `"query"`, `"result_id"`)
}

// Regression: ChatMember emits user before status (Client.cpp JsonChatMember:5712).
func TestChatMemberUserBeforeStatus(t *testing.T) {
	got := mustMarshal(t, ChatMember{User: &User{ID: 1}, Status: "member"})
	assertOrder(t, got, `"user"`, `"status"`)
}

// Regression: MessageEntity core order is offset, length, type.
func TestMessageEntityOrder(t *testing.T) {
	got := mustMarshal(t, MessageEntity{Offset: 1, Length: 2, Type: "bold"})
	assertOrder(t, got, `"offset"`, `"length"`, `"type"`)
}

// Regression: PaidMedia is a per-variant union (Client.cpp JsonPaidMedia:2450).
func TestPaidMediaVariants(t *testing.T) {
	preview := mustMarshal(t, PaidMedia{Type: "preview", Width: 1, Height: 2, Duration: 3})
	assertOrder(t, preview, `"type"`, `"width"`, `"height"`, `"duration"`)
	if strings.Contains(preview, "video") {
		t.Errorf("preview must not emit video: %s", preview)
	}

	video := mustMarshal(t, PaidMedia{Type: "video", Video: &Video{FileID: "v", FileUniqueID: "u", Duration: 1, Width: 1, Height: 1}})
	assertOrder(t, video, `"type"`, `"video"`)
}

// Regression: BackgroundType is a per-variant union (Client.cpp JsonBackgroundType:3165).
func TestBackgroundTypeVariants(t *testing.T) {
	wp := mustMarshal(t, BackgroundType{Type: "wallpaper", Document: &Document{FileID: "d", FileUniqueID: "u"}, DarkThemeDimming: 50, IsBlurred: true})
	assertOrder(t, wp, `"type"`, `"document"`, `"dark_theme_dimming"`, `"is_blurred"`)

	pat := mustMarshal(t, BackgroundType{Type: "pattern", Document: &Document{FileID: "d", FileUniqueID: "u"}, Fill: &BackgroundFill{Type: "solid", Color: 1}, Intensity: 40, IsInverted: true})
	assertOrder(t, pat, `"type"`, `"document"`, `"fill"`, `"intensity"`, `"is_inverted"`)

	fill := mustMarshal(t, BackgroundType{Type: "fill", Fill: &BackgroundFill{Type: "gradient", TopColor: 1, BottomColor: 2, RotationAngle: 30}, DarkThemeDimming: 60})
	assertOrder(t, fill, `"type"`, `"fill"`, `"dark_theme_dimming"`)

	theme := mustMarshal(t, BackgroundType{Type: "chat_theme", ThemeName: "t"})
	assertOrder(t, theme, `"type"`, `"theme_name"`)
}

// Regression: all_members_are_administrators is a *bool so basic groups emit it
// unconditionally (including false) while other chat types omit it (Client.cpp:1619).
func TestAllMembersAreAdministratorsPointerBool(t *testing.T) {
	if j := mustMarshal(t, ChatFullInfo{Chat: Chat{ID: 1, Type: ChatTypePrivate}}); strings.Contains(j, "all_members_are_administrators") {
		t.Errorf("non-group must omit all_members_are_administrators: %s", j)
	}
	allAdmins := false
	j := mustMarshal(t, ChatFullInfo{Chat: Chat{ID: 1, Type: ChatTypeGroup}, AllMembersAreAdministrators: &allAdmins})
	if !strings.Contains(j, `"all_members_are_administrators":false`) {
		t.Errorf("group must emit all_members_are_administrators:false: %s", j)
	}
}
