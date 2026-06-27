package convert

import (
	"encoding/json"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// jsonOf marshals v and panics on error (test helper).
func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// asJSON unmarshals into a generic value for structural comparison.
func asJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
	return v
}

func TestRichText_PlainString(t *testing.T) {
	got := jsonOf(t, RichText(&tg.TextPlain{Text: "hello"}))
	if got != `"hello"` {
		t.Fatalf("plain text must be a bare JSON string, got %s", got)
	}
}

func TestRichText_Empty(t *testing.T) {
	got := jsonOf(t, RichText(&tg.TextEmpty{}))
	if got != `""` {
		t.Fatalf("empty text must be empty string, got %s", got)
	}
}

func TestRichText_Nil(t *testing.T) {
	if jsonOf(t, RichText(nil)) != `""` {
		t.Fatal("nil rich text must be empty string")
	}
}

func TestRichText_ConcatArray(t *testing.T) {
	rt := RichText(&tg.TextConcat{Texts: []tg.RichTextClass{
		&tg.TextPlain{Text: "a "},
		&tg.TextBold{Text: &tg.TextPlain{Text: "b"}},
	}})
	got := asJSON(t, jsonOf(t, rt))
	arr, ok := got.([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("concat must be a 2-element array, got %v", got)
	}
	if arr[0] != "a " {
		t.Fatalf("first element must be plain string, got %v", arr[0])
	}
}

func TestRichText_TypedVariants(t *testing.T) {
	cases := []struct {
		name string
		in   tg.RichTextClass
		want string // expected JSON (type discriminator check)
	}{
		{"bold", &tg.TextBold{Text: &tg.TextPlain{Text: "x"}}, `{"type":"bold","text":"x"}`},
		{"italic", &tg.TextItalic{Text: &tg.TextPlain{Text: "x"}}, `{"type":"italic","text":"x"}`},
		{"underline", &tg.TextUnderline{Text: &tg.TextPlain{Text: "x"}}, `{"type":"underline","text":"x"}`},
		{"strike", &tg.TextStrike{Text: &tg.TextPlain{Text: "x"}}, `{"type":"strikethrough","text":"x"}`},
		{"fixed", &tg.TextFixed{Text: &tg.TextPlain{Text: "x"}}, `{"type":"code","text":"x"}`},
		{"url", &tg.TextURL{Text: &tg.TextPlain{Text: "x"}, URL: "https://e.com"}, `{"type":"url","text":"x","url":"https://e.com"}`},
		{"email", &tg.TextEmail{Text: &tg.TextPlain{Text: "x"}, Email: "a@b.c"}, `{"type":"email_address","text":"x","email_address":"a@b.c"}`},
		{"phone", &tg.TextPhone{Text: &tg.TextPlain{Text: "x"}, Phone: "+1"}, `{"type":"phone_number","text":"x","phone_number":"+1"}`},
		{"subscript", &tg.TextSubscript{Text: &tg.TextPlain{Text: "x"}}, `{"type":"subscript","text":"x"}`},
		{"superscript", &tg.TextSuperscript{Text: &tg.TextPlain{Text: "x"}}, `{"type":"superscript","text":"x"}`},
		{"marked", &tg.TextMarked{Text: &tg.TextPlain{Text: "x"}}, `{"type":"marked","text":"x"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := jsonOf(t, RichText(c.in))
			if got != c.want {
				t.Errorf("\n got %s\nwant %s", got, c.want)
			}
		})
	}
}

func TestRichText_MentionName(t *testing.T) {
	got := jsonOf(t, RichText(&tg.TextMentionName{Text: &tg.TextPlain{Text: "u"}, UserID: 42}))
	// user is a nested object; just verify the type + user.id are present.
	v := asJSON(t, got).(map[string]any)
	if v["type"] != "text_mention" {
		t.Fatalf("type: got %v", v["type"])
	}
	if user, ok := v["user"].(map[string]any); !ok || user["id"] != float64(42) {
		t.Fatalf("user.id: got %v", v["user"])
	}
}

func TestRichBlock_Paragraph(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockParagraph{Text: &tg.TextPlain{Text: "hi"}}))
	want := `{"type":"paragraph","text":"hi"}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRichBlock_Preformatted(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockPreformatted{Text: &tg.TextPlain{Text: "code"}, Language: "go"}))
	want := `{"type":"pre","text":"code","language":"go"}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRichBlock_Preformatted_NoLanguage(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockPreformatted{Text: &tg.TextPlain{Text: "code"}}))
	want := `{"type":"pre","text":"code"}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRichBlock_Footer(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockFooter{Text: &tg.TextPlain{Text: "f"}}))
	want := `{"type":"footer","text":"f"}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRichBlock_Divider(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockDivider{}))
	want := `{"type":"divider"}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRichBlock_Math(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockMath{Source: "a+b"}))
	want := `{"type":"mathematical_expression","expression":"a+b"}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRichBlock_Anchor(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockAnchor{Name: "sec1"}))
	want := `{"type":"anchor","name":"sec1"}`
	if got != want {
		t.Errorf("\n got %s\nwant %s", got, want)
	}
}

func TestRichBlock_Blockquote(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockBlockquote{
		Text:    &tg.TextPlain{Text: "q"},
		Caption: &tg.TextPlain{Text: "c"},
	}))
	v := asJSON(t, got).(map[string]any)
	if v["type"] != "blockquote" {
		t.Fatalf("type: %v", v["type"])
	}
	if v["credit"] != "c" {
		t.Fatalf("credit: %v", v["credit"])
	}
}

func TestRichBlock_Pullquote(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockPullquote{
		Text:    &tg.TextPlain{Text: "q"},
		Caption: &tg.TextPlain{Text: "c"},
	}))
	v := asJSON(t, got).(map[string]any)
	if v["type"] != "pullquote" || v["text"] != "q" || v["credit"] != "c" {
		t.Fatalf("got %v", v)
	}
}

func TestRichBlock_List(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockList{Items: []tg.PageListItemClass{
		&tg.PageListItemText{Text: &tg.TextPlain{Text: "item1"}},
	}}))
	v := asJSON(t, got).(map[string]any)
	if v["type"] != "list" {
		t.Fatalf("type: %v", v["type"])
	}
	items, ok := v["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items: %v", v["items"])
	}
}

func TestRichBlock_Details(t *testing.T) {
	got := jsonOf(t, RichBlock(&tg.PageBlockDetails{
		Title:  &tg.TextPlain{Text: "summary"},
		Blocks: []tg.PageBlockClass{&tg.PageBlockParagraph{Text: &tg.TextPlain{Text: "body"}}},
		Open:   true,
	}))
	v := asJSON(t, got).(map[string]any)
	if v["type"] != "details" || v["summary"] != "summary" || v["is_open"] != true {
		t.Fatalf("got %v", v)
	}
}

func TestRichBlock_UnknownReturnsNil(t *testing.T) {
	// PageBlockTitle is not mapped → nil (skipped).
	if got := RichBlock(&tg.PageBlockTitle{}); got != nil {
		t.Fatalf("unmapped block must return nil, got %v", got)
	}
}

func TestRichMessage_Nil(t *testing.T) {
	if RichMessage(nil) != nil {
		t.Fatal("nil input must return nil")
	}
}

func TestRichMessage_FullStructure(t *testing.T) {
	rm := &tg.RichMessage{
		Rtl: true,
		Blocks: []tg.PageBlockClass{
			&tg.PageBlockParagraph{Text: &tg.TextConcat{Texts: []tg.RichTextClass{
				&tg.TextPlain{Text: "hello "},
				&tg.TextBold{Text: &tg.TextPlain{Text: "world"}},
			}}},
			&tg.PageBlockDivider{},
			&tg.PageBlockParagraph{Text: &tg.TextPlain{Text: "end"}},
		},
	}
	out := RichMessage(rm)
	got := asJSON(t, jsonOf(t, out)).(map[string]any)
	if got["is_rtl"] != true {
		t.Errorf("is_rtl: %v", got["is_rtl"])
	}
	blocks, ok := got["blocks"].([]any)
	if !ok || len(blocks) != 3 {
		t.Fatalf("blocks: %v", got["blocks"])
	}
	// First block: paragraph with concat text → ["hello ", {"type":"bold","text":"world"}]
	p0 := blocks[0].(map[string]any)
	if p0["type"] != "paragraph" {
		t.Errorf("block0 type: %v", p0["type"])
	}
	text, ok := p0["text"].([]any)
	if !ok || len(text) != 2 {
		t.Fatalf("block0 text: %v", p0["text"])
	}
	if text[0] != "hello " {
		t.Errorf("block0 text[0]: %v", text[0])
	}
	bold, ok := text[1].(map[string]any)
	if !ok || bold["type"] != "bold" || bold["text"] != "world" {
		t.Errorf("block0 text[1]: %v", text[1])
	}
}

func TestRichMessage_MediaBlocks(t *testing.T) {
	rm := &tg.RichMessage{
		Blocks: []tg.PageBlockClass{
			&tg.PageBlockPhoto{PhotoID: 7, Caption: &tg.PageCaption{Text: &tg.TextPlain{Text: "cap"}}},
			&tg.PageBlockPhoto{PhotoID: 999}, // unresolved → omitted
			&tg.PageBlockVideo{VideoID: 100},
			&tg.PageBlockAudio{AudioID: 200},
		},
		Photos: []tg.PhotoClass{
			&tg.Photo{ID: 7, Sizes: []tg.PhotoSizeClass{&tg.PhotoSize{Type: "x", W: 50, H: 50, Size: 1000}}},
		},
		Documents: []tg.DocumentClass{
			&tg.Document{ID: 100},
			&tg.Document{ID: 200},
		},
	}
	out := RichMessage(rm)
	got := asJSON(t, jsonOf(t, out)).(map[string]any)
	blocks, ok := got["blocks"].([]any)
	if !ok || len(blocks) != 3 {
		t.Fatalf("blocks len = %d, want 3 (unresolved photo 999 must be omitted): %v", len(blocks), got["blocks"])
	}
	wantTypes := []string{"photo", "video", "audio"}
	for i, want := range wantTypes {
		blk, ok := blocks[i].(map[string]any)
		if !ok || blk["type"] != want {
			t.Errorf("block%d = %v, want type %q", i, blocks[i], want)
		}
	}
	// The photo block must carry the resolved media + caption.
	photo := blocks[0].(map[string]any)
	if _, hasPhoto := photo["photo"]; !hasPhoto {
		t.Errorf("photo block missing media field: %v", photo)
	}
}

func TestRichMessage_TypeSatisfaction(t *testing.T) {
	// Ensure all concrete RichBlock/RichText structs satisfy their interfaces
	// (compile-time guard against drift).
	var _ apitypes.RichBlock = apitypes.RichBlockParagraph{}
	var _ apitypes.RichBlock = apitypes.RichBlockSectionHeading{}
	var _ apitypes.RichBlock = apitypes.RichBlockPreformatted{}
	var _ apitypes.RichBlock = apitypes.RichBlockFooter{}
	var _ apitypes.RichBlock = apitypes.RichBlockDivider{}
	var _ apitypes.RichBlock = apitypes.RichBlockMathematicalExpression{}
	var _ apitypes.RichBlock = apitypes.RichBlockAnchor{}
	var _ apitypes.RichBlock = apitypes.RichBlockList{}
	var _ apitypes.RichBlock = apitypes.RichBlockBlockQuotation{}
	var _ apitypes.RichBlock = apitypes.RichBlockPullQuotation{}
	var _ apitypes.RichBlock = apitypes.RichBlockCollage{}
	var _ apitypes.RichBlock = apitypes.RichBlockSlideshow{}
	var _ apitypes.RichBlock = apitypes.RichBlockTable{}
	var _ apitypes.RichBlock = apitypes.RichBlockDetails{}
	var _ apitypes.RichBlock = apitypes.RichBlockMap{}
	var _ apitypes.RichBlock = apitypes.RichBlockAnimation{}
	var _ apitypes.RichBlock = apitypes.RichBlockAudio{}
	var _ apitypes.RichBlock = apitypes.RichBlockPhoto{}
	var _ apitypes.RichBlock = apitypes.RichBlockVideo{}
	var _ apitypes.RichBlock = apitypes.RichBlockVoiceNote{}
	var _ apitypes.RichBlock = apitypes.RichBlockThinking{}

	var _ apitypes.RichText = apitypes.RichTextPlain("")
	var _ apitypes.RichText = apitypes.RichTexts{}
	var _ apitypes.RichText = apitypes.RichTextBold{}
	var _ apitypes.RichText = apitypes.RichTextItalic{}
	var _ apitypes.RichText = apitypes.RichTextUnderline{}
	var _ apitypes.RichText = apitypes.RichTextStrikethrough{}
	var _ apitypes.RichText = apitypes.RichTextSpoiler{}
	var _ apitypes.RichText = apitypes.RichTextDateTime{}
	var _ apitypes.RichText = apitypes.RichTextTextMention{}
	var _ apitypes.RichText = apitypes.RichTextSubscript{}
	var _ apitypes.RichText = apitypes.RichTextSuperscript{}
	var _ apitypes.RichText = apitypes.RichTextMarked{}
	var _ apitypes.RichText = apitypes.RichTextCode{}
	var _ apitypes.RichText = apitypes.RichTextCustomEmoji{}
	var _ apitypes.RichText = apitypes.RichTextMathematicalExpression{}
	var _ apitypes.RichText = apitypes.RichTextURL{}
	var _ apitypes.RichText = apitypes.RichTextEmailAddress{}
	var _ apitypes.RichText = apitypes.RichTextPhoneNumber{}
	var _ apitypes.RichText = apitypes.RichTextBankCardNumber{}
	var _ apitypes.RichText = apitypes.RichTextMention{}
	var _ apitypes.RichText = apitypes.RichTextHashtag{}
	var _ apitypes.RichText = apitypes.RichTextCashtag{}
	var _ apitypes.RichText = apitypes.RichTextBotCommand{}
	var _ apitypes.RichText = apitypes.RichTextAnchor{}
	var _ apitypes.RichText = apitypes.RichTextAnchorLink{}
	var _ apitypes.RichText = apitypes.RichTextReference{}
	var _ apitypes.RichText = apitypes.RichTextReferenceLink{}
}
