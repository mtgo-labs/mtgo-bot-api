package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// --- convert.Message ---

func TestMessage_Nil(t *testing.T) {
	if Message(nil) != nil {
		t.Error("nil input should return nil")
	}
}

func TestMessage_BasicText(t *testing.T) {
	m := &tg.Message{
		ID:      42,
		Date:    1700000000,
		Message: "hello world",
		PeerID:  &tg.PeerUser{UserID: 123},
		FromID:  &tg.PeerUser{UserID: 456},
	}
	out := Message(m)
	if out == nil {
		t.Fatal("nil output")
	}
	if out.MessageID != 42 {
		t.Errorf("MessageID = %d, want 42", out.MessageID)
	}
	if out.Date != 1700000000 {
		t.Errorf("Date = %d, want 1700000000", out.Date)
	}
	if out.Text != "hello world" {
		t.Errorf("Text = %q, want 'hello world'", out.Text)
	}
	if out.Chat.ID != 123 {
		t.Errorf("Chat.ID = %d, want 123", out.Chat.ID)
	}
	if out.From == nil || out.From.ID != 456 {
		t.Errorf("From.ID = %v, want 456", out.From)
	}
}

func TestMessage_Entities(t *testing.T) {
	m := &tg.Message{
		ID:      1,
		Message: "**bold**",
		PeerID:  &tg.PeerUser{UserID: 1},
		Entities: []tg.MessageEntityClass{
			&tg.MessageEntityBold{Offset: 0, Length: 8},
		},
	}
	out := Message(m)
	if len(out.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(out.Entities))
	}
	if out.Entities[0].Type != "bold" {
		t.Errorf("entity type = %s, want bold", out.Entities[0].Type)
	}
}

func TestMessage_FromChat(t *testing.T) {
	m := &tg.Message{
		ID:     1,
		PeerID: &tg.PeerUser{UserID: 1},
		FromID: &tg.PeerChat{ChatID: 999},
	}
	out := Message(m)
	if out.SenderChat == nil || out.SenderChat.ID != 999 {
		t.Errorf("SenderChat = %v, want ID 999", out.SenderChat)
	}
	if out.From != nil {
		t.Error("From should be nil when FromID is a chat")
	}
}

func TestMessage_FromChannel(t *testing.T) {
	m := &tg.Message{
		ID:     1,
		PeerID: &tg.PeerUser{UserID: 1},
		FromID: &tg.PeerChannel{ChannelID: 777},
	}
	out := Message(m)
	if out.SenderChat == nil || out.SenderChat.ID != 777 {
		t.Errorf("SenderChat = %v, want ID 777", out.SenderChat)
	}
}

// --- convert.Chat ---

func TestChat_FromPeerUser(t *testing.T) {
	out := Chat(&tg.PeerUser{UserID: 100})
	if out == nil {
		t.Fatal("nil output")
	}
	if out.ID != 100 {
		t.Errorf("ID = %d, want 100", out.ID)
	}
}

// --- convert.DocumentMedia (animation classification) ---

func TestDocumentMedia_AnimationClassification(t *testing.T) {
	// A document with BOTH DocumentAttributeAnimated and DocumentAttributeVideo
	// should classify as an animation (not video) due to priority ordering.
	d := &tg.Document{
		ID:       1,
		MimeType: "video/mp4",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{Duration: 5, W: 400, H: 400},
			&tg.DocumentAttributeAnimated{},
		},
	}
	out := &apitypes.Message{}
	DocumentMedia(d, out)

	// Should set Animation (and Document), NOT Video.
	if out.Animation == nil {
		t.Error("Animation should be set for animated documents")
	}
	if out.Video != nil {
		t.Error("Video should NOT be set when the document is animated")
	}
	if out.Document == nil {
		t.Error("Document should also be set for animations (official emits both)")
	}
}

func TestDocumentMedia_PlainVideo(t *testing.T) {
	// A document with ONLY DocumentAttributeVideo (no Animated) → Video.
	d := &tg.Document{
		ID:       2,
		MimeType: "video/mp4",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{Duration: 10, W: 1280, H: 720},
		},
	}
	out := &apitypes.Message{}
	DocumentMedia(d, out)

	if out.Video == nil {
		t.Error("Video should be set for plain video documents")
	}
	if out.Animation != nil {
		t.Error("Animation should NOT be set for plain videos")
	}
}

func TestDocumentMedia_AudioClassification(t *testing.T) {
	d := &tg.Document{
		ID:       3,
		MimeType: "audio/mpeg",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeAudio{Duration: 180, Title: "Song"},
		},
	}
	out := &apitypes.Message{}
	DocumentMedia(d, out)

	if out.Audio == nil {
		t.Error("Audio should be set for audio documents")
	}
}

func TestDocumentMedia_VoiceClassification(t *testing.T) {
	d := &tg.Document{
		ID:       4,
		MimeType: "audio/ogg",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeAudio{Voice: true, Duration: 5},
		},
	}
	out := &apitypes.Message{}
	DocumentMedia(d, out)

	if out.Voice == nil {
		t.Error("Voice should be set for voice documents")
	}
	if out.Audio != nil {
		t.Error("Audio should NOT be set when it's a voice note")
	}
}

func TestDocumentMedia_PlainDocument(t *testing.T) {
	// A document with no special attributes → plain Document.
	d := &tg.Document{
		ID:       5,
		MimeType: "application/pdf",
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: "report.pdf"},
		},
	}
	out := &apitypes.Message{}
	DocumentMedia(d, out)

	if out.Document == nil {
		t.Error("Document should be set for plain documents")
	}
	if out.Animation != nil || out.Video != nil || out.Audio != nil {
		t.Error("No media type should be set for a plain document")
	}
}
