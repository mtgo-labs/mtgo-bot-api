package convert

import (
	"encoding/json"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

func TestDefaultInlineMime_AllTypes(t *testing.T) {
	tests := []struct {
		docType string
		want    string
	}{
		{"gif", "image/gif"},
		{"mpeg4_gif", "video/mp4"},
		{"video", "video/mp4"},
		{"audio", "audio/mpeg"},
		{"voice", "audio/ogg"},
		{"document", "application/octet-stream"},
		{"sticker", "application/octet-stream"},
		{"unknown", "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.docType, func(t *testing.T) {
			if got := defaultInlineMime(tt.docType); got != tt.want {
				t.Errorf("defaultInlineMime(%q) = %s, want %s", tt.docType, got, tt.want)
			}
		})
	}
}

func TestChat_FromPeerChat(t *testing.T) {
	out := Chat(&tg.PeerChat{ChatID: 999})
	if out == nil {
		t.Fatal("nil output")
	}
	if out.ID != 999 {
		t.Errorf("ID = %d, want 999", out.ID)
	}
}

func TestChat_FromPeerChannel(t *testing.T) {
	out := Chat(&tg.PeerChannel{ChannelID: 777})
	if out == nil {
		t.Fatal("nil output")
	}
	// Channel ID is negative in Bot API (-1000000000000 - channelID).
	if out.ID >= 0 {
		t.Errorf("channel chat ID should be negative, got %d", out.ID)
	}
}

func TestChat_Nil(t *testing.T) {
	out := Chat(nil)
	if out != nil {
		t.Error("nil peer should return nil")
	}
}

func TestConvertInputMessageContent_HTML(t *testing.T) {
	raw := json.RawMessage(`{"message_text":"<b>bold</b>","parse_mode":"HTML"}`)
	msg, err := convertInputMessageContent(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if msg == nil {
		t.Fatal("nil message")
	}
	// Should produce an InputBotInlineMessageText with HTML entities.
	tm, ok := msg.(*tg.InputBotInlineMessageText)
	if !ok {
		t.Fatalf("expected *InputBotInlineMessageText, got %T", msg)
	}
	if tm.Message != "<b>bold</b>" {
		t.Errorf("Message = %s", tm.Message)
	}
}

func TestConvertInputMessageContent_MarkdownV2(t *testing.T) {
	raw := json.RawMessage(`{"message_text":"*bold*","parse_mode":"MarkdownV2"}`)
	msg, err := convertInputMessageContent(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if msg == nil {
		t.Fatal("nil message")
	}
}

func TestConvertInputMessageContent_Empty(t *testing.T) {
	msg, err := convertInputMessageContent(nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if msg != nil {
		t.Error("empty content should return nil msg")
	}
}

func TestConvertInputMessageContent_PlainText(t *testing.T) {
	raw := json.RawMessage(`{"message_text":"plain"}`)
	msg, err := convertInputMessageContent(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if msg == nil {
		t.Fatal("nil message")
	}
}

func TestEnsureInlineMessage_WithMessage(t *testing.T) {
	// When msg is non-nil, ensureInlineMessage should return it unchanged.
	original := &tg.InputBotInlineMessageMediaAuto{Message: "caption"}
	result := ensureInlineMessage(original, "fallback")
	if result != original {
		t.Error("non-nil msg should be returned as-is")
	}
}

func TestEnsureInlineMessage_NilDefaultsToMediaAuto(t *testing.T) {
	result := ensureInlineMessage(nil, "caption text")
	ma, ok := result.(*tg.InputBotInlineMessageMediaAuto)
	if !ok {
		t.Fatalf("expected *InputBotInlineMessageMediaAuto, got %T", result)
	}
	if ma.Message != "caption text" {
		t.Errorf("Message = %s, want 'caption text'", ma.Message)
	}
}

func TestEnsureInlineMessage_EmptyCaption(t *testing.T) {
	result := ensureInlineMessage(nil, "")
	ma, ok := result.(*tg.InputBotInlineMessageMediaAuto)
	if !ok {
		t.Fatalf("expected *InputBotInlineMessageMediaAuto, got %T", result)
	}
	if ma.Message != "" {
		t.Errorf("Message = %s, want empty", ma.Message)
	}
}

func TestJsonFieldString(t *testing.T) {
	// Present field.
	raw := json.RawMessage(`{"gif_url":"https://example.com/g.gif"}`)
	if got := jsonFieldString(raw, "gif_url"); got != "https://example.com/g.gif" {
		t.Errorf("jsonFieldString = %s", got)
	}
	// Absent field.
	if got := jsonFieldString(raw, "missing"); got != "" {
		t.Errorf("absent field should return empty, got %s", got)
	}
	// Invalid JSON.
	if got := jsonFieldString(json.RawMessage(`invalid`), "x"); got != "" {
		t.Errorf("invalid JSON should return empty")
	}
}
