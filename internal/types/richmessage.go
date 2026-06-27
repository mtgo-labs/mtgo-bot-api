package types

import "encoding/json"

// RichMessage represents a rich message.
// https://core.telegram.org/bots/api#richmessage
type RichMessage struct {
	Blocks []RichBlock `json:"blocks"`
	IsRtl  bool        `json:"is_rtl,omitempty"`
}

// ─── RichBlock (polymorphic union) ───
// https://core.telegram.org/bots/api#richblock
//
// Each concrete RichBlock* struct marshals to a JSON object whose "type" field
// discriminates the variant. The Type field is set by the converter.

type RichBlock interface {
	isRichBlock()
}

// RichBlockParagraph (type "paragraph").
type RichBlockParagraph struct {
	Type string   `json:"type"` // "paragraph"
	Text RichText `json:"text"`
}

func (RichBlockParagraph) isRichBlock() {}

// RichBlockSectionHeading (type "heading").
type RichBlockSectionHeading struct {
	Type string   `json:"type"` // "heading"
	Text RichText `json:"text"`
	Size int32    `json:"size"`
}

func (RichBlockSectionHeading) isRichBlock() {}

// RichBlockPreformatted (type "pre").
type RichBlockPreformatted struct {
	Type     string   `json:"type"` // "pre"
	Text     RichText `json:"text"`
	Language string   `json:"language,omitempty"`
}

func (RichBlockPreformatted) isRichBlock() {}

// RichBlockFooter (type "footer").
type RichBlockFooter struct {
	Type string   `json:"type"` // "footer"
	Text RichText `json:"text"`
}

func (RichBlockFooter) isRichBlock() {}

// RichBlockDivider (type "divider").
type RichBlockDivider struct {
	Type string `json:"type"` // "divider"
}

func (RichBlockDivider) isRichBlock() {}

// RichBlockMathematicalExpression (type "mathematical_expression").
type RichBlockMathematicalExpression struct {
	Type       string `json:"type"` // "mathematical_expression"
	Expression string `json:"expression"`
}

func (RichBlockMathematicalExpression) isRichBlock() {}

// RichBlockAnchor (type "anchor").
type RichBlockAnchor struct {
	Type string `json:"type"` // "anchor"
	Name string `json:"name"`
}

func (RichBlockAnchor) isRichBlock() {}

// RichBlockList (type "list").
type RichBlockList struct {
	Type  string              `json:"type"` // "list"
	Items []RichBlockListItem `json:"items"`
}

func (RichBlockList) isRichBlock() {}

// RichBlockListItem is an item within a RichBlockList.
type RichBlockListItem struct {
	Label       string      `json:"label"`
	Blocks      []RichBlock `json:"blocks"`
	HasCheckbox bool        `json:"has_checkbox,omitempty"`
	IsChecked   bool        `json:"is_checked,omitempty"`
	Type        string      `json:"type,omitempty"`
	Value       string      `json:"value,omitempty"`
}

// RichBlockBlockQuotation (type "blockquote").
type RichBlockBlockQuotation struct {
	Type   string      `json:"type"` // "blockquote"
	Blocks []RichBlock `json:"blocks"`
	Credit RichText    `json:"credit,omitempty"`
}

func (RichBlockBlockQuotation) isRichBlock() {}

// RichBlockPullQuotation (type "pullquote").
type RichBlockPullQuotation struct {
	Type   string   `json:"type"` // "pullquote"
	Text   RichText `json:"text"`
	Credit RichText `json:"credit,omitempty"`
}

func (RichBlockPullQuotation) isRichBlock() {}

// RichBlockCollage (type "collage").
type RichBlockCollage struct {
	Type    string            `json:"type"` // "collage"
	Blocks  []RichBlock       `json:"blocks"`
	Caption *RichBlockCaption `json:"caption,omitempty"`
}

func (RichBlockCollage) isRichBlock() {}

// RichBlockSlideshow (type "slideshow").
type RichBlockSlideshow struct {
	Type    string            `json:"type"` // "slideshow"
	Blocks  []RichBlock       `json:"blocks"`
	Caption *RichBlockCaption `json:"caption,omitempty"`
}

func (RichBlockSlideshow) isRichBlock() {}

// RichBlockTable (type "table").
type RichBlockTable struct {
	Type       string            `json:"type"` // "table"
	Cells      [][]RichTableCell `json:"cells"`
	Caption    RichText          `json:"caption,omitempty"`
	IsBordered bool              `json:"is_bordered,omitempty"`
	IsStriped  bool              `json:"is_striped,omitempty"`
}

func (RichBlockTable) isRichBlock() {}

// RichTableCell is a cell within a RichBlockTable.
type RichTableCell struct {
	Text     RichText `json:"text,omitempty"`
	IsHeader bool     `json:"is_header,omitempty"`
	Colspan  int32    `json:"colspan,omitempty"`
	Rowspan  int32    `json:"rowspan,omitempty"`
	Align    string   `json:"align"`  // "left" | "center" | "right"
	Valign   string   `json:"valign"` // "top" | "middle" | "bottom"
}

// RichBlockDetails (type "details").
type RichBlockDetails struct {
	Type    string      `json:"type"` // "details"
	Summary RichText    `json:"summary"`
	Blocks  []RichBlock `json:"blocks"`
	IsOpen  bool        `json:"is_open,omitempty"`
}

func (RichBlockDetails) isRichBlock() {}

// RichBlockMap (type "map").
type RichBlockMap struct {
	Type     string            `json:"type"` // "map"
	Location *Location         `json:"location"`
	Zoom     int32             `json:"zoom"`
	Width    int32             `json:"width"`
	Height   int32             `json:"height"`
	Caption  *RichBlockCaption `json:"caption,omitempty"`
}

func (RichBlockMap) isRichBlock() {}

// RichBlockAnimation (type "animation").
type RichBlockAnimation struct {
	Type         string            `json:"type"` // "animation"
	Animation    *Animation        `json:"animation"`
	Caption      *RichBlockCaption `json:"caption,omitempty"`
	NeedAutoplay bool              `json:"need_autoplay,omitempty"`
	HasSpoiler   bool              `json:"has_spoiler,omitempty"`
}

func (RichBlockAnimation) isRichBlock() {}

// RichBlockAudio (type "audio").
type RichBlockAudio struct {
	Type    string            `json:"type"` // "audio"
	Audio   *Audio            `json:"audio"`
	Caption *RichBlockCaption `json:"caption,omitempty"`
}

func (RichBlockAudio) isRichBlock() {}

// RichBlockPhoto (type "photo").
type RichBlockPhoto struct {
	Type       string            `json:"type"` // "photo"
	Photo      *PhotoSize        `json:"photo"`
	Caption    *RichBlockCaption `json:"caption,omitempty"`
	HasSpoiler bool              `json:"has_spoiler,omitempty"`
}

func (RichBlockPhoto) isRichBlock() {}

// RichBlockVideo (type "video").
type RichBlockVideo struct {
	Type         string            `json:"type"` // "video"
	Video        *Video            `json:"video"`
	Caption      *RichBlockCaption `json:"caption,omitempty"`
	NeedAutoplay bool              `json:"need_autoplay,omitempty"`
	IsLooped     bool              `json:"is_looped,omitempty"`
	HasSpoiler   bool              `json:"has_spoiler,omitempty"`
}

func (RichBlockVideo) isRichBlock() {}

// RichBlockVoiceNote (type "voice_note").
type RichBlockVoiceNote struct {
	Type      string            `json:"type"` // "voice_note"
	VoiceNote *Voice            `json:"voice_note"`
	Caption   *RichBlockCaption `json:"caption,omitempty"`
}

func (RichBlockVoiceNote) isRichBlock() {}

// RichBlockThinking (type "thinking").
type RichBlockThinking struct {
	Type string `json:"type"` // "thinking"
}

func (RichBlockThinking) isRichBlock() {}

// RichBlockCaption is a caption on media/collage/slideshow/map blocks.
type RichBlockCaption struct {
	Text   RichText `json:"text"`
	Credit RichText `json:"credit,omitempty"`
}

// ─── RichText (polymorphic union) ───
// https://core.telegram.org/bots/api#richtext
//
// RichText is one of:
//   - a plain JSON string (RichTextPlain)
//   - a JSON array of RichText (RichTexts)
//   - a typed JSON object (RichTextBold, …) with a "type" discriminator

type RichText interface {
	isRichText()
}

// RichTextPlain marshals to a bare JSON string.
type RichTextPlain string

func (RichTextPlain) isRichText() {}

// MarshalJSON emits the plain text as a JSON string.
func (p RichTextPlain) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(p))
}

// RichTexts marshals to a JSON array of RichText.
type RichTexts []RichText

func (RichTexts) isRichText() {}

// MarshalJSON emits the slice as a JSON array.
func (s RichTexts) MarshalJSON() ([]byte, error) {
	arr := make([]json.RawMessage, 0, len(s))
	for _, t := range s {
		b, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		arr = append(arr, b)
	}
	return json.Marshal(arr)
}

// RichTextBold (type "bold").
type RichTextBold struct {
	Type string   `json:"type"` // "bold"
	Text RichText `json:"text"`
}

func (RichTextBold) isRichText() {}

// RichTextItalic (type "italic").
type RichTextItalic struct {
	Type string   `json:"type"` // "italic"
	Text RichText `json:"text"`
}

func (RichTextItalic) isRichText() {}

// RichTextUnderline (type "underline").
type RichTextUnderline struct {
	Type string   `json:"type"` // "underline"
	Text RichText `json:"text"`
}

func (RichTextUnderline) isRichText() {}

// RichTextStrikethrough (type "strikethrough").
type RichTextStrikethrough struct {
	Type string   `json:"type"` // "strikethrough"
	Text RichText `json:"text"`
}

func (RichTextStrikethrough) isRichText() {}

// RichTextSpoiler (type "spoiler").
type RichTextSpoiler struct {
	Type string   `json:"type"` // "spoiler"
	Text RichText `json:"text"`
}

func (RichTextSpoiler) isRichText() {}

// RichTextDateTime (type "date_time").
type RichTextDateTime struct {
	Type           string   `json:"type"` // "date_time"
	Text           RichText `json:"text"`
	UnixTime       int32    `json:"unix_time"`
	DateTimeFormat string   `json:"date_time_format"`
}

func (RichTextDateTime) isRichText() {}

// RichTextTextMention (type "text_mention") references a user.
type RichTextTextMention struct {
	Type string   `json:"type"` // "text_mention"
	Text RichText `json:"text"`
	User *User    `json:"user"`
}

func (RichTextTextMention) isRichText() {}

// RichTextSubscript (type "subscript").
type RichTextSubscript struct {
	Type string   `json:"type"` // "subscript"
	Text RichText `json:"text"`
}

func (RichTextSubscript) isRichText() {}

// RichTextSuperscript (type "superscript").
type RichTextSuperscript struct {
	Type string   `json:"type"` // "superscript"
	Text RichText `json:"text"`
}

func (RichTextSuperscript) isRichText() {}

// RichTextMarked (type "marked").
type RichTextMarked struct {
	Type string   `json:"type"` // "marked"
	Text RichText `json:"text"`
}

func (RichTextMarked) isRichText() {}

// RichTextCode (type "code").
type RichTextCode struct {
	Type string   `json:"type"` // "code"
	Text RichText `json:"text"`
}

func (RichTextCode) isRichText() {}

// RichTextCustomEmoji (type "custom_emoji").
type RichTextCustomEmoji struct {
	Type            string `json:"type"` // "custom_emoji"
	CustomEmojiID   string `json:"custom_emoji_id"`
	AlternativeText string `json:"alternative_text"`
}

func (RichTextCustomEmoji) isRichText() {}

// RichTextMathematicalExpression (type "mathematical_expression").
type RichTextMathematicalExpression struct {
	Type       string `json:"type"` // "mathematical_expression"
	Expression string `json:"expression"`
}

func (RichTextMathematicalExpression) isRichText() {}

// RichTextURL (type "url").
type RichTextURL struct {
	Type string   `json:"type"` // "url"
	Text RichText `json:"text"`
	URL  string   `json:"url"`
}

func (RichTextURL) isRichText() {}

// RichTextEmailAddress (type "email_address").
type RichTextEmailAddress struct {
	Type         string   `json:"type"` // "email_address"
	Text         RichText `json:"text"`
	EmailAddress string   `json:"email_address"`
}

func (RichTextEmailAddress) isRichText() {}

// RichTextPhoneNumber (type "phone_number").
type RichTextPhoneNumber struct {
	Type        string   `json:"type"` // "phone_number"
	Text        RichText `json:"text"`
	PhoneNumber string   `json:"phone_number"`
}

func (RichTextPhoneNumber) isRichText() {}

// RichTextBankCardNumber (type "bank_card_number").
type RichTextBankCardNumber struct {
	Type           string   `json:"type"` // "bank_card_number"
	Text           RichText `json:"text"`
	BankCardNumber string   `json:"bank_card_number"`
}

func (RichTextBankCardNumber) isRichText() {}

// RichTextMention (type "mention").
type RichTextMention struct {
	Type     string   `json:"type"` // "mention"
	Text     RichText `json:"text"`
	Username string   `json:"username"`
}

func (RichTextMention) isRichText() {}

// RichTextHashtag (type "hashtag").
type RichTextHashtag struct {
	Type    string   `json:"type"` // "hashtag"
	Text    RichText `json:"text"`
	Hashtag string   `json:"hashtag"`
}

func (RichTextHashtag) isRichText() {}

// RichTextCashtag (type "cashtag").
type RichTextCashtag struct {
	Type    string   `json:"type"` // "cashtag"
	Text    RichText `json:"text"`
	Cashtag string   `json:"cashtag"`
}

func (RichTextCashtag) isRichText() {}

// RichTextBotCommand (type "bot_command").
type RichTextBotCommand struct {
	Type       string   `json:"type"` // "bot_command"
	Text       RichText `json:"text"`
	BotCommand string   `json:"bot_command"`
}

func (RichTextBotCommand) isRichText() {}

// RichTextAnchor (type "anchor").
type RichTextAnchor struct {
	Type string `json:"type"` // "anchor"
	Name string `json:"name"`
}

func (RichTextAnchor) isRichText() {}

// RichTextAnchorLink (type "anchor_link").
type RichTextAnchorLink struct {
	Type       string   `json:"type"` // "anchor_link"
	Text       RichText `json:"text"`
	AnchorName string   `json:"anchor_name"`
}

func (RichTextAnchorLink) isRichText() {}

// RichTextReference (type "reference").
type RichTextReference struct {
	Type string   `json:"type"` // "reference"
	Text RichText `json:"text"`
	Name string   `json:"name"`
}

func (RichTextReference) isRichText() {}

// RichTextReferenceLink (type "reference_link").
type RichTextReferenceLink struct {
	Type          string   `json:"type"` // "reference_link"
	Text          RichText `json:"text"`
	ReferenceName string   `json:"reference_name"`
}

func (RichTextReferenceLink) isRichText() {}
