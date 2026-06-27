package client

import (
	"context"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
)

// TestResolvePaidMediaItemFileIDAndURL covers the non-upload resolution paths
// (attach:// needs a live upload RPC and is exercised via the upload helpers'
// own tests). Validates the fix that replaced the sendPaidMedia placeholder
// (which sent empty ids) and the invoice attach/URL/file_id support.
func TestResolvePaidMediaItemFileIDAndURL(t *testing.T) {
	c := &Client{}
	ctx := context.Background()

	// Photo by file_id → InputMediaPhoto carrying the decoded InputPhoto.
	photoFID := fileid.EncodeThumbnailPhoto(2, 111, 222, []byte("ref"), 'x')
	m, err := c.resolvePaidMediaItem(ctx, newQ("sendinvoice", nil), `{"type":"photo","media":"`+photoFID+`"}`)
	if err != nil {
		t.Fatalf("photo file_id: %v", err)
	}
	ip, ok := m.(*tg.InputMediaPhoto)
	if !ok {
		t.Fatalf("photo file_id: got %T, want *InputMediaPhoto", m)
	}
	photo, ok := ip.ID.(*tg.InputPhoto)
	if !ok || photo.ID != 111 || photo.AccessHash != 222 {
		t.Errorf("photo InputPhoto = %+v, want id=111 access_hash=222", ip.ID)
	}

	// Photo by URL → InputMediaPhotoExternal.
	m, _ = c.resolvePaidMediaItem(ctx, newQ("x", nil), `{"type":"photo","media":"https://e.com/p.jpg"}`)
	if ipe, ok := m.(*tg.InputMediaPhotoExternal); !ok || ipe.URL != "https://e.com/p.jpg" {
		t.Errorf("photo URL: got %+v", m)
	}

	// Video by URL → InputMediaDocumentExternal.
	m, _ = c.resolvePaidMediaItem(ctx, newQ("x", nil), `{"type":"video","media":"https://e.com/v.mp4"}`)
	if ide, ok := m.(*tg.InputMediaDocumentExternal); !ok || ide.URL != "https://e.com/v.mp4" {
		t.Errorf("video URL: got %+v", m)
	}

	// Errors.
	if _, err := c.resolvePaidMediaItem(ctx, newQ("x", nil), `{"type":"bogus","media":"x"}`); err == nil {
		t.Error("bogus type: want error")
	}
	if _, err := c.resolvePaidMediaItem(ctx, newQ("x", nil), `{"type":"photo","media":""}`); err == nil {
		t.Error("empty media: want error")
	}
}

func TestResolvePaidMediaArray(t *testing.T) {
	c := &Client{}
	raw := `[{"type":"photo","media":"https://e.com/a.jpg"},{"type":"video","media":"https://e.com/b.mp4"}]`
	list, err := c.resolvePaidMediaArray(context.Background(), newQ("sendpaidmedia", nil), raw)
	if err != nil {
		t.Fatalf("array: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d, want 2", len(list))
	}
	if _, ok := list[0].(*tg.InputMediaPhotoExternal); !ok {
		t.Errorf("list[0]=%T, want *InputMediaPhotoExternal", list[0])
	}
	if _, ok := list[1].(*tg.InputMediaDocumentExternal); !ok {
		t.Errorf("list[1]=%T, want *InputMediaDocumentExternal", list[1])
	}
}
