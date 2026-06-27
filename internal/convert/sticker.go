// Package convert translates between raw tg (MTProto) types and Bot API
// JSON types. sticker.go handles Document → Sticker and StickerSet conversion.
package convert

import (
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
	"github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// StickerFromDocument converts a tg.Document into a Bot API Sticker.
// emojiMap maps document IDs → emoji strings (built from StickerPack).
// setName is the sticker set's short_name (empty for standalone stickers).
func StickerFromDocument(doc *tg.Document, emojiMap map[int64]string, setName string) *types.Sticker {
	if doc == nil {
		return nil
	}
	s := &types.Sticker{
		FileID:       fileid.EncodeDocument(doc.DCID, doc.ID, doc.AccessHash, doc.FileReference, fileid.TypeSticker),
		FileUniqueID: fileid.EncodeDocumentUnique(doc.ID, fileid.TypeSticker),
		FileSize:     int(doc.Size),
	}

	var stickerAttr *tg.DocumentAttributeSticker
	var customEmojiAttr *tg.DocumentAttributeCustomEmoji

	for _, attr := range doc.Attributes {
		switch a := attr.(type) {
		case *tg.DocumentAttributeImageSize:
			s.Width = int(a.W)
			s.Height = int(a.H)
		case *tg.DocumentAttributeSticker:
			stickerAttr = a
		case *tg.DocumentAttributeCustomEmoji:
			customEmojiAttr = a
		case *tg.DocumentAttributeAnimated:
			s.IsAnimated = true
		case *tg.DocumentAttributeVideo:
			s.IsVideo = true
			if s.Width == 0 {
				s.Width = int(a.W)
				s.Height = int(a.H)
			}
		}
	}

	// Sticker format is encoded by mime_type (TGS/Webm/Webp); the animated/video
	// attributes are often absent (e.g. gift custom-emoji stickers), so fall back
	// to mime_type — mirrors TDLib's sticker format detection. TGS = animated,
	// WebM = video, WebP = static.
	if !s.IsAnimated && !s.IsVideo {
		switch doc.MimeType {
		case "application/x-tgsticker":
			s.IsAnimated = true
		case "video/webm":
			s.IsVideo = true
		}
	}

	// Type determination.
	switch {
	case customEmojiAttr != nil:
		s.Type = "custom_emoji"
		s.CustomEmojiID = strconv.FormatInt(doc.ID, 10)
	case stickerAttr != nil && stickerAttr.Mask:
		s.Type = "mask"
	default:
		s.Type = "regular"
	}

	// Emoji: prefer the single attribute Alt (the reference's sticker_->emoji_);
	// a sticker can belong to several packs, and the pack map would join them,
	// but the Bot API emits one emoji. Fall back to the pack map (single emoji)
	// when Alt is empty.
	if stickerAttr != nil && stickerAttr.Alt != "" {
		s.Emoji = stickerAttr.Alt
	} else if customEmojiAttr != nil && customEmojiAttr.Alt != "" {
		s.Emoji = customEmojiAttr.Alt
	} else if emoji, ok := emojiMap[doc.ID]; ok {
		s.Emoji = emoji
	}

	// Set name: prefer the caller-provided name, else the attribute's set short
	// name if present. (Set refs by ID only — common for gifts — need client-
	// layer resolution and are passed in via setName.)
	if setName != "" {
		s.SetName = setName
	} else {
		var set tg.InputStickerSetClass
		switch {
		case stickerAttr != nil:
			set = stickerAttr.Stickerset
		case customEmojiAttr != nil:
			set = customEmojiAttr.Stickerset
		}
		if ss, ok := set.(*tg.InputStickerSetShortName); ok {
			s.SetName = ss.ShortName
		}
	}

	// Mask position.
	if stickerAttr != nil && stickerAttr.MaskCoords != nil {
		s.MaskPosition = maskCoordsToPosition(stickerAttr.MaskCoords)
	}

	// Thumbnail (the reference emits both "thumbnail" and "legacy "thumb").
	if thumb := documentThumbPhotoSize(doc); thumb != nil {
		s.Thumbnail = thumb
		s.Thumb = thumb
	}

	return s
}

// documentThumbPhotoSize picks the largest document thumbnail and encodes it as
// a Bot API PhotoSize, using the document-thumbnail file_id format (TypeThumbnail
// source). Returns nil when the document has no usable thumbnail.
func documentThumbPhotoSize(doc *tg.Document) *types.PhotoSize {
	var best *tg.PhotoSize
	for _, th := range doc.Thumbs {
		if ps, ok := th.(*tg.PhotoSize); ok && ps.Type != "" {
			if best == nil || int(ps.W)*int(ps.H) > int(best.W)*int(best.H) {
				best = ps
			}
		}
	}
	if best == nil {
		return nil
	}
	return &types.PhotoSize{
		FileID:       fileid.EncodeDocumentThumbnail(doc.DCID, doc.ID, doc.AccessHash, doc.FileReference, best.Type[0]),
		FileUniqueID: fileid.EncodeDocumentThumbnailUnique(doc.ID, best.Type[0]),
		FileSize:     int(best.Size),
		Width:        int(best.W),
		Height:       int(best.H),
	}
}

// setThumbPhotoSize builds the Bot API PhotoSize for a sticker set's own
// thumbnail (the StickerSet.thumbnail/thumb field), using the
// StickerSetThumbnailVersion file_id source. Returns nil when the set has no
// versioned thumbnail.
func setThumbPhotoSize(sset *tg.StickerSet) *types.PhotoSize {
	if sset == nil || sset.ThumbVersion == 0 {
		return nil
	}
	var best *tg.PhotoSize
	for _, th := range sset.Thumbs {
		if ps, ok := th.(*tg.PhotoSize); ok {
			if best == nil || int(ps.W)*int(ps.H) > int(best.W)*int(best.H) {
				best = ps
			}
		}
	}
	ps := &types.PhotoSize{
		FileID:       fileid.EncodeStickerSetThumb(sset.ThumbDCID, sset.ID, sset.AccessHash, sset.ThumbVersion),
		FileUniqueID: fileid.EncodeStickerSetThumbUnique(sset.ID, sset.ThumbVersion),
	}
	if best != nil {
		ps.FileSize = int(best.Size)
		ps.Width = int(best.W)
		ps.Height = int(best.H)
	}
	return ps
}

// StickerDocSetID returns the sticker-set ID referenced by a sticker or
// custom-emoji document (via DocumentAttributeSticker/CustomEmoji.Stickerset),
// when that reference is an InputStickerSetID. Used to resolve a sticker's
// set_name client-side (the reference emits the set's short_name).
func StickerDocSetID(doc *tg.Document) (int64, bool) {
	if doc == nil {
		return 0, false
	}
	for _, attr := range doc.Attributes {
		var set tg.InputStickerSetClass
		switch a := attr.(type) {
		case *tg.DocumentAttributeSticker:
			set = a.Stickerset
		case *tg.DocumentAttributeCustomEmoji:
			set = a.Stickerset
		}
		if s, ok := set.(*tg.InputStickerSetID); ok && s.ID != 0 {
			return s.ID, true
		}
	}
	return 0, false
}

// maskCoordsToPosition converts tg.MaskCoords → Bot API MaskPosition.
func maskCoordsToPosition(mc *tg.MaskCoords) *types.MaskPosition {
	if mc == nil {
		return nil
	}
	var point string
	switch mc.N {
	case 0:
		point = "forehead"
	case 1:
		point = "eyes"
	case 2:
		point = "mouth"
	case 3:
		point = "chin"
	default:
		return nil
	}
	return &types.MaskPosition{
		Point:  point,
		XShift: mc.X,
		YShift: mc.Y,
		Scale:  mc.Zoom,
	}
}

// buildEmojiMap converts []*tg.StickerPack into a documentID → emoji map.
// Each StickerPack maps one emoticon to multiple document IDs; a document can
// appear in several packs. The Bot API Sticker.emoji is a single emoji, so the
// first pack's emoticon wins (the attribute Alt is preferred upstream anyway).
func buildEmojiMap(packs []*tg.StickerPack) map[int64]string {
	m := make(map[int64]string)
	for _, p := range packs {
		for _, docID := range p.Documents {
			if _, ok := m[docID]; !ok {
				m[docID] = p.Emoticon
			}
		}
	}
	return m
}

// StickerSetFromMessages converts a tg.MessagesStickerSet (the result of
// messages.getStickerSet) into a Bot API StickerSet with all stickers.
func StickerSetFromMessages(ss *tg.MessagesStickerSet) *types.StickerSet {
	if ss == nil {
		return nil
	}

	// Extract set metadata from StickerSetClass → *tg.StickerSet.
	var sset *tg.StickerSet
	if s, ok := ss.Set.(*tg.StickerSet); ok {
		sset = s
	}

	emojiMap := buildEmojiMap(ss.Packs)

	out := &types.StickerSet{}
	if sset != nil {
		out.Name = sset.ShortName
		out.Title = sset.Title
		if thumb := setThumbPhotoSize(sset); thumb != nil {
			out.Thumbnail = thumb
			out.Thumb = thumb
		}
		// ThumbDocumentID: the set thumbnail is a document (not a versioned
			// thumb). Look it up in the Documents array and build a PhotoSize.
		if out.Thumbnail == nil && sset.ThumbDocumentID != 0 {
			for _, docClass := range ss.Documents {
				if doc, ok := docClass.(*tg.Document); ok && doc.ID == sset.ThumbDocumentID {
					ps := &types.PhotoSize{
						FileID:       fileid.EncodeDocument(doc.DCID, doc.ID, doc.AccessHash, doc.FileReference, fileid.TypeThumbnail),
						FileUniqueID: fileid.EncodeDocumentUnique(doc.ID, fileid.TypeThumbnail),
						FileSize:     int(doc.Size),
					}
					for _, attr := range doc.Attributes {
						if sz, ok := attr.(*tg.DocumentAttributeImageSize); ok {
							ps.Width = int(sz.W)
							ps.Height = int(sz.H)
						}
					}
					out.Thumbnail = ps
					out.Thumb = ps
					break
				}
			}
		}
		switch {
		case sset.Emojis:
			out.StickerType = "custom_emoji"
		case sset.Masks:
			out.StickerType = "mask"
			out.ContainsMasks = true
		default:
			out.StickerType = "regular"
		}
	}

	// Convert each document to a Sticker.
	for _, docClass := range ss.Documents {
		doc, ok := docClass.(*tg.Document)
		if !ok {
			continue
		}
		sticker := StickerFromDocument(doc, emojiMap, out.Name)
		out.Stickers = append(out.Stickers, *sticker)
	}

	return out
}

// StickersFromDocuments converts a slice of tg.Document into []Sticker.
// setName is the sticker set's short name (empty for standalone stickers).
func StickersFromDocuments(docs []tg.DocumentClass, setName string) []types.Sticker {
	var out []types.Sticker
	for _, docClass := range docs {
		doc, ok := docClass.(*tg.Document)
		if !ok {
			continue
		}
		s := StickerFromDocument(doc, nil, setName)
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}

