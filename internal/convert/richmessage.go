package convert

import (
	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// RichMessage converts an MTProto *tg.RichMessage into the Bot API RichMessage
// JSON shape {blocks:[...], is_rtl?}. Mirrors the official JsonRichMessage +
// JsonRichBlocks + JsonRichBlock + JsonRichText (telegram-bot-api Client.cpp).
func RichMessage(rm *tg.RichMessage) *apitypes.RichMessage {
	if rm == nil {
		return nil
	}
	out := &apitypes.RichMessage{IsRtl: rm.Rtl}
	for _, b := range rm.Blocks {
		if blk := richBlockResolve(b, rm.Photos, rm.Documents); blk != nil {
			out.Blocks = append(out.Blocks, blk)
		}
	}
	return out
}

// RichBlock converts a single MTProto PageBlock into a Bot API RichBlock
// WITHOUT media context (photo/video/audio blocks resolve to nil because their
// media is referenced by ID in the parent RichMessage). Used by tests.
func RichBlock(b tg.PageBlockClass) apitypes.RichBlock {
	return richBlockResolve(b, nil, nil)
}

// richBlockResolve is the full block converter; photos/docs are the parent
// RichMessage's media arrays, used to resolve photo/video/audio blocks by ID.
func richBlockResolve(b tg.PageBlockClass, photos []tg.PhotoClass, docs []tg.DocumentClass) apitypes.RichBlock {
	if b == nil {
		return nil
	}
	switch v := b.(type) {
	case *tg.PageBlockParagraph:
		return &apitypes.RichBlockParagraph{Type: "paragraph", Text: RichText(v.Text)}
	case *tg.PageBlockPreformatted:
		return &apitypes.RichBlockPreformatted{Type: "pre", Text: RichText(v.Text), Language: v.Language}
	case *tg.PageBlockFooter:
		return &apitypes.RichBlockFooter{Type: "footer", Text: RichText(v.Text)}
	case *tg.PageBlockHeader:
		return &apitypes.RichBlockSectionHeading{Type: "heading", Text: RichText(v.Text)}
	case *tg.PageBlockDivider:
		return &apitypes.RichBlockDivider{Type: "divider"}
	case *tg.PageBlockMath:
		return &apitypes.RichBlockMathematicalExpression{Type: "mathematical_expression", Expression: v.Source}
	case *tg.PageBlockAnchor:
		return &apitypes.RichBlockAnchor{Type: "anchor", Name: v.Name}
	case *tg.PageBlockBlockquote:
		return &apitypes.RichBlockBlockQuotation{
			Type:   "blockquote",
			Blocks: blockSlice(v.Text),
			Credit: RichText(v.Caption),
		}
	case *tg.PageBlockPullquote:
		return &apitypes.RichBlockPullQuotation{
			Type:   "pullquote",
			Text:   RichText(v.Text),
			Credit: RichText(v.Caption),
		}
	case *tg.PageBlockPhoto:
		photo := findPhoto(photos, v.PhotoID)
		if photo == nil {
			return nil
		}
		return &apitypes.RichBlockPhoto{
			Type:       "photo",
			Photo:      largestPhotoSize(photo),
			Caption:    richCaption(v.Caption),
			HasSpoiler: v.Spoiler,
		}
	case *tg.PageBlockVideo:
		doc := findDocument(docs, v.VideoID)
		if doc == nil {
			return nil
		}
		return &apitypes.RichBlockVideo{
			Type:         "video",
			Video:        Video(doc),
			Caption:      richCaption(v.Caption),
			NeedAutoplay: v.Autoplay,
			IsLooped:     v.Loop,
			HasSpoiler:   v.Spoiler,
		}
	case *tg.PageBlockAudio:
		doc := findDocument(docs, v.AudioID)
		if doc == nil {
			return nil
		}
		return &apitypes.RichBlockAudio{
			Type:    "audio",
			Audio:   Audio(doc),
			Caption: richCaption(v.Caption),
		}
	case *tg.PageBlockList:
		items := make([]apitypes.RichBlockListItem, 0, len(v.Items))
		for _, it := range v.Items {
			if li := richListItem(it); li != nil {
				items = append(items, *li)
			}
		}
		return &apitypes.RichBlockList{Type: "list", Items: items}
	case *tg.PageBlockDetails:
		return &apitypes.RichBlockDetails{
			Type:    "details",
			Summary: RichText(v.Title),
			Blocks:  pageBlockSlice(v.Blocks),
			IsOpen:  v.Open,
		}
	case *tg.PageBlockCollage:
		return &apitypes.RichBlockCollage{Type: "collage", Blocks: pageBlockSlice(v.Items), Caption: richCaption(v.Caption)}
	case *tg.PageBlockSlideshow:
		return &apitypes.RichBlockSlideshow{Type: "slideshow", Blocks: pageBlockSlice(v.Items), Caption: richCaption(v.Caption)}
	default:
		// Unsupported block (media-bearing blocks: photo/video/audio/etc.
		// need media conversion; channel/embed/related-articles are rare in
		// rich messages). Emit nothing rather than a wrong shape.
		return nil
	}
}

// pageBlockSlice converts a []PageBlockClass into []RichBlock.
func pageBlockSlice(items []tg.PageBlockClass) []apitypes.RichBlock {
	out := make([]apitypes.RichBlock, 0, len(items))
	for _, sub := range items {
		if blk := RichBlock(sub); blk != nil {
			out = append(out, blk)
		}
	}
	return out
}

// blockSlice is a legacy helper for the blockquote-style blocks where MTProto
// carries a single RichText; it wraps it as a one-element paragraph list when
// non-nil. (Kept for parity with the official {blocks:[...]} shape.)
func blockSlice(text tg.RichTextClass) []apitypes.RichBlock {
	if text == nil {
		return nil
	}
	return []apitypes.RichBlock{&apitypes.RichBlockParagraph{Type: "paragraph", Text: RichText(text)}}
}

// richListItem converts a PageListItem into a *RichBlockListItem.
func richListItem(it tg.PageListItemClass) *apitypes.RichBlockListItem {
	switch v := it.(type) {
	case *tg.PageListItemText:
		return &apitypes.RichBlockListItem{
			Blocks: []apitypes.RichBlock{&apitypes.RichBlockParagraph{Type: "paragraph", Text: RichText(v.Text)}},
		}
	case *tg.PageListItemBlocks:
		return &apitypes.RichBlockListItem{Blocks: pageBlockSlice(v.Blocks)}
	}
	return nil
}

// richCaption converts a *tg.PageCaption into a *RichBlockCaption.
func richCaption(c *tg.PageCaption) *apitypes.RichBlockCaption {
	if c == nil {
		return nil
	}
	return &apitypes.RichBlockCaption{Text: RichText(c.Text)}
}

// RichText converts an MTProto RichText (tg.RichTextClass) into a Bot API
// RichText. Mirrors the official JsonRichText (telegram-bot-api Client.cpp).
func RichText(t tg.RichTextClass) apitypes.RichText {
	if t == nil {
		return apitypes.RichTextPlain("")
	}
	switch v := t.(type) {
	case *tg.TextPlain:
		return apitypes.RichTextPlain(v.Text)
	case *tg.TextEmpty:
		return apitypes.RichTextPlain("")
	case *tg.TextConcat:
		arr := make(apitypes.RichTexts, 0, len(v.Texts))
		for _, sub := range v.Texts {
			arr = append(arr, RichText(sub))
		}
		return arr
	case *tg.TextBold:
		return &apitypes.RichTextBold{Type: "bold", Text: RichText(v.Text)}
	case *tg.TextItalic:
		return &apitypes.RichTextItalic{Type: "italic", Text: RichText(v.Text)}
	case *tg.TextUnderline:
		return &apitypes.RichTextUnderline{Type: "underline", Text: RichText(v.Text)}
	case *tg.TextStrike:
		return &apitypes.RichTextStrikethrough{Type: "strikethrough", Text: RichText(v.Text)}
	case *tg.TextFixed:
		return &apitypes.RichTextCode{Type: "code", Text: RichText(v.Text)}
	case *tg.TextURL:
		return &apitypes.RichTextURL{Type: "url", Text: RichText(v.Text), URL: v.URL}
	case *tg.TextEmail:
		return &apitypes.RichTextEmailAddress{Type: "email_address", Text: RichText(v.Text), EmailAddress: v.Email}
	case *tg.TextPhone:
		return &apitypes.RichTextPhoneNumber{Type: "phone_number", Text: RichText(v.Text), PhoneNumber: v.Phone}
	case *tg.TextSubscript:
		return &apitypes.RichTextSubscript{Type: "subscript", Text: RichText(v.Text)}
	case *tg.TextSuperscript:
		return &apitypes.RichTextSuperscript{Type: "superscript", Text: RichText(v.Text)}
	case *tg.TextMarked:
		return &apitypes.RichTextMarked{Type: "marked", Text: RichText(v.Text)}
	case *tg.TextMentionName:
		return &apitypes.RichTextTextMention{Type: "text_mention", Text: RichText(v.Text), User: &apitypes.User{ID: v.UserID}}
	default:
		// Unsupported rich-text variant (spoiler, custom emoji, reference,
		// anchor, …). Fall back to a best-effort empty string.
		return apitypes.RichTextPlain("")
	}
}

// findPhoto resolves a PageBlockPhoto.PhotoID against the parent RichMessage's
// Photos slice. Returns nil if not found.
func findPhoto(photos []tg.PhotoClass, id int64) *tg.Photo {
	for _, p := range photos {
		if ph, ok := p.(*tg.Photo); ok && ph.ID == id {
			return ph
		}
	}
	return nil
}

// findDocument resolves a VideoID/AudioID against the parent RichMessage's
// Documents slice. Returns nil if not found.
func findDocument(docs []tg.DocumentClass, id int64) *tg.Document {
	for _, d := range docs {
		if doc, ok := d.(*tg.Document); ok && doc.ID == id {
			return doc
		}
	}
	return nil
}

// largestPhotoSize converts a tg.Photo to its largest Bot API PhotoSize.
func largestPhotoSize(p *tg.Photo) *apitypes.PhotoSize {
	sizes := Photo(p)
	var best *apitypes.PhotoSize
	for i := range sizes {
		if best == nil || sizes[i].Width > best.Width {
			best = &sizes[i]
		}
	}
	return best
}
