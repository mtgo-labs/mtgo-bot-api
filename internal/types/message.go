package types

import "encoding/json"

// Message represents a Telegram message (Bot API type "Message").
// This is the core return type for sendMessage, forwardMessage, etc.
//
// Fields use snake_case json tags matching the Bot API spec. Booleans never
// use omitempty (Constitution Principle 1). Optional pointer/slice fields use
// omitempty so they are absent from JSON when nil.
type Message struct {
	// Header (Client.cpp:4673-4697).
	BusinessConnectionID string `json:"business_connection_id,omitempty"`
	SenderBusinessBot    *User  `json:"sender_business_bot,omitempty"`
	MessageID            int64  `json:"message_id"`
	From                 *User  `json:"from,omitempty"`
	AuthorSignature      string `json:"author_signature,omitempty"`
	SenderChat           *Chat  `json:"sender_chat,omitempty"`
	Chat                 Chat   `json:"chat"`
	Date                 int64  `json:"date"`
	EditDate             int64  `json:"edit_date,omitempty"`
	MessageThreadID      int64  `json:"message_thread_id,omitempty"`
	// Forward origin (4698-4739).
	ForwardOrigin        *MessageOrigin `json:"forward_origin,omitempty"`
	IsAutomaticForward   bool           `json:"is_automatic_forward,omitempty"`
	ForwardFrom          *User          `json:"forward_from,omitempty"`
	ForwardFromChat      *Chat          `json:"forward_from_chat,omitempty"`
	ForwardSignature     string         `json:"forward_signature,omitempty"`
	ForwardFromMessageID int64          `json:"forward_from_message_id,omitempty"`
	ForwardSenderName    string         `json:"forward_sender_name,omitempty"`
	ForwardDate          int64          `json:"forward_date,omitempty"`
	// Reply (4740-4768).
	ReplyToMessage         *Message           `json:"reply_to_message,omitempty"`
	ExternalReply          *ExternalReplyInfo `json:"external_reply,omitempty"`
	Quote                  *TextQuote         `json:"quote,omitempty"`
	ReplyToChecklistTaskID int                `json:"reply_to_checklist_task_id,omitempty"`
	ReplyToPollOptionID    int                `json:"reply_to_poll_option_id,omitempty"`
	ReplyToStory           *Story             `json:"reply_to_story,omitempty"`
	// Grouping (4769-4774).
	MediaGroupID string `json:"media_group_id,omitempty"`
	GuestQueryID string `json:"guest_query_id,omitempty"`
	// Text content (4776-4785).
	Text               string              `json:"text,omitempty"`
	Entities           []MessageEntity     `json:"entities,omitempty"`
	LinkPreviewOptions *LinkPreviewOptions `json:"link_preview_options,omitempty"`
	// Media content (4787-4847).
	Animation *Animation     `json:"animation,omitempty"`
	Audio     *Audio         `json:"audio,omitempty"`
	Document  *Document      `json:"document,omitempty"`
	PaidMedia *PaidMediaInfo `json:"paid_media,omitempty"`
	LivePhoto *LivePhoto     `json:"live_photo,omitempty"`
	Photo     []PhotoSize    `json:"photo,omitempty"`
	Sticker   *Sticker       `json:"sticker,omitempty"`
	Video     *Video         `json:"video,omitempty"`
	VideoNote *VideoNote     `json:"video_note,omitempty"`
	Voice     *Voice         `json:"voice,omitempty"`
	// Caption (add_caption, after media).
	Caption               string          `json:"caption,omitempty"`
	CaptionEntities       []MessageEntity `json:"caption_entities,omitempty"`
	ShowCaptionAboveMedia bool            `json:"show_caption_above_media,omitempty"`
	HasMediaSpoiler       bool            `json:"has_media_spoiler,omitempty"`
	// Other content (4848-4893).
	Contact  *Contact  `json:"contact,omitempty"`
	Dice     *Dice     `json:"dice,omitempty"`
	Game     *Game     `json:"game,omitempty"`
	Invoice  *Invoice  `json:"invoice,omitempty"`
	Location *Location `json:"location,omitempty"`
	Venue    *Venue    `json:"venue,omitempty"`
	Poll     *Poll     `json:"poll,omitempty"`
	// Service messages (4894-5290).
	NewChatParticipant            *User                          `json:"new_chat_participant,omitempty"`
	NewChatMember                 *User                          `json:"new_chat_member,omitempty"`
	NewChatMembers                []User                         `json:"new_chat_members,omitempty"`
	LeftChatParticipant           *User                          `json:"left_chat_participant,omitempty"`
	LeftChatMember                *User                          `json:"left_chat_member,omitempty"`
	NewChatTitle                  string                         `json:"new_chat_title,omitempty"`
	NewChatPhoto                  []PhotoSize                    `json:"new_chat_photo,omitempty"`
	DeleteChatPhoto               bool                           `json:"delete_chat_photo,omitempty"`
	GroupChatCreated              bool                           `json:"group_chat_created,omitempty"`
	SupergroupChatCreated         bool                           `json:"supergroup_chat_created,omitempty"`
	ChannelChatCreated            bool                           `json:"channel_chat_created,omitempty"`
	MessageAutoDeleteTimerChanged *MessageAutoDeleteTimerChanged `json:"message_auto_delete_timer_changed,omitempty"`
	MigrateToChatID               int64                          `json:"migrate_to_chat_id,omitempty"`
	MigrateFromChatID             int64                          `json:"migrate_from_chat_id,omitempty"`
	PinnedMessage                 *Message                       `json:"pinned_message,omitempty"`
	SuccessfulPayment             *SuccessfulPayment             `json:"successful_payment,omitempty"`
	RefundedPayment               *RefundedPayment               `json:"refunded_payment,omitempty"`
	UsersShared                   *UsersShared                   `json:"users_shared,omitempty"`
	UserShared                    *UserShared                    `json:"user_shared,omitempty"`
	ChatShared                    *ChatShared                    `json:"chat_shared,omitempty"`
	ConnectedWebsite              string                         `json:"connected_website,omitempty"`
	WriteAccessAllowed            *WriteAccessAllowed            `json:"write_access_allowed,omitempty"`
	PassportData                  *PassportData                  `json:"passport_data,omitempty"`
	ProximityAlertTriggered       *ProximityAlertTriggered       `json:"proximity_alert_triggered,omitempty"`
	ChatBoostAdded                *ChatBoostAdded                `json:"boost_added,omitempty"`
	ChatBackgroundSet             *ChatBackground                `json:"chat_background_set,omitempty"`
	ForumTopicCreated             *ForumTopicCreated             `json:"forum_topic_created,omitempty"`
	ForumTopicEdited              *ForumTopicEdited              `json:"forum_topic_edited,omitempty"`
	ForumTopicClosed              *ForumTopicClosed              `json:"forum_topic_closed,omitempty"`
	ForumTopicReopened            *ForumTopicReopened            `json:"forum_topic_reopened,omitempty"`
	GeneralForumTopicHidden       *GeneralForumTopicHidden       `json:"general_forum_topic_hidden,omitempty"`
	GeneralForumTopicUnhidden     *GeneralForumTopicUnhidden     `json:"general_forum_topic_unhidden,omitempty"`
	GiveawayCreated               *GiveawayCreated               `json:"giveaway_created,omitempty"`
	Giveaway                      *Giveaway                      `json:"giveaway,omitempty"`
	GiveawayWinners               *GiveawayWinners               `json:"giveaway_winners,omitempty"`
	GiveawayCompleted             *GiveawayCompleted             `json:"giveaway_completed,omitempty"`
	VideoChatScheduled            *VideoChatScheduled            `json:"video_chat_scheduled,omitempty"`
	VoiceChatScheduled            *VideoChatScheduled            `json:"voice_chat_scheduled,omitempty"`
	VideoChatStarted              *VideoChatStarted              `json:"video_chat_started,omitempty"`
	VoiceChatStarted              *VideoChatStarted              `json:"voice_chat_started,omitempty"`
	VideoChatEnded                *VideoChatEnded                `json:"video_chat_ended,omitempty"`
	VoiceChatEnded                *VideoChatEnded                `json:"voice_chat_ended,omitempty"`
	VideoChatParticipantsInvited  *VideoChatParticipantsInvited  `json:"video_chat_participants_invited,omitempty"`
	VoiceChatParticipantsInvited  *VideoChatParticipantsInvited  `json:"voice_chat_participants_invited,omitempty"`
	WebAppData                    *WebAppData                    `json:"web_app_data,omitempty"`
	Story                         *Story                         `json:"story,omitempty"`
	Gift                          *GiftInfo                      `json:"gift,omitempty"`
	GiftUpgradeSent               *GiftInfo                      `json:"gift_upgrade_sent,omitempty"`
	UniqueGift                    *UniqueGiftInfo                `json:"unique_gift,omitempty"`
	Checklist                     *Checklist                     `json:"checklist,omitempty"`
	ChecklistTasksDone            *ChecklistTasksDone            `json:"checklist_tasks_done,omitempty"`
	ChecklistTasksAdded           *ChecklistTasksAdded           `json:"checklist_tasks_added,omitempty"`
	SuggestedPostApproved         *SuggestedPostApproved         `json:"suggested_post_approved,omitempty"`
	SuggestedPostDeclined         *SuggestedPostDeclined         `json:"suggested_post_declined,omitempty"`
	SuggestedPostPaid             *SuggestedPostPaid             `json:"suggested_post_paid,omitempty"`
	SuggestedPostRefunded         *SuggestedPostRefunded         `json:"suggested_post_refunded,omitempty"`
	SuggestedPostApprovalFailed   *SuggestedPostApprovalFailed   `json:"suggested_post_approval_failed,omitempty"`
	PollOptionAdded               *PollOptionAdded               `json:"poll_option_added,omitempty"`
	PollOptionDeleted             *PollOptionDeleted             `json:"poll_option_deleted,omitempty"`
	ManagedBotCreated             *ManagedBotCreated             `json:"managed_bot_created,omitempty"`
	ChatOwnerLeft                 *ChatOwnerLeft                 `json:"chat_owner_left,omitempty"`
	ChatOwnerChanged              *ChatOwnerChanged              `json:"chat_owner_changed,omitempty"`
	DirectMessagePriceChanged     *DirectMessagePriceChanged     `json:"direct_message_price_changed,omitempty"`
	PaidMessagePriceChanged       *PaidMessagePriceChanged       `json:"paid_message_price_changed,omitempty"`
	// Trailing scalars (5291-5337).
	ReplyMarkup         *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
	ViaBot              *User                 `json:"via_bot,omitempty"`
	HasProtectedContent bool                  `json:"has_protected_content,omitempty"`
	IsTopicMessage      bool                  `json:"is_topic_message,omitempty"`
	DirectMessagesTopic *DirectMessagesTopic  `json:"direct_messages_topic,omitempty"`
	IsFromOffline       bool                  `json:"is_from_offline,omitempty"`
	EffectID            string                `json:"effect_id,omitempty"`
	SenderBoostCount    int                   `json:"sender_boost_count,omitempty"`
	SenderTag           string                `json:"sender_tag,omitempty"`
	PaidStarCount       int64                 `json:"paid_star_count,omitempty"`
	IsPaidPost          bool                  `json:"is_paid_post,omitempty"`
	SuggestedPostInfo   *SuggestedPostInfo    `json:"suggested_post_info,omitempty"`
	// Non-reference: custom rich-message rendering.
	RichMessage *RichMessage `json:"rich_message,omitempty"`
}

// DirectMessagesTopic identifies a direct messages topic (reference
// JsonDirectMessagesTopic, Client.cpp:4266-4267: topic_id + user, both always).
type DirectMessagesTopic struct {
	TopicID int64 `json:"topic_id"`
	User    *User `json:"user"`
}

// MessageEntity represents a special entity in a text message (Bot API type "MessageEntity").
type MessageEntity struct {
	Offset         int    `json:"offset"`
	Length         int    `json:"length"`
	Type           string `json:"type"`
	URL            string `json:"url,omitempty"`
	User           *User  `json:"user,omitempty"`
	Language       string `json:"language,omitempty"`
	CustomEmojiID  string `json:"custom_emoji_id,omitempty"`
	UnixTime       int    `json:"unix_time,omitempty"`
	DateTimeFormat string `json:"date_time_format,omitempty"`
}

// LinkPreviewOptions describes options for link preview generation.
type LinkPreviewOptions struct {
	IsDisabled       bool   `json:"is_disabled,omitempty"`
	URL              string `json:"url,omitempty"`
	PreferSmallMedia bool   `json:"prefer_small_media,omitempty"`
	PreferLargeMedia bool   `json:"prefer_large_media,omitempty"`
	ShowAboveText    bool   `json:"show_above_text,omitempty"`
}

// MessageAutoDeleteTimerChanged represents a change in auto-delete timer settings.
type MessageAutoDeleteTimerChanged struct {
	MessageAutoDeleteTime int `json:"message_auto_delete_time"`
}

// ForumTopicCreated represents a service message about forum topic creation.
type ForumTopicCreated struct {
	Name              string `json:"name"`
	IconColor         int    `json:"icon_color"`
	IconCustomEmojiID string `json:"icon_custom_emoji_id,omitempty"`
}

// ForumTopicEdited represents a service message about forum topic editing.
type ForumTopicEdited struct {
	Name              string `json:"name,omitempty"`
	IconCustomEmojiID string `json:"icon_custom_emoji_id,omitempty"`
}

// ForumTopicClosed represents a service message about forum topic closure.
type ForumTopicClosed struct{}

// ForumTopicReopened represents a service message about forum topic reopening.
type ForumTopicReopened struct{}

// GeneralForumTopicHidden represents a service message about general forum topic hiding.
type GeneralForumTopicHidden struct{}

// GeneralForumTopicUnhidden represents a service message about general forum topic unhiding.
type GeneralForumTopicUnhidden struct{}

// InlineKeyboardMarkup represents an inline keyboard attached to a message.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton represents a button in an inline keyboard.
type InlineKeyboardButton struct {
	Text                         string                       `json:"text"`
	URL                          string                       `json:"url,omitempty"`
	CallbackData                 string                       `json:"callback_data,omitempty"`
	WebApp                       *WebAppInfo                  `json:"web_app,omitempty"`
	LoginURL                     *LoginURL                    `json:"login_url,omitempty"`
	SwitchInlineQuery            string                       `json:"switch_inline_query,omitempty"`
	SwitchInlineQueryCurrentChat string                       `json:"switch_inline_query_current_chat,omitempty"`
	SwitchInlineQueryChosenChat  *SwitchInlineQueryChosenChat `json:"switch_inline_query_chosen_chat,omitempty"`
	Pay                          bool                         `json:"pay,omitempty"`
}

// WebAppInfo contains information about a Web App.
type WebAppInfo struct {
	URL string `json:"url"`
}

// LoginURL contains information about a login URL.
type LoginURL struct {
	URL                string `json:"url"`
	ForwardText        string `json:"forward_text,omitempty"`
	BotUsername        string `json:"bot_username,omitempty"`
	RequestWriteAccess bool   `json:"request_write_access"`
}

// SwitchInlineQueryChosenChat represents an inline query chat selection.
type SwitchInlineQueryChosenChat struct {
	Query             string `json:"query,omitempty"`
	AllowUserChats    bool   `json:"allow_user_chats"`
	AllowBotChats     bool   `json:"allow_bot_chats"`
	AllowGroupChats   bool   `json:"allow_group_chats"`
	AllowChannelChats bool   `json:"allow_channel_chats"`
}

// PhotoSize represents a photo size.
type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size,omitempty"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// Animation represents an animation file.
// Animation represents a GIF/animation. Field order matches Client.cpp
// JsonAnimation::store: file_name, mime_type, duration/width/height, thumbnail,
// then the common file fields.
type Animation struct {
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	Duration     int        `json:"duration"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	Thumb        *PhotoSize `json:"thumb,omitempty"`
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	FileSize     int        `json:"file_size,omitempty"`
}

// Audio represents an audio file. Field order matches Client.cpp JsonAudio::store.
type Audio struct {
	Duration     int        `json:"duration"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	Title        string     `json:"title,omitempty"`
	Performer    string     `json:"performer,omitempty"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	Thumb        *PhotoSize `json:"thumb,omitempty"`
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	FileSize     int        `json:"file_size,omitempty"`
}

// Document represents a general file. Field order matches Client.cpp
// JsonDocument::store: file_name, mime_type, thumbnail, then file fields.
type Document struct {
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	Thumb        *PhotoSize `json:"thumb,omitempty"`
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	FileSize     int        `json:"file_size,omitempty"`
}

// Video represents a video file. Field order matches Client.cpp JsonVideo::store.
type Video struct {
	Duration       int            `json:"duration"`
	Width          int            `json:"width"`
	Height         int            `json:"height"`
	FileName       string         `json:"file_name,omitempty"`
	MimeType       string         `json:"mime_type,omitempty"`
	Cover          *PhotoSize     `json:"cover,omitempty"`
	StartTimestamp int            `json:"start_timestamp,omitempty"`
	Qualities      []VideoQuality `json:"qualities,omitempty"`
	Thumbnail      *PhotoSize     `json:"thumbnail,omitempty"`
	Thumb          *PhotoSize     `json:"thumb,omitempty"`
	FileID         string         `json:"file_id"`
	FileUniqueID   string         `json:"file_unique_id"`
	FileSize       int            `json:"file_size,omitempty"`
}

// VideoQuality describes one alternative video stream (reference JsonVideoQuality,
// Client.cpp:2358: width/height/codec, all always present).
type VideoQuality struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Codec  string `json:"codec"`
}

// VideoNote represents a video message. Field order matches Client.cpp
// JsonVideoNote::store: duration, length, thumbnail, then file fields.
type VideoNote struct {
	Duration     int        `json:"duration"`
	Length       int        `json:"length"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	Thumb        *PhotoSize `json:"thumb,omitempty"`
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	FileSize     int        `json:"file_size,omitempty"`
}

// Voice represents a voice note. Field order matches Client.cpp JsonVoiceNote::store.
type Voice struct {
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type,omitempty"`
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size,omitempty"`
}

// Contact represents a phone contact.
// Contact represents a phone contact. Field order matches Client.cpp
// JsonContact::store (2550): phone_number, first_name, last_name, vcard, user_id.
type Contact struct {
	PhoneNumber string `json:"phone_number"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name,omitempty"`
	VCard       string `json:"vcard,omitempty"`
	UserID      int64  `json:"user_id,omitempty"`
}

// Dice represents an animated emoji that displays a random value.
type Dice struct {
	Emoji string `json:"emoji"`
	Value int    `json:"value"`
}

// Game represents a game. Field order matches Client.cpp JsonGame::store (2588):
// title, text, text_entities, description, photo, animation.
type Game struct {
	Title        string          `json:"title"`
	Text         string          `json:"text,omitempty"`
	TextEntities []MessageEntity `json:"text_entities,omitempty"`
	Description  string          `json:"description"`
	Photo        []PhotoSize     `json:"photo"`
	Animation    *Animation      `json:"animation,omitempty"`
}

// Poll represents a poll. Field order matches Client.cpp JsonPoll::store (2729):
// id, question, question_entities, options, total_voter_count, [open_period,
// close_date] (emitted together when both != 0), is_closed, is_anonymous,
// allows_multiple_answers, allows_revoting, members_only, country_codes, type,
// [quiz: correct_option_id (only if exactly 1), correct_option_ids,
// explanation+entities, explanation_media], description+entities, media.
type Poll struct {
	ID                    string          `json:"id"`
	Question              string          `json:"question"`
	QuestionEntities      []MessageEntity `json:"question_entities,omitempty"`
	Options               []PollOption    `json:"options"`
	TotalVoterCount       int             `json:"total_voter_count"`
	OpenPeriod            int             `json:"open_period,omitempty"`
	CloseDate             int             `json:"close_date,omitempty"`
	IsClosed              bool            `json:"is_closed"`
	IsAnonymous           bool            `json:"is_anonymous"`
	AllowsMultipleAnswers bool            `json:"allows_multiple_answers"`
	AllowsRevoting        bool            `json:"allows_revoting"`
	MembersOnly           bool            `json:"members_only"`
	CountryCodes          []string        `json:"country_codes,omitempty"`
	Type                  string          `json:"type"`
	CorrectOptionID       int             `json:"correct_option_id,omitempty"`
	CorrectOptionIDs      []int           `json:"correct_option_ids,omitempty"`
	Explanation           string          `json:"explanation,omitempty"`
	ExplanationEntities   []MessageEntity `json:"explanation_entities,omitempty"`
	ExplanationMedia      *PollMedia      `json:"explanation_media,omitempty"`
	Description           string          `json:"description,omitempty"`
	DescriptionEntities   []MessageEntity `json:"description_entities,omitempty"`
	Media                 *PollMedia      `json:"media,omitempty"`
}

// Link is the poll-media link variant (reference JsonLink: {url}).
type Link struct {
	URL string `json:"url"`
}

// PollMedia is the media attached to a poll or quiz explanation. It is a
// discriminated union; exactly one variant pointer is set (reference
// JsonPollMedia, Client.cpp:2635). All variants are optional/omitempty.
type PollMedia struct {
	Animation *Animation `json:"animation,omitempty"`
	Audio     *Audio     `json:"audio,omitempty"`
	Document  *Document  `json:"document,omitempty"`
	Link      *Link      `json:"link,omitempty"`
	Location  *Location  `json:"location,omitempty"`
	Photo     *PhotoSize `json:"photo,omitempty"`
	Sticker   *Sticker   `json:"sticker,omitempty"`
	Venue     *Venue     `json:"venue,omitempty"`
	Video     *Video     `json:"video,omitempty"`
	LivePhoto *LivePhoto `json:"live_photo,omitempty"`
}

// PollOption represents an option in a poll. Field order matches Client.cpp
// JsonPollOption::store (2703): persistent_id, text, text_entities, media,
// voter_count, then (for user-added options) added_by_user/added_by_chat, addition_date.
type PollOption struct {
	PersistentID string          `json:"persistent_id"`
	Text         string          `json:"text"`
	TextEntities []MessageEntity `json:"text_entities,omitempty"`
	Media        *PollMedia      `json:"media,omitempty"`
	VoterCount   int             `json:"voter_count"`
	AddedByUser  *User           `json:"added_by_user,omitempty"`
	AddedByChat  *Chat           `json:"added_by_chat,omitempty"`
	AdditionDate int             `json:"addition_date,omitempty"`
}

// Venue represents a venue.
type Venue struct {
	Location        Location `json:"location"`
	Title           string   `json:"title"`
	Address         string   `json:"address"`
	FoursquareID    string   `json:"foursquare_id,omitempty"`
	FoursquareType  string   `json:"foursquare_type,omitempty"`
	GooglePlaceID   string   `json:"google_place_id,omitempty"`
	GooglePlaceType string   `json:"google_place_type,omitempty"`
}

// Location represents a point on the map. Coordinates use ForceFloat so whole
// numbers render with ".0" as the reference (TDLib) does.
//
// MarshalJSON governs output (the struct tags below are unused for marshal) and
// branches on whether the location is live (LivePeriod set), matching the two
// reference emitters:
//   - plain (JsonLocation::store, Client.cpp:1132): latitude, longitude, horizontal_accuracy?
//   - live  (JsonLiveLocation::store, Client.cpp:1105): latitude, longitude, live_period,
//     heading?, proximity_alert_radius?, horizontal_accuracy?
type Location struct {
	Latitude             ForceFloat `json:"latitude"`
	Longitude            ForceFloat `json:"longitude"`
	HorizontalAccuracy   ForceFloat `json:"horizontal_accuracy,omitempty"`
	LivePeriod           int        `json:"live_period,omitempty"`
	Heading              int        `json:"heading,omitempty"`
	ProximityAlertRadius int        `json:"proximity_alert_radius,omitempty"`
}

func (l Location) MarshalJSON() ([]byte, error) {
	if l.LivePeriod != 0 {
		return json.Marshal(struct {
			Latitude             ForceFloat `json:"latitude"`
			Longitude            ForceFloat `json:"longitude"`
			LivePeriod           int        `json:"live_period"`
			Heading              int        `json:"heading,omitempty"`
			ProximityAlertRadius int        `json:"proximity_alert_radius,omitempty"`
			HorizontalAccuracy   ForceFloat `json:"horizontal_accuracy,omitempty"`
		}{l.Latitude, l.Longitude, l.LivePeriod, l.Heading, l.ProximityAlertRadius, l.HorizontalAccuracy})
	}
	return json.Marshal(struct {
		Latitude           ForceFloat `json:"latitude"`
		Longitude          ForceFloat `json:"longitude"`
		HorizontalAccuracy ForceFloat `json:"horizontal_accuracy,omitempty"`
	}{l.Latitude, l.Longitude, l.HorizontalAccuracy})
}

// Sticker represents a sticker.
type Sticker struct {
	Width            int           `json:"width"`
	Height           int           `json:"height"`
	Emoji            string        `json:"emoji,omitempty"`
	SetName          string        `json:"set_name,omitempty"`
	IsAnimated       bool          `json:"is_animated"`
	IsVideo          bool          `json:"is_video"`
	Type             string        `json:"type"`
	PremiumAnimation *File         `json:"premium_animation,omitempty"`
	MaskPosition     *MaskPosition `json:"mask_position,omitempty"`
	CustomEmojiID    string        `json:"custom_emoji_id,omitempty"`
	NeedsRepainting  bool          `json:"needs_repainting,omitempty"`
	Thumbnail        *PhotoSize    `json:"thumbnail,omitempty"`
	Thumb            *PhotoSize    `json:"thumb,omitempty"`
	FileID           string        `json:"file_id"`
	FileUniqueID     string        `json:"file_unique_id"`
	FileSize         int           `json:"file_size,omitempty"`
}

// File represents a file ready to be downloaded.
type File struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size,omitempty"`
	FilePath     string `json:"file_path,omitempty"`
}

// MaskPosition describes the position on faces where a mask should be placed.
type MaskPosition struct {
	Point  string  `json:"point"`
	XShift float64 `json:"x_shift"`
	YShift float64 `json:"y_shift"`
	Scale  float64 `json:"scale"`
}

// PaidMediaInfo describes paid media.
type PaidMediaInfo struct {
	StarCount int         `json:"star_count"`
	PaidMedia []PaidMedia `json:"paid_media"`
}

// PaidMedia describes one paid media item. It is a discriminated union on Type;
// MarshalJSON emits the variant-specific fields in the official Client.cpp
// JsonPaidMedia::store (2450) order:
//   - "preview":     width?, height?, duration?
//   - "photo":       photo ([]PhotoSize)
//   - "live_photo":  live_photo
//   - "video":       video
//   - "other":       (nothing extra)
type PaidMedia struct {
	Type      string
	Width     int
	Height    int
	Duration  int
	Photo     []PhotoSize
	Video     *Video
	LivePhoto *LivePhoto
}

func (m PaidMedia) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case "preview":
		return json.Marshal(struct {
			Type     string `json:"type"`
			Width    int    `json:"width,omitempty"`
			Height   int    `json:"height,omitempty"`
			Duration int    `json:"duration,omitempty"`
		}{m.Type, m.Width, m.Height, m.Duration})
	case "photo":
		return json.Marshal(struct {
			Type  string      `json:"type"`
			Photo []PhotoSize `json:"photo"`
		}{m.Type, m.Photo})
	case "live_photo":
		return json.Marshal(struct {
			Type      string     `json:"type"`
			LivePhoto *LivePhoto `json:"live_photo"`
		}{m.Type, m.LivePhoto})
	case "video":
		return json.Marshal(struct {
			Type  string `json:"type"`
			Video *Video `json:"video"`
		}{m.Type, m.Video})
	default: // "other" / paidMediaUnsupported
		return json.Marshal(struct {
			Type string `json:"type"`
		}{m.Type})
	}
}

// MessageOrigin describes the origin of a forwarded message.
// Field order matches Client.cpp JsonMessageOrigin::store: type, then the
// variant-specific field(s), then author_signature (chat/channel), then date
// last (date is emitted after the switch for every variant).
type MessageOrigin struct {
	Type            string `json:"type"`
	SenderUser      *User  `json:"sender_user,omitempty"`
	SenderUserName  string `json:"sender_user_name,omitempty"`
	SenderChat      *Chat  `json:"sender_chat,omitempty"` // "chat" variant
	Chat            *Chat  `json:"chat,omitempty"`        // "channel" variant
	MessageID       int64  `json:"message_id,omitempty"`
	AuthorSignature string `json:"author_signature,omitempty"`
	Date            int64  `json:"date"`
}

// ExternalReplyInfo contains information about an external reply.
type ExternalReplyInfo struct {
	Origin             *MessageOrigin      `json:"origin"`
	Chat               *Chat               `json:"chat,omitempty"`
	MessageID          int64               `json:"message_id,omitempty"`
	LinkPreviewOptions *LinkPreviewOptions `json:"link_preview_options,omitempty"`
	Animation          *Animation          `json:"animation,omitempty"`
	Audio              *Audio              `json:"audio,omitempty"`
	Document           *Document           `json:"document,omitempty"`
	Photo              []PhotoSize         `json:"photo,omitempty"`
	Sticker            *Sticker            `json:"sticker,omitempty"`
	Story              *Story              `json:"story,omitempty"`
	Video              *Video              `json:"video,omitempty"`
	VideoNote          *VideoNote          `json:"video_note,omitempty"`
	Voice              *Voice              `json:"voice,omitempty"`
	HasMediaSpoiler    bool                `json:"has_media_spoiler,omitempty"`
}

// TextQuote contains information about a quoted part of a message.
type TextQuote struct {
	Text     string          `json:"text"`
	Entities []MessageEntity `json:"entities,omitempty"`
	Position int             `json:"position"`
	IsManual bool            `json:"is_manual"`
}

// Story represents a Telegram story.
type Story struct {
	Chat Chat  `json:"chat"`
	ID   int64 `json:"id"`
}

// ProximityAlertTriggered represents a proximity alert.
type ProximityAlertTriggered struct {
	Traveler *User `json:"traveler"`
	Watcher  *User `json:"watcher"`
	Distance int   `json:"distance"`
}

// ChatBoostAdded represents a service message about a chat boost.
type ChatBoostAdded struct {
	BoostCount int `json:"boost_count"`
}

// ChatBackground represents a chat background change (Client.cpp JsonChatBackground:3232:
// a single "type" object).
type ChatBackground struct {
	Type BackgroundType `json:"type"`
}

// BackgroundType is a discriminated union on Type. MarshalJSON emits the
// variant-specific fields in the official Client.cpp JsonBackgroundType::store
// (3165) order:
//   - "wallpaper":  document, dark_theme_dimming, is_blurred?, is_moving?
//   - "pattern":    document, fill, intensity, is_inverted?, is_moving?
//   - "fill":       fill, dark_theme_dimming
//   - "chat_theme": theme_name
type BackgroundType struct {
	Type             string
	Document         *Document
	DarkThemeDimming int
	IsBlurred        bool
	IsMoving         bool
	Fill             *BackgroundFill
	Intensity        int
	IsInverted       bool
	ThemeName        string
}

func (b BackgroundType) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case "wallpaper":
		return json.Marshal(struct {
			Type             string    `json:"type"`
			Document         *Document `json:"document"`
			DarkThemeDimming int       `json:"dark_theme_dimming"`
			IsBlurred        bool      `json:"is_blurred,omitempty"`
			IsMoving         bool      `json:"is_moving,omitempty"`
		}{b.Type, b.Document, b.DarkThemeDimming, b.IsBlurred, b.IsMoving})
	case "pattern":
		return json.Marshal(struct {
			Type       string          `json:"type"`
			Document   *Document       `json:"document"`
			Fill       *BackgroundFill `json:"fill"`
			Intensity  int             `json:"intensity"`
			IsInverted bool            `json:"is_inverted,omitempty"`
			IsMoving   bool            `json:"is_moving,omitempty"`
		}{b.Type, b.Document, b.Fill, b.Intensity, b.IsInverted, b.IsMoving})
	case "fill":
		return json.Marshal(struct {
			Type             string          `json:"type"`
			Fill             *BackgroundFill `json:"fill"`
			DarkThemeDimming int             `json:"dark_theme_dimming"`
		}{b.Type, b.Fill, b.DarkThemeDimming})
	case "chat_theme":
		return json.Marshal(struct {
			Type      string `json:"type"`
			ThemeName string `json:"theme_name"`
		}{b.Type, b.ThemeName})
	default:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{b.Type})
	}
}

// BackgroundFill is a discriminated union on Type (Client.cpp JsonBackgroundFill::store:3128):
//   - "solid":            color
//   - "gradient":         top_color, bottom_color, rotation_angle
//   - "freeform_gradient": colors
type BackgroundFill struct {
	Type          string
	Color         int
	TopColor      int
	BottomColor   int
	RotationAngle int
	Colors        []int
}

func (f BackgroundFill) MarshalJSON() ([]byte, error) {
	switch f.Type {
	case "solid":
		return json.Marshal(struct {
			Type  string `json:"type"`
			Color int    `json:"color"`
		}{f.Type, f.Color})
	case "gradient":
		return json.Marshal(struct {
			Type          string `json:"type"`
			TopColor      int    `json:"top_color"`
			BottomColor   int    `json:"bottom_color"`
			RotationAngle int    `json:"rotation_angle"`
		}{f.Type, f.TopColor, f.BottomColor, f.RotationAngle})
	case "freeform_gradient":
		return json.Marshal(struct {
			Type   string `json:"type"`
			Colors []int  `json:"colors"`
		}{f.Type, f.Colors})
	default:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{f.Type})
	}
}

// GiveawayCreated represents a service message about giveaway creation.
type GiveawayCreated struct {
	PrizeStarCount int `json:"prize_star_count,omitempty"`
}

// Giveaway represents a giveaway.
type Giveaway struct {
	Chats                         []Chat   `json:"chats"`
	WinnersSelectionDate          int64    `json:"winners_selection_date"`
	WinnerCount                   int      `json:"winner_count"`
	OnlyNewMembers                bool     `json:"only_new_members"`
	HasPublicWinners              bool     `json:"has_public_winners"`
	PrizeDescription              string   `json:"prize_description,omitempty"`
	CountryCodes                  []string `json:"country_codes,omitempty"`
	PrizeStarCount                int      `json:"prize_star_count,omitempty"`
	PremiumSubscriptionMonthCount int      `json:"premium_subscription_month_count,omitempty"`
}

// GiveawayWinners represents a giveaway with winners.
type GiveawayWinners struct {
	Chat                          Chat   `json:"chat"`
	GiveawayMessageID             int64  `json:"giveaway_message_id"`
	WinnersSelectionDate          int64  `json:"winners_selection_date"`
	WinnerCount                   int    `json:"winner_count"`
	Winners                       []User `json:"winners"`
	AdditionalChatCount           int    `json:"additional_chat_count,omitempty"`
	PrizeStarCount                int    `json:"prize_star_count,omitempty"`
	PremiumSubscriptionMonthCount int    `json:"premium_subscription_month_count,omitempty"`
	UnclaimedPrizeCount           int    `json:"unclaimed_prize_count,omitempty"`
	OnlyNewMembers                bool   `json:"only_new_members"`
	WasRefunded                   bool   `json:"was_refunded"`
	PrizeDescription              string `json:"prize_description,omitempty"`
}

// GiveawayCompleted represents a completed giveaway.
type GiveawayCompleted struct {
	WinnerCount         int      `json:"winner_count"`
	UnclaimedPrizeCount int      `json:"unclaimed_prize_count"`
	GiveawayMessage     *Message `json:"giveaway_message,omitempty"`
	IsStarGiveaway      bool     `json:"is_star_giveaway"`
}

// VideoChatScheduled represents a service message about video chat scheduling.
type VideoChatScheduled struct {
	StartDate int64 `json:"start_date"`
}

// VideoChatStarted represents a service message about video chat start.
type VideoChatStarted struct{}

// VideoChatEnded represents a service message about video chat end.
type VideoChatEnded struct {
	Duration int `json:"duration"`
}

// VideoChatParticipantsInvited represents a service message about video chat participants.
type VideoChatParticipantsInvited struct {
	Users []User `json:"users"`
}

// WebAppData contains data from a Web App.
type WebAppData struct {
	Data       string `json:"data"`
	ButtonText string `json:"button_text"`
}

// UsersShared contains information about shared users. Field order matches
// Client.cpp JsonUsersShared::store (3832): user_ids, users, request_id.
type UsersShared struct {
	UserIDs   []int64      `json:"user_ids"`
	Users     []SharedUser `json:"users"`
	RequestID int          `json:"request_id"`
}

// SharedUser is one user in a UsersShared payload (reference JsonSharedUser,
// Client.cpp:3805): user_id, then optional first_name/last_name/username/photo.
type SharedUser struct {
	UserID    int64       `json:"user_id"`
	FirstName string      `json:"first_name,omitempty"`
	LastName  string      `json:"last_name,omitempty"`
	Username  string      `json:"username,omitempty"`
	Photo     []PhotoSize `json:"photo,omitempty"`
}

// UserShared is the legacy single-user variant (reference JsonUserShared,
// Client.cpp:3793): user_id, request_id.
type UserShared struct {
	UserID    int64 `json:"user_id"`
	RequestID int   `json:"request_id"`
}

// ChatShared contains information about a shared chat. Field order matches
// Client.cpp JsonChatShared::store (3850): chat_id, title, username, photo, request_id.
type ChatShared struct {
	ChatID    int64       `json:"chat_id"`
	Title     string      `json:"title,omitempty"`
	Username  string      `json:"username,omitempty"`
	Photo     []PhotoSize `json:"photo,omitempty"`
	RequestID int         `json:"request_id"`
}

// WriteAccessAllowed represents a service message about write access being allowed.
type WriteAccessAllowed struct {
	FromRequest        bool   `json:"from_request"`
	WebAppName         string `json:"web_app_name,omitempty"`
	FromAttachmentMenu bool   `json:"from_attachment_menu"`
}

// PassportData contains Telegram Passport data.
type PassportData struct {
	Data        []EncryptedPassportElement `json:"data"`
	Credentials EncryptedCredentials       `json:"credentials"`
}

// EncryptedPassportElement contains encrypted Telegram Passport element data.
type EncryptedPassportElement struct {
	Type        string         `json:"type"`
	Data        string         `json:"data,omitempty"`
	PhoneNumber string         `json:"phone_number,omitempty"`
	Email       string         `json:"email,omitempty"`
	Files       []PassportFile `json:"files,omitempty"`
	FrontSide   *PassportFile  `json:"front_side,omitempty"`
	ReverseSide *PassportFile  `json:"reverse_side,omitempty"`
	Selfie      *PassportFile  `json:"selfie,omitempty"`
	Translation []PassportFile `json:"translation,omitempty"`
	Hash        string         `json:"hash"`
}

// PassportFile represents a file uploaded to Telegram Passport.
type PassportFile struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileSize     int    `json:"file_size"`
	FileDate     int    `json:"file_date"`
}

// EncryptedCredentials contains encrypted Telegram Passport credentials.
type EncryptedCredentials struct {
	Data   string `json:"data"`
	Hash   string `json:"hash"`
	Secret string `json:"secret"`
}

// Invoice contains basic information about an invoice.
type Invoice struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	StartParameter string `json:"start_parameter"`
	Currency       string `json:"currency"`
	TotalAmount    int64  `json:"total_amount"`
}

// SuccessfulPayment contains basic information about a successful payment.
type SuccessfulPayment struct {
	Currency                string     `json:"currency"`
	TotalAmount             int64      `json:"total_amount"`
	InvoicePayload          string     `json:"invoice_payload"`
	ShippingOptionID        string     `json:"shipping_option_id,omitempty"`
	OrderInfo               *OrderInfo `json:"order_info,omitempty"`
	TelegramPaymentChargeID string     `json:"telegram_payment_charge_id"`
	ProviderPaymentChargeID string     `json:"provider_payment_charge_id"`
}

// RefundedPayment contains basic information about a refunded payment.
type RefundedPayment struct {
	Currency                string `json:"currency"`
	TotalAmount             int64  `json:"total_amount"`
	InvoicePayload          string `json:"invoice_payload"`
	TelegramPaymentChargeID string `json:"telegram_payment_charge_id"`
	ProviderPaymentChargeID string `json:"provider_payment_charge_id,omitempty"`
}

// OrderInfo contains information about an order.
type OrderInfo struct {
	Name            string           `json:"name,omitempty"`
	PhoneNumber     string           `json:"phone_number,omitempty"`
	Email           string           `json:"email,omitempty"`
	ShippingAddress *ShippingAddress `json:"shipping_address,omitempty"`
}

// ShippingAddress represents a shipping address.
type ShippingAddress struct {
	CountryCode string `json:"country_code"`
	State       string `json:"state"`
	City        string `json:"city"`
	StreetLine1 string `json:"street_line1"`
	StreetLine2 string `json:"street_line2"`
	PostCode    string `json:"post_code"`
}

// WebhookInfo describes the current status of a webhook.
type WebhookInfo struct {
	URL                  string   `json:"url"`
	HasCustomCertificate bool     `json:"has_custom_certificate"`
	PendingUpdateCount   int      `json:"pending_update_count"`
	IPAddress            string   `json:"ip_address,omitempty"`
	LastErrorDate        int64    `json:"last_error_date,omitempty"`
	LastErrorMessage     string   `json:"last_error_message,omitempty"`
	MaxConnections       int      `json:"max_connections,omitempty"`
	AllowedUpdates       []string `json:"allowed_updates,omitempty"`
	LastSyncErrorDate    int64    `json:"last_synchronization_error_date,omitempty"`
}

// LivePhoto represents a live photo (photo + video).
type LivePhoto struct {
	Photo []PhotoSize `json:"photo"`
	Video *Video      `json:"video"`
}

// GiftInfo represents a gift message.
type GiftInfo struct {
	Gift             *GiftDetails `json:"gift"`
	UpgradeStarCount int          `json:"upgrade_star_count,omitempty"`
	OwnedGiftID      string       `json:"owned_gift_id,omitempty"`
}

// Gift represents a gift available to send (getAvailableGifts). Field order
// matches the reference Client.cpp JsonGift (1824-1859).
type Gift struct {
	ID                     string   `json:"id"`
	Sticker                *Sticker `json:"sticker"`
	StarCount              int64    `json:"star_count"`
	UpgradeStarCount       int64    `json:"upgrade_star_count,omitempty"`
	RemainingCount         int32    `json:"remaining_count,omitempty"`
	TotalCount             int32    `json:"total_count,omitempty"`
	PersonalRemainingCount int32    `json:"personal_remaining_count,omitempty"`
	PersonalTotalCount     int32    `json:"personal_total_count,omitempty"`
	PublisherChat          *Chat    `json:"publisher_chat,omitempty"`
	IsPremium              bool     `json:"is_premium,omitempty"`
	HasColors              bool     `json:"has_colors,omitempty"`
	UniqueGiftVariantCount int32    `json:"unique_gift_variant_count,omitempty"`
	Background             any      `json:"background,omitempty"`
}

// Gifts is the getAvailableGifts response envelope.
type Gifts struct {
	Gifts []Gift `json:"gifts"`
}

// GiftDetails represents gift details.
type GiftDetails struct {
	ID                   string   `json:"id"`
	Sticker              *Sticker `json:"sticker"`
	StarCount            int      `json:"star_count,omitempty"`
	DefaultSaleStarCount int      `json:"default_sale_star_count,omitempty"`
	UpgradeStarCount     int      `json:"upgrade_star_count,omitempty"`
	TotalCount           int      `json:"total_count,omitempty"`
	RemainingCount       int      `json:"remaining_count,omitempty"`
}

// UniqueGiftInfo represents an upgraded/unique gift message.
type UniqueGiftInfo struct {
	Gift              *UpgradedGift `json:"gift"`
	OwnedGiftID       string        `json:"owned_gift_id,omitempty"`
	TransferStarCount int           `json:"transfer_star_count,omitempty"`
}

// UpgradedGift represents an upgraded gift.
type UpgradedGift struct {
	Name             string               `json:"name"`
	UniqueNumber     int                  `json:"unique_number,omitempty"`
	UniqueTotalCount int                  `json:"unique_total_count,omitempty"`
	Backdrop         *GiftBackdrop        `json:"backdrop,omitempty"`
	Model            *GiftModel           `json:"model,omitempty"`
	OriginalDetails  *GiftOriginalDetails `json:"original_details,omitempty"`
	StarCount        int                  `json:"star_count,omitempty"`
}

// GiftBackdrop represents a gift backdrop.
type GiftBackdrop struct {
	Name           string `json:"name"`
	CenterColorID  int    `json:"center_color_id"`
	EdgeColorID    int    `json:"edge_color_id"`
	PatternColorID int    `json:"pattern_color_id"`
	TextColorID    int    `json:"text_color_id"`
}

// GiftModel represents a gift model.
type GiftModel struct {
	Name           string   `json:"name"`
	Sticker        *Sticker `json:"sticker"`
	RarityPerMille int      `json:"rarity_per_mille,omitempty"`
}

// GiftOriginalDetails represents original gift details.
type GiftOriginalDetails struct {
	SenderChatID   int64           `json:"sender_chat_id,omitempty"`
	ReceiverChatID int64           `json:"receiver_chat_id,omitempty"`
	Date           int64           `json:"date"`
	Text           string          `json:"text,omitempty"`
	Entities       []MessageEntity `json:"entities,omitempty"`
}

// RichMessage is defined in richmessage.go (output shape {blocks, is_rtl}).

// Checklist represents a checklist.
type Checklist struct {
	Title                    string          `json:"title"`
	Tasks                    []ChecklistTask `json:"tasks"`
	OthersCanAddTasks        bool            `json:"others_can_add_tasks"`
	OthersCanMarkTasksAsDone bool            `json:"others_can_mark_tasks_as_done"`
}

// ChecklistTask represents a task in a checklist.
type ChecklistTask struct {
	ID              int             `json:"id"`
	Text            string          `json:"text"`
	TextEntities    []MessageEntity `json:"text_entities,omitempty"`
	CompletedByUser *User           `json:"completed_by_user,omitempty"`
	CompletedByChat *Chat           `json:"completed_by_chat,omitempty"`
}

// ChecklistTasksDone represents completed checklist tasks.
type ChecklistTasksDone struct {
	ChecklistMessageID int64           `json:"checklist_message_id"`
	CompletedTasks     []ChecklistTask `json:"completed_tasks"`
}

// ChecklistTasksAdded represents added checklist tasks.
type ChecklistTasksAdded struct {
	ChecklistMessageID int64           `json:"checklist_message_id"`
	Tasks              []ChecklistTask `json:"tasks"`
}

// SuggestedPostInfo represents suggested post metadata.
type SuggestedPostInfo struct {
	State    string              `json:"state"`
	Date     int64               `json:"date,omitempty"`
	SendDate int64               `json:"send_date,omitempty"`
	Price    *SuggestedPostPrice `json:"price,omitempty"`
}

// SuggestedPostPrice represents a suggested post price.
type SuggestedPostPrice struct {
	StarCount int64 `json:"star_count,omitempty"`
	Toncoin   int64 `json:"toncoin,omitempty"`
}

// SuggestedPostApproved represents a suggested post approval.
type SuggestedPostApproved struct {
	SuggestedPostMessageID int64 `json:"suggested_post_message_id"`
	SendDate               int64 `json:"send_date,omitempty"`
}

// SuggestedPostDeclined represents a suggested post decline.
type SuggestedPostDeclined struct {
	SuggestedPostMessageID int64  `json:"suggested_post_message_id"`
	Comment                string `json:"comment,omitempty"`
}

// SuggestedPostPaid represents a suggested post payment.
type SuggestedPostPaid struct {
	SuggestedPostMessageID int64 `json:"suggested_post_message_id"`
	StarAmount             int64 `json:"star_amount"`
}

// SuggestedPostRefunded represents a suggested post refund.
type SuggestedPostRefunded struct {
	SuggestedPostMessageID int64 `json:"suggested_post_message_id"`
}

// SuggestedPostApprovalFailed represents a suggested post approval failure.
type SuggestedPostApprovalFailed struct {
	SuggestedPostMessageID int64 `json:"suggested_post_message_id"`
}

// PollOptionAdded represents a poll option addition.
type PollOptionAdded struct {
	PollMessageID   int64       `json:"poll_message_id"`
	Option          *PollOption `json:"option"`
	TotalVoterCount int         `json:"total_voter_count,omitempty"`
}

// PollOptionDeleted represents a poll option deletion.
type PollOptionDeleted struct {
	PollMessageID   int64 `json:"poll_message_id"`
	OptionID        int   `json:"option_id"`
	TotalVoterCount int   `json:"total_voter_count,omitempty"`
}

// ManagedBotCreated represents a managed bot creation message.
type ManagedBotCreated struct {
	Bot *User `json:"bot"`
}

// ChatOwnerLeft represents a chat owner leaving.
type ChatOwnerLeft struct {
	Owner     *User `json:"owner,omitempty"`
	OwnerChat *Chat `json:"owner_chat,omitempty"`
}

// ChatOwnerChanged represents a chat owner change.
type ChatOwnerChanged struct {
	PreviousOwner     *User `json:"previous_owner,omitempty"`
	PreviousOwnerChat *Chat `json:"previous_owner_chat,omitempty"`
	NewOwner          *User `json:"new_owner,omitempty"`
	NewOwnerChat      *Chat `json:"new_owner_chat,omitempty"`
}

// DirectMessagePriceChanged represents a direct message price change.
type DirectMessagePriceChanged struct {
	AreDirectMessagesEnabled bool  `json:"are_direct_messages_enabled"`
	StarCount                int64 `json:"star_count,omitempty"`
}

// PaidMessagePriceChanged represents a paid message price change.
type PaidMessagePriceChanged struct {
	PaidMessageStarCount int64 `json:"paid_message_star_count"`
}
