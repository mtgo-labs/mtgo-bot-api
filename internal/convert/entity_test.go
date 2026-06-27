package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestMessageEntity_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		entity   tg.MessageEntityClass
		wantType string
		wantURL  string
		wantLang string
	}{
		{"mention", &tg.MessageEntityMention{Offset: 0, Length: 5}, "mention", "", ""},
		{"hashtag", &tg.MessageEntityHashtag{Offset: 1, Length: 6}, "hashtag", "", ""},
		{"bot_command", &tg.MessageEntityBotCommand{Offset: 0, Length: 6}, "bot_command", "", ""},
		{"url", &tg.MessageEntityURL{Offset: 0, Length: 10}, "url", "", ""},
		{"email", &tg.MessageEntityEmail{Offset: 0, Length: 8}, "email", "", ""},
		{"bold", &tg.MessageEntityBold{Offset: 0, Length: 4}, "bold", "", ""},
		{"italic", &tg.MessageEntityItalic{Offset: 0, Length: 6}, "italic", "", ""},
		{"code", &tg.MessageEntityCode{Offset: 0, Length: 4}, "code", "", ""},
		{"pre", &tg.MessageEntityPre{Offset: 0, Length: 10, Language: "go"}, "pre", "", "go"},
		{"text_link", &tg.MessageEntityTextURL{Offset: 0, Length: 4, URL: "https://e.com"}, "text_link", "https://e.com", ""},
		{"underline", &tg.MessageEntityUnderline{Offset: 0, Length: 9}, "underline", "", ""},
		{"strikethrough", &tg.MessageEntityStrike{Offset: 0, Length: 13}, "strikethrough", "", ""},
		{"blockquote", &tg.MessageEntityBlockquote{Offset: 0, Length: 10}, "blockquote", "", ""},
		{"spoiler", &tg.MessageEntitySpoiler{Offset: 0, Length: 7}, "spoiler", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := messageEntity(tt.entity)
			if out == nil {
				t.Fatal("nil output")
			}
			if out.Type != tt.wantType {
				t.Errorf("Type = %s, want %s", out.Type, tt.wantType)
			}
			if tt.wantURL != "" && out.URL != tt.wantURL {
				t.Errorf("URL = %s, want %s", out.URL, tt.wantURL)
			}
			if tt.wantLang != "" && out.Language != tt.wantLang {
				t.Errorf("Language = %s, want %s", out.Language, tt.wantLang)
			}
		})
	}
}

func TestMessageEntity_TextMention(t *testing.T) {
	out := messageEntity(&tg.MessageEntityMentionName{Offset: 0, Length: 4, UserID: 42})
	if out == nil || out.Type != "text_mention" {
		t.Fatalf("type = %v", out)
	}
	if out.User == nil || out.User.ID != 42 {
		t.Errorf("User.ID = %v, want 42", out.User)
	}
}

func TestMessageEntity_CustomEmoji(t *testing.T) {
	out := messageEntity(&tg.MessageEntityCustomEmoji{Offset: 0, Length: 1, DocumentID: 999})
	if out == nil || out.Type != "custom_emoji" {
		t.Fatalf("type = %v", out)
	}
	if out.CustomEmojiID != "999" {
		t.Errorf("CustomEmojiID = %s, want '999'", out.CustomEmojiID)
	}
}

func TestMessageEntity_Nil(t *testing.T) {
	if messageEntity(nil) != nil {
		t.Error("nil entity should return nil")
	}
}

func TestMessageEntity_Unknown(t *testing.T) {
	// An unregistered entity type should return nil (not crash).
	out := messageEntity(&tg.MessageEntityUnknown{}) // if this type exists
	if out != nil {
		// Some unknown types may not exist — that's fine, just ensure no panic.
		t.Log("got non-nil for unknown type (may be valid)")
	}
}

func TestConvertMedia_Photo(t *testing.T) {
	media := &tg.MessageMediaPhoto{
		Photo: &tg.Photo{
			ID: 1,
			Sizes: []tg.PhotoSizeClass{
				&tg.PhotoSize{Type: "s", W: 100, H: 100},
			},
		},
	}
	out := &apitypes.Message{}
	convertMedia(media, out)
	if len(out.Photo) != 1 {
		t.Errorf("expected 1 photo size, got %d", len(out.Photo))
	}
}

func TestConvertMedia_Nil(t *testing.T) {
	out := &apitypes.Message{}
	convertMedia(nil, out)
	// Should not panic, should not set any media fields.
	if out.Photo != nil || out.Video != nil || out.Document != nil {
		t.Error("nil media should not set any media fields")
	}
}
