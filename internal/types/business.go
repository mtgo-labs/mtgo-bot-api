package types

// BusinessConnection describes a connection of a bot with a business account.
// Reference: Bot API type "BusinessConnection".
type BusinessConnection struct {
	ID        string `json:"id"`
	User      User   `json:"user"`
	UserID    int64  `json:"user_id"`
	Date      int64  `json:"date"`
	CanReply  bool   `json:"can_reply"`
	IsEnabled bool   `json:"is_enabled"`
}
