package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

// --- convert.InlineQueryResults ---

func TestInlineQueryResults_Article(t *testing.T) {
	raw := `[{"type":"article","id":"1","title":"Test","input_message_content":{"message_text":"hello"}}]`
	results, err := InlineQueryResults(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r, ok := results[0].(*tg.InputBotInlineResult)
	if !ok {
		t.Fatalf("expected *InputBotInlineResult, got %T", results[0])
	}
	if r.ID != "1" || r.Type != "article" || r.Title != "Test" {
		t.Errorf("fields: ID=%s Type=%s Title=%s", r.ID, r.Type, r.Title)
	}
	if r.SendMessage == nil {
		t.Fatal("SendMessage must not be nil (required TL field)")
	}
}

func TestInlineQueryResults_PhotoContent(t *testing.T) {
	// Photo result — media goes in content (flag 5), NOT url (flag 3).
	raw := `[{"type":"photo","id":"2","title":"P","photo_url":"https://example.com/p.jpg","thumbnail_url":"https://example.com/t.jpg"}]`
	results, err := InlineQueryResults(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	r, ok := results[0].(*tg.InputBotInlineResult)
	if !ok {
		t.Fatalf("expected *InputBotInlineResult, got %T", results[0])
	}
	if r.Content == nil {
		t.Fatal("Content must be set for photo results")
	}
	if r.URL != "" {
		t.Error("URL (flag 3) must NOT be set for photo results")
	}
	if r.Thumb == nil {
		t.Error("Thumb must be set when thumbnail_url provided")
	}
}

func TestInlineQueryResults_GifContentTypeField(t *testing.T) {
	// GIF uses gif_url (per-type field), not generic url.
	raw := `[{"type":"gif","id":"3","gif_url":"https://example.com/g.gif","thumbnail_url":"https://example.com/t.jpg"}]`
	results, err := InlineQueryResults(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	r, ok := results[0].(*tg.InputBotInlineResult)
	if !ok {
		t.Fatalf("expected *InputBotInlineResult, got %T", results[0])
	}
	if r.Content == nil {
		t.Fatal("Content must be set from gif_url")
	}
	if r.Content.MimeType != "image/gif" {
		t.Errorf("mime = %s, want image/gif", r.Content.MimeType)
	}
}

func TestInlineQueryResults_NilSendMessageDefaultsToMediaAuto(t *testing.T) {
	// Result without input_message_content → SendMessage defaults to MediaAuto
	// (must not be nil — would panic at encode time).
	raw := `[{"type":"photo","id":"4","photo_url":"https://e.com/p.jpg","thumbnail_url":"https://e.com/t.jpg"}]`
	results, err := InlineQueryResults(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	r := results[0].(*tg.InputBotInlineResult)
	if r.SendMessage == nil {
		t.Fatal("SendMessage must default to MediaAuto (never nil)")
	}
}

func TestInlineQueryResults_Empty(t *testing.T) {
	results, err := InlineQueryResults("[]")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestInlineQueryResults_InvalidJSON(t *testing.T) {
	_, err := InlineQueryResults("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- convert.ReplyMarkup ---

func TestReplyMarkup_InlineKeyboard(t *testing.T) {
	raw := `{"inline_keyboard":[[{"text":"OK","callback_data":"ok"}]]}`
	markup, err := ReplyMarkup(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	ikb, ok := markup.(*tg.ReplyInlineMarkup)
	if !ok {
		t.Fatalf("expected *ReplyInlineMarkup, got %T", markup)
	}
	if len(ikb.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(ikb.Rows))
	}
}

func TestReplyMarkup_Empty(t *testing.T) {
	// Empty string should not crash (may return nil markup).
	markup, _ := ReplyMarkup("")
	if markup != nil {
		t.Error("empty markup should return nil")
	}
}
