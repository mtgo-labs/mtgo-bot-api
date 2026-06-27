package types

// BotCommand represents a bot command.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// BotCommandScope represents the scope to which bot commands are applied.
type BotCommandScope struct {
	Type   string `json:"type"`
	ChatID int64  `json:"chat_id,omitempty"`
}

// UserProfilePhotos represents a user's profile photos.
type UserProfilePhotos struct {
	TotalCount int           `json:"total_count"`
	Photos     [][]PhotoSize `json:"photos"`
}
