package types

// InlineQuery represents an incoming inline query. Field order matches Client.cpp
// JsonInlineQuery::store (5369): id, from, location, chat_type, query, offset.
type InlineQuery struct {
	ID       string    `json:"id"`
	From     *User     `json:"from"`
	Location *Location `json:"location,omitempty"`
	ChatType string    `json:"chat_type,omitempty"`
	Query    string    `json:"query"`
	Offset   string    `json:"offset"`
}

// InlineQueryResult represents one result of an inline query.
// This is a union type — the Type field determines which fields are relevant.
type InlineQueryResult struct {
	Type                string                `json:"type"`
	ID                  string                `json:"id"`
	Title               string                `json:"title,omitempty"`
	Description         string                `json:"description,omitempty"`
	Caption             string                `json:"caption,omitempty"`
	ParseMode           string                `json:"parse_mode,omitempty"`
	CaptionEntities     []MessageEntity       `json:"caption_entities,omitempty"`
	ReplyMarkup         *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
	InputMessageContent any                   `json:"input_message_content,omitempty"`
	// Photo
	PhotoURL    string `json:"photo_url,omitempty"`
	PhotoFileID string `json:"photo_file_id,omitempty"`
	PhotoWidth  int    `json:"photo_width,omitempty"`
	PhotoHeight int    `json:"photo_height,omitempty"`
	// GIF / MPEG4 GIF / Video / Audio / Voice / Document / Sticker
	URL         string `json:"url,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	ThumbURL    string `json:"thumb_url,omitempty"`
	ThumbWidth  int    `json:"thumb_width,omitempty"`
	ThumbHeight int    `json:"thumb_height,omitempty"`
	// Article / Contact / Venue / Location
	PhoneNumber          string  `json:"phone_number,omitempty"`
	FirstName            string  `json:"first_name,omitempty"`
	LastName             string  `json:"last_name,omitempty"`
	Address              string  `json:"address,omitempty"`
	FoursquareID         string  `json:"foursquare_id,omitempty"`
	Latitude             float64 `json:"latitude,omitempty"`
	Longitude            float64 `json:"longitude,omitempty"`
	HorizontalAccuracy   float64 `json:"horizontal_accuracy,omitempty"`
	LivePeriod           int     `json:"live_period,omitempty"`
	Heading              int     `json:"heading,omitempty"`
	ProximityAlertRadius int     `json:"proximity_alert_radius,omitempty"`
	// Game
	GameShortName string `json:"game_short_name,omitempty"`
	// Cached
	CachedPhotoID    string `json:"cached_photo_id,omitempty"`
	CachedGIFID      string `json:"cached_gif_id,omitempty"`
	CachedMpeg4GIFID string `json:"cached_mpeg4_gif_id,omitempty"`
	CachedVideoID    string `json:"cached_video_id,omitempty"`
	CachedAudioID    string `json:"cached_audio_id,omitempty"`
	CachedVoiceID    string `json:"cached_voice_id,omitempty"`
	CachedDocumentID string `json:"cached_document_id,omitempty"`
	CachedStickerID  string `json:"cached_sticker_id,omitempty"`
	// Article link options
	URLs    []string `json:"urls,omitempty"`
	Link    string   `json:"link,omitempty"`
	Content string   `json:"content,omitempty"`
}

// InputTextMessageContent represents the content of a text message to be sent
// as the result of an inline query.
type InputTextMessageContent struct {
	MessageText        string              `json:"message_text"`
	ParseMode          string              `json:"parse_mode,omitempty"`
	Entities           []MessageEntity     `json:"entities,omitempty"`
	LinkPreviewOptions *LinkPreviewOptions `json:"link_preview_options,omitempty"`
}

// InputLocationMessageContent represents the content of a location message.
type InputLocationMessageContent struct {
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	HorizontalAccuracy   float64 `json:"horizontal_accuracy,omitempty"`
	LivePeriod           int     `json:"live_period,omitempty"`
	Heading              int     `json:"heading,omitempty"`
	ProximityAlertRadius int     `json:"proximity_alert_radius,omitempty"`
}

// InputVenueMessageContent represents the content of a venue message.
type InputVenueMessageContent struct {
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Title           string  `json:"title"`
	Address         string  `json:"address"`
	FoursquareID    string  `json:"foursquare_id,omitempty"`
	FoursquareType  string  `json:"foursquare_type,omitempty"`
	GooglePlaceID   string  `json:"google_place_id,omitempty"`
	GooglePlaceType string  `json:"google_place_type,omitempty"`
}

// InputContactMessageContent represents the content of a contact message.
type InputContactMessageContent struct {
	PhoneNumber string `json:"phone_number"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name,omitempty"`
	VCard       string `json:"vcard,omitempty"`
}

// InputInvoiceMessageContent represents the content of an invoice message.
type InputInvoiceMessageContent struct {
	Title                     string         `json:"title"`
	Description               string         `json:"description"`
	Payload                   string         `json:"payload"`
	ProviderToken             string         `json:"provider_token"`
	Currency                  string         `json:"currency"`
	Prices                    []LabeledPrice `json:"prices"`
	MaxTipAmount              int64          `json:"max_tip_amount,omitempty"`
	SuggestedTipAmounts       []int64        `json:"suggested_tip_amounts,omitempty"`
	ProviderData              string         `json:"provider_data,omitempty"`
	PhotoURL                  string         `json:"photo_url,omitempty"`
	PhotoSize                 int            `json:"photo_size,omitempty"`
	PhotoWidth                int            `json:"photo_width,omitempty"`
	PhotoHeight               int            `json:"photo_height,omitempty"`
	NeedName                  bool           `json:"need_name"`
	NeedPhoneNumber           bool           `json:"need_phone_number"`
	NeedEmail                 bool           `json:"need_email"`
	NeedShippingAddress       bool           `json:"need_shipping_address"`
	SendPhoneNumberToProvider bool           `json:"send_phone_number_to_provider"`
	SendEmailToProvider       bool           `json:"send_email_to_provider"`
	IsFlexible                bool           `json:"is_flexible"`
}

// LabeledPrice represents a portion of the price for goods or services.
type LabeledPrice struct {
	Label  string `json:"label"`
	Amount int64  `json:"amount"`
}

// CallbackQuery represents an incoming callback query from a callback button.
type CallbackQuery struct {
	ID              string   `json:"id"`
	From            *User    `json:"from"`
	Message         *Message `json:"message,omitempty"`
	InlineMessageID string   `json:"inline_message_id,omitempty"`
	ChatInstance    string   `json:"chat_instance"`
	Data            string   `json:"data,omitempty"`
	GameShortName   string   `json:"game_short_name,omitempty"`
}

// ChosenInlineResult represents a result of an inline query that was chosen
// by the user and sent to their chat partner. Field order matches Client.cpp
// JsonChosenInlineResult::store (5431): from, location, inline_message_id, query, result_id.
type ChosenInlineResult struct {
	From            *User     `json:"from"`
	Location        *Location `json:"location,omitempty"`
	InlineMessageID string    `json:"inline_message_id,omitempty"`
	Query           string    `json:"query"`
	ResultID        string    `json:"result_id"`
}

// GameHighScore represents one row of the high scores table.
type GameHighScore struct {
	Position int   `json:"position"`
	User     *User `json:"user"`
	Score    int   `json:"score"`
}

// PreparedInlineMessage represents a prepared inline message.
type PreparedInlineMessage struct {
	ID             string `json:"id"`
	ExpirationDate int64  `json:"expiration_date"`
}
