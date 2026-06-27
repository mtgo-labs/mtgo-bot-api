package types

// StickerSet represents a sticker set.
// Reference: Bot API type "StickerSet".
type StickerSet struct {
	Name          string     `json:"name"`
	Title         string     `json:"title"`
	Thumbnail     *PhotoSize `json:"thumbnail,omitempty"`
	Thumb         *PhotoSize `json:"thumb,omitempty"`
	StickerType   string     `json:"sticker_type"`
	ContainsMasks bool       `json:"contains_masks"`
	Stickers      []Sticker  `json:"stickers"`
}
