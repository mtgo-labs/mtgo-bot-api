package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- RichBlock JSON marshaling ---

func TestRichBlockParagraph_JSON(t *testing.T) {
	b := RichBlockParagraph{Type: "paragraph", Text: RichTextPlain("hello")}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"type":"paragraph"`) {
		t.Errorf("missing type; got %s", s)
	}
	if !strings.Contains(s, `"hello"`) {
		t.Errorf("missing text; got %s", s)
	}
}

func TestRichBlockDivider_JSON(t *testing.T) {
	b := RichBlockDivider{Type: "divider"}
	data, _ := json.Marshal(b)
	if !strings.Contains(string(data), `"type":"divider"`) {
		t.Errorf("got %s", data)
	}
}

func TestRichBlockPreformatted_JSON(t *testing.T) {
	b := RichBlockPreformatted{Type: "pre", Text: RichTextPlain("code"), Language: "go"}
	data, _ := json.Marshal(b)
	s := string(data)
	if !strings.Contains(s, `"language":"go"`) {
		t.Errorf("missing language; got %s", s)
	}
}

func TestRichBlockList_JSON(t *testing.T) {
	b := RichBlockList{
		Type:  "list",
		Items: []RichBlockListItem{{Blocks: []RichBlock{&RichBlockParagraph{Type: "paragraph", Text: RichTextPlain("item")}}}},
	}
	data, _ := json.Marshal(b)
	if !strings.Contains(string(data), `"items"`) {
		t.Errorf("missing items; got %s", data)
	}
}

// --- RichText JSON marshaling ---

func TestRichTextPlain_MarshalJSON(t *testing.T) {
	rt := RichTextPlain("plain text")
	data, err := json.Marshal(rt)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"plain text"` {
		t.Errorf("RichTextPlain must marshal to bare string; got %s", data)
	}
}

func TestRichTexts_MarshalJSON(t *testing.T) {
	rt := RichTexts{RichTextPlain("a"), RichTextPlain("b")}
	data, err := json.Marshal(rt)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		t.Errorf("RichTexts must marshal to array; got %s", s)
	}
	if !strings.Contains(s, `"a"`) || !strings.Contains(s, `"b"`) {
		t.Errorf("array must contain elements; got %s", s)
	}
}

func TestRichTextBold_JSON(t *testing.T) {
	b := RichTextBold{Type: "bold", Text: RichTextPlain("x")}
	data, _ := json.Marshal(b)
	s := string(data)
	if !strings.Contains(s, `"type":"bold"`) || !strings.Contains(s, `"text":"x"`) {
		t.Errorf("got %s", s)
	}
}

func TestRichTextUrl_JSON(t *testing.T) {
	u := RichTextURL{Type: "url", Text: RichTextPlain("link"), URL: "https://e.com"}
	data, _ := json.Marshal(u)
	s := string(data)
	if !strings.Contains(s, `"url":"https://e.com"`) {
		t.Errorf("missing url; got %s", s)
	}
}

func TestRichTextEmpty_MarshalJSON(t *testing.T) {
	rt := RichTextPlain("")
	data, _ := json.Marshal(rt)
	if string(data) != `""` {
		t.Errorf("empty plain text; got %s", data)
	}
}

// --- RichBlock interface satisfaction (exercises isRichBlock markers) ---

func TestAllRichBlocks_ImplementInterface(t *testing.T) {
	// Instantiating + using in a []RichBlock exercises the isRichBlock markers.
	blocks := []RichBlock{
		&RichBlockParagraph{Type: "paragraph", Text: RichTextPlain("")},
		&RichBlockSectionHeading{Type: "heading", Text: RichTextPlain("")},
		&RichBlockPreformatted{Type: "pre", Text: RichTextPlain("")},
		&RichBlockFooter{Type: "footer", Text: RichTextPlain("")},
		&RichBlockDivider{Type: "divider"},
		&RichBlockMathematicalExpression{Type: "mathematical_expression", Expression: "a+b"},
		&RichBlockAnchor{Type: "anchor", Name: "x"},
		&RichBlockList{Type: "list"},
		&RichBlockBlockQuotation{Type: "blockquote"},
		&RichBlockPullQuotation{Type: "pullquote", Text: RichTextPlain("")},
		&RichBlockCollage{Type: "collage"},
		&RichBlockSlideshow{Type: "slideshow"},
		&RichBlockTable{Type: "table"},
		&RichBlockDetails{Type: "details", Summary: RichTextPlain("")},
		&RichBlockThinking{Type: "thinking"},
	}
	if len(blocks) != 15 {
		t.Fatalf("expected 15 blocks, got %d", len(blocks))
	}
	// Marshal each — exercises MarshalJSON + isRichBlock markers.
	for i, b := range blocks {
		if _, err := json.Marshal(b); err != nil {
			t.Errorf("block[%d] marshal error: %v", i, err)
		}
	}
}

// --- RichMessage JSON ---

func TestRichMessage_JSON(t *testing.T) {
	rm := RichMessage{
		Blocks: []RichBlock{&RichBlockParagraph{Type: "paragraph", Text: RichTextPlain("hi")}},
		IsRtl:  true,
	}
	data, _ := json.Marshal(rm)
	s := string(data)
	if !strings.Contains(s, `"blocks"`) {
		t.Errorf("missing blocks; got %s", s)
	}
	if !strings.Contains(s, `"is_rtl":true`) {
		t.Errorf("missing is_rtl; got %s", s)
	}
}

// --- User MarshalJSON (full vs minimal) ---

func TestUser_MinimalJSON_NonFull(t *testing.T) {
	u := User{ID: 1, FirstName: "A", full: false}
	data, _ := json.Marshal(u)
	s := string(data)
	if strings.Contains(s, "can_join_groups") {
		t.Errorf("non-full user must not have capability fields; got %s", s)
	}
}

func TestUser_FullJSON_GetMe(t *testing.T) {
	u := User{ID: 1, FirstName: "Bot", IsBot: true}
	u2 := UserForGetMe(&u)
	data, _ := json.Marshal(u2)
	s := string(data)
	if !strings.Contains(s, "can_join_groups") {
		t.Errorf("getMe user must have capability fields; got %s", s)
	}
}

func TestUserForGetMe_Nil(t *testing.T) {
	if UserForGetMe(nil) != nil {
		t.Error("nil should return nil")
	}
}
