package convert

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

// jsonKeys returns the top-level JSON object key order for v.
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		t.Fatalf("expected object, got %v", tok)
	}
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatalf("token: %v", err)
		}
		keys = append(keys, k.(string))
		if err := skipValue(dec); err != nil {
			t.Fatalf("skip: %v", err)
		}
	}
	return keys
}

// skipValue consumes one whole JSON value (primitive, object, or array) from dec.
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := tok.(json.Delim); ok && (delim == '{' || delim == '[') {
		for dec.More() {
			if err := skipValue(dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil { // closing delimiter
			return err
		}
	}
	return nil
}

func TestConvertFwdFrom_UserVariantOrder(t *testing.T) {
	// Reference JsonMessageOrigin "user": {type, sender_user, date} — date last.
	o := convertFwdFrom(&tg.MessageFwdHeader{
		Date:   100,
		FromID: &tg.PeerUser{UserID: 7},
	})
	if o.Type != "user" || o.SenderUser == nil || o.SenderUser.ID != 7 {
		t.Fatalf("user variant wrong: %+v", o)
	}
	keys := jsonKeys(t, o)
	want := []string{"type", "sender_user", "date"}
	if !equalSlice(keys, want) {
		t.Errorf("key order = %v, want %v", keys, want)
	}
}

func TestConvertFwdFrom_ChannelVariantUsesChat(t *testing.T) {
	// "channel" emits "chat" (Bot API id form), NOT sender_chat.
	o := convertFwdFrom(&tg.MessageFwdHeader{
		Date:        100,
		FromID:      &tg.PeerChannel{ChannelID: 5},
		ChannelPost: 99,
	})
	if o.Type != "channel" {
		t.Fatalf("type = %q", o.Type)
	}
	if o.Chat == nil || o.Chat.ID != -1000000000005 || o.Chat.Type != "channel" {
		t.Errorf("channel chat field = %+v, want id=-1000000000005 type=channel", o.Chat)
	}
	if o.SenderChat != nil {
		t.Errorf("channel variant must not set sender_chat, got %+v", o.SenderChat)
	}
	if o.MessageID != 99 {
		t.Errorf("message_id = %d, want 99", o.MessageID)
	}
}

func TestConvertFwdFrom_ChatVariantID(t *testing.T) {
	// "chat" emits sender_chat in basic-group id form (-ChatID).
	o := convertFwdFrom(&tg.MessageFwdHeader{
		Date:   100,
		FromID: &tg.PeerChat{ChatID: 42},
	})
	if o.Type != "chat" || o.SenderChat == nil || o.SenderChat.ID != -42 {
		t.Errorf("chat variant = %+v, want sender_chat id=-42", o)
	}
}

func TestConvertFwdFrom_HiddenUser(t *testing.T) {
	o := convertFwdFrom(&tg.MessageFwdHeader{
		Date:     100,
		FromName: "anon",
	})
	if o.Type != "hidden_user" || o.SenderUserName != "anon" {
		t.Errorf("hidden_user variant = %+v", o)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
