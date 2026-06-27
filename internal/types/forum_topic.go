package types

// ForumTopicInfo is returned by createForumTopic. It describes the newly
// created topic. Reference: Bot API createForumTopic response.
type ForumTopicInfo struct {
	MessageThreadID     int32  `json:"message_thread_id"`
	Name                string `json:"name"`
	IconColor           int32  `json:"icon_color"`
	IconCustomEmojiID   string `json:"icon_custom_emoji_id,omitempty"`
}
