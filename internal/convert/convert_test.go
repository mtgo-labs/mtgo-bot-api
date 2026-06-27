package convert

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestUser_BasicFields(t *testing.T) {
	u := &tg.User{ID: 123, FirstName: "Alice", LastName: "Smith", Username: "alice"}
	out := User(u)
	if out == nil {
		t.Fatal("nil output")
	}
	if out.ID != 123 || out.FirstName != "Alice" || out.LastName != "Smith" || out.Username != "alice" {
		t.Errorf("field mismatch: %+v", out)
	}
}

func TestUser_BotFlag(t *testing.T) {
	u := &tg.User{ID: 1, Bot: true}
	out := User(u)
	if !out.IsBot {
		t.Error("Bot=true should map to IsBot=true")
	}
}

func TestUser_NonBotMinimalJSON(t *testing.T) {
	u := &tg.User{ID: 1, FirstName: "Bob"}
	out := User(u)
	b, _ := json.Marshal(out)
	s := string(b)
	for _, key := range []string{
		`"can_join_groups"`, `"supports_inline_queries"`, `"can_connect_to_business"`,
		`"has_main_web_app"`, `"allows_users_to_create_topics"`,
	} {
		if strings.Contains(s, key) {
			t.Errorf("non-bot user JSON must not contain %s; got %s", key, s)
		}
	}
	// Must contain basic fields.
	for _, key := range []string{`"id"`, `"is_bot"`, `"first_name"`} {
		if !strings.Contains(s, key) {
			t.Errorf("non-bot user JSON must contain %s; got %s", key, s)
		}
	}
}

func TestUser_GetMeFullJSON(t *testing.T) {
	u := &tg.User{ID: 1, FirstName: "Bot", Bot: true}
	out := User(u)
	out = apitypes.UserForGetMe(out)
	b, _ := json.Marshal(out)
	s := string(b)
	if !strings.Contains(s, `"can_join_groups"`) {
		t.Errorf("getMe user JSON must contain capability fields; got %s", s)
	}
}

func TestPhoto_Sizes(t *testing.T) {
	p := &tg.Photo{
		ID: 1,
		Sizes: []tg.PhotoSizeClass{
			&tg.PhotoSize{Type: "s", W: 100, H: 100},
			&tg.PhotoSize{Type: "m", W: 320, H: 320},
		},
	}
	sizes := Photo(p)
	if len(sizes) != 2 {
		t.Fatalf("expected 2 sizes, got %d", len(sizes))
	}
}

func TestPhoto_Empty(t *testing.T) {
	sizes := Photo(nil)
	if len(sizes) != 0 {
		t.Errorf("nil photo should return empty, got %d", len(sizes))
	}
}

func TestDocument_BasicFields(t *testing.T) {
	d := &tg.Document{ID: 42, MimeType: "application/pdf", Size: 1024, DCID: 2}
	out := Document(d)
	if out == nil {
		t.Fatal("nil output")
	}
	if out.MimeType != "application/pdf" {
		t.Errorf("MimeType = %s, want application/pdf", out.MimeType)
	}
	if out.FileSize != 1024 {
		t.Errorf("FileSize = %d, want 1024", out.FileSize)
	}
	if out.FileID == "" {
		t.Error("FileID should be non-empty")
	}
}

func TestDocument_Nil(t *testing.T) {
	out := Document(nil)
	if out != nil {
		t.Error("nil document should return nil")
	}
}

func TestRichText_PlainInPhoto(t *testing.T) {
	// Verify RichText works inside a block (integration).
	rm := &tg.RichMessage{
		Blocks: []tg.PageBlockClass{
			&tg.PageBlockParagraph{Text: &tg.TextPlain{Text: "hello"}},
		},
	}
	out := RichMessage(rm)
	if out == nil || len(out.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %+v", out)
	}
	b, _ := json.Marshal(out)
	if !strings.Contains(string(b), `"paragraph"`) {
		t.Errorf("expected paragraph type; got %s", b)
	}
	if !strings.Contains(string(b), `"hello"`) {
		t.Errorf("expected text 'hello'; got %s", b)
	}
}
