package types

// ChatType enumerates chat kinds.
type ChatType string

const (
	ChatTypePrivate    ChatType = "private"
	ChatTypeGroup      ChatType = "group"
	ChatTypeSupergroup ChatType = "supergroup"
	ChatTypeChannel    ChatType = "channel"
)

// Chat represents the minimal Bot API "Chat" object. Extended fields live in
// ChatFullInfo (returned by getChat). Chat is returned inside Message/Update.
//
// Field order matches the official Bot API response.
type Chat struct {
	ID        int64    `json:"id"`
	FirstName string   `json:"first_name,omitempty"`
	LastName  string   `json:"last_name,omitempty"`
	Username  string   `json:"username,omitempty"`
	Title     string   `json:"title,omitempty"`
	Type      ChatType `json:"type"`
	IsForum   bool     `json:"is_forum,omitempty"`
}

// ChatFullInfo is the full chat description returned by getChat.
// Embeds Chat and adds extended fields.
//
// omitempty rule: the official telegram-bot-api reference emits most optional
// booleans only when they are true (via `if (x) object("x", JsonTrue())`).
// To match byte-for-byte, such bools carry `,omitempty` here. Bools that the
// reference always emits (none in ChatFullInfo) must stay non-omitempty.
// See specs/2-type-fidelity-audit/audit-report.md §2 for the field-by-field
// reference mapping (Client.cpp JsonChat::store lines 1518-1807).
type ChatFullInfo struct {
	Chat
	// Private-chat-only fields (Client.cpp 1541-1591). Populated by the
	// users.getFullUser path (T226); zero/empty for groups/channels.
	CanSendGift                    bool                  `json:"can_send_gift,omitempty"`
	ActiveUsernames                []string              `json:"active_usernames,omitempty"`
	HasPrivateForwards             bool                  `json:"has_private_forwards,omitempty"`
	HasRestrictedVoiceAndVideo     bool                  `json:"has_restricted_voice_and_video_messages,omitempty"`
	Bio                            string                `json:"bio,omitempty"`
	BusinessIntro                  *BusinessIntro        `json:"business_intro,omitempty"`
	BusinessLocation               *BusinessLocation     `json:"business_location,omitempty"`
	BusinessOpeningHours           *BusinessOpeningHours `json:"business_opening_hours,omitempty"`
	AcceptedGiftTypes              *AcceptedGiftTypes    `json:"accepted_gift_types,omitempty"`
	Birthdate                      *Birthdate            `json:"birthdate,omitempty"`
	PersonalChat                   *Chat                 `json:"personal_chat,omitempty"`
	Rating                         *ChatRating           `json:"rating,omitempty"`
	FirstProfileAudio              *Audio                `json:"first_profile_audio,omitempty"`
	PaidMessageStarCount           int32                 `json:"paid_message_star_count,omitempty"`
	Photo                          *ChatPhoto            `json:"photo,omitempty"`
	MaxReactionCount               int32                 `json:"max_reaction_count"`
	AccentColorID                  int32                 `json:"accent_color_id"`
	BackgroundCustomEmojiID        string                `json:"background_custom_emoji_id,omitempty"`
	UniqueGiftColors               *UniqueGiftColors     `json:"unique_gift_colors,omitempty"`
	ProfileAccentColorID           int32                 `json:"profile_accent_color_id,omitempty"`
	ProfileBackgroundCustomEmojiID string                `json:"profile_background_custom_emoji_id,omitempty"`
	EmojiStatusCustomEmojiID       string                `json:"emoji_status_custom_emoji_id,omitempty"`
	EmojiStatusExpirationDate      int64                 `json:"emoji_status_expiration_date,omitempty"`
	AvailableReactions             []ReactionType        `json:"available_reactions,omitempty"`
	// Supergroup/channel flags (Client.cpp 1646-1721). Emitted only when true.
	JoinToSendMessages           bool  `json:"join_to_send_messages,omitempty"`
	JoinByRequest                bool  `json:"join_by_request,omitempty"`
	HasAggressiveAntiSpamEnabled bool  `json:"has_aggressive_anti_spam_enabled,omitempty"`
	HasHiddenMembers             bool  `json:"has_hidden_members,omitempty"`
	HasProtectedContent          bool  `json:"has_protected_content,omitempty"`
	HasVisibleHistory            bool  `json:"has_visible_history,omitempty"`
	IsDirectMessages             bool  `json:"is_direct_messages,omitempty"`
	CanSendPaidMedia             bool  `json:"can_send_paid_media,omitempty"`
	CanSetStickerSet             bool  `json:"can_set_sticker_set,omitempty"`
	ParentChat                   *Chat `json:"parent_chat,omitempty"`
	GuardBot                     *User `json:"guard_bot,omitempty"`
	// Group-only (Client.cpp 1612-1620). *bool so groups emit it unconditionally
	// (including false) while other chat types omit it (nil).
	AllMembersAreAdministrators *bool `json:"all_members_are_administrators,omitempty"`
	// Common extended fields (Client.cpp 1729-1799, is_full block).
	Description               string           `json:"description,omitempty"`
	InviteLink                string           `json:"invite_link,omitempty"`
	PinnedMessage             *Message         `json:"pinned_message,omitempty"`
	Permissions               *ChatPermissions `json:"permissions,omitempty"`
	SlowModeDelay             int32            `json:"slow_mode_delay,omitempty"`
	UnrestrictBoostCount      int32            `json:"unrestrict_boost_count,omitempty"`
	MessageAutoDeleteTime     int32            `json:"message_auto_delete_time,omitempty"`
	StickerSetName            string           `json:"sticker_set_name,omitempty"`
	CustomEmojiStickerSetName string           `json:"custom_emoji_sticker_set_name,omitempty"`
	LinkedChatID              int64            `json:"linked_chat_id,omitempty"`
	Location                  *ChatLocation    `json:"location,omitempty"`
}

// ChatPhoto represents a chat profile photo.
type ChatPhoto struct {
	SmallFileID       string `json:"small_file_id"`
	SmallFileUniqueID string `json:"small_file_unique_id"`
	BigFileID         string `json:"big_file_id"`
	BigFileUniqueID   string `json:"big_file_unique_id"`
}

// ChatPermissions represents actions a non-administrator user is allowed to
// take in a chat. All 17 fields are always emitted by the reference
// (Client.cpp json_store_permissions lines 17492-17513).
type ChatPermissions struct {
	CanSendMessages       bool `json:"can_send_messages"`
	CanSendMediaMessages  bool `json:"can_send_media_messages"`
	CanSendAudios         bool `json:"can_send_audios"`
	CanSendDocuments      bool `json:"can_send_documents"`
	CanSendPhotos         bool `json:"can_send_photos"`
	CanSendVideos         bool `json:"can_send_videos"`
	CanSendVideoNotes     bool `json:"can_send_video_notes"`
	CanSendVoiceNotes     bool `json:"can_send_voice_notes"`
	CanSendPolls          bool `json:"can_send_polls"`
	CanSendOtherMessages  bool `json:"can_send_other_messages"`
	CanAddWebPagePreviews bool `json:"can_add_web_page_previews"`
	CanReactToMessages    bool `json:"can_react_to_messages"`
	CanEditTag            bool `json:"can_edit_tag"`
	CanChangeInfo         bool `json:"can_change_info"`
	CanInviteUsers        bool `json:"can_invite_users"`
	CanPinMessages        bool `json:"can_pin_messages"`
	CanManageTopics       bool `json:"can_manage_topics"`
}

// ChatAdministratorRights represents the rights of an administrator in a chat.
type ChatAdministratorRights struct {
	CanManageChat           bool `json:"can_manage_chat"`
	CanDeleteMessages       bool `json:"can_delete_messages"`
	CanManageVideoChats     bool `json:"can_manage_video_chats"`
	CanRestrictMembers      bool `json:"can_restrict_members"`
	CanPromoteMembers       bool `json:"can_promote_members"`
	CanChangeInfo           bool `json:"can_change_info"`
	CanInviteUsers          bool `json:"can_invite_users"`
	CanPostMessages         bool `json:"can_post_messages"`
	CanEditMessages         bool `json:"can_edit_messages"`
	CanPinMessages          bool `json:"can_pin_messages"`
	CanPostStories          bool `json:"can_post_stories"`
	CanEditStories          bool `json:"can_edit_stories"`
	CanDeleteStories        bool `json:"can_delete_stories"`
	IsAnonymous             bool `json:"is_anonymous"`
	CanManageTopics         bool `json:"can_manage_topics"`
	CanManageDirectMessages bool `json:"can_manage_direct_messages"`
	CanManageTags           bool `json:"can_manage_tags"`
}

// ChatMember represents a user in a chat.
//
// P0 header order matches Client.cpp JsonChatMember::store (5712): user first,
// then status. The full reference is a per-status union (creator/admin/member/
// restricted/banned each emit different fields, with custom_title vs tag, nested
// rights/permissions blocks emitted unconditionally and chat-type-gated — see
// json_store_administrator_rights 17461 and json_store_permissions 17492). A
// fully faithful version needs a chat-type-aware custom MarshalJSON; tracked as
// a follow-up. The fixed struct order below gets the header right.
type ChatMember struct {
	User                    *User  `json:"user"`
	Status                  string `json:"status"`
	CustomTitle             string `json:"custom_title,omitempty"`
	IsAnonymous             bool   `json:"is_anonymous,omitempty"`
	CanBeEdited             bool   `json:"can_be_edited,omitempty"`
	CanManageChat           bool   `json:"can_manage_chat,omitempty"`
	CanChangeInfo           bool   `json:"can_change_info,omitempty"`
	CanPostMessages         bool   `json:"can_post_messages,omitempty"`
	CanEditMessages         bool   `json:"can_edit_messages,omitempty"`
	CanDeleteMessages       bool   `json:"can_delete_messages,omitempty"`
	CanInviteUsers          bool   `json:"can_invite_users,omitempty"`
	CanRestrictMembers      bool   `json:"can_restrict_members,omitempty"`
	CanPinMessages          bool   `json:"can_pin_messages,omitempty"`
	CanManageTopics         bool   `json:"can_manage_topics,omitempty"`
	CanPromoteMembers       bool   `json:"can_promote_members,omitempty"`
	CanManageVideoChats     bool   `json:"can_manage_video_chats,omitempty"`
	CanManageVoiceChats     bool   `json:"can_manage_voice_chats,omitempty"`
	CanPostStories          bool   `json:"can_post_stories,omitempty"`
	CanEditStories          bool   `json:"can_edit_stories,omitempty"`
	CanDeleteStories        bool   `json:"can_delete_stories,omitempty"`
	CanManageDirectMessages bool   `json:"can_manage_direct_messages,omitempty"`
	CanManageTags           bool   `json:"can_manage_tags,omitempty"`
	IsMember                bool   `json:"is_member,omitempty"`
	CanSendMessages         bool   `json:"can_send_messages,omitempty"`
	CanSendAudios           bool   `json:"can_send_audios,omitempty"`
	CanSendDocuments        bool   `json:"can_send_documents,omitempty"`
	CanSendPhotos           bool   `json:"can_send_photos,omitempty"`
	CanSendVideos           bool   `json:"can_send_videos,omitempty"`
	CanSendVideoNotes       bool   `json:"can_send_video_notes,omitempty"`
	CanSendVoiceNotes       bool   `json:"can_send_voice_notes,omitempty"`
	CanSendPolls            bool   `json:"can_send_polls,omitempty"`
	CanSendOtherMessages    bool   `json:"can_send_other_messages,omitempty"`
	CanAddWebPagePreviews   bool   `json:"can_add_web_page_previews,omitempty"`
	UntilDate               int64  `json:"until_date,omitempty"`
	Tag                     string `json:"tag,omitempty"`
	CanSendMediaMessages    bool   `json:"can_send_media_messages,omitempty"`
	CanReactToMessages      bool   `json:"can_react_to_messages,omitempty"`
	CanEditTag              bool   `json:"can_edit_tag,omitempty"`

	// chatType governs admin-rights/permissions gating in MarshalJSON
	// (json_store_administrator_rights/permissions are chat-type-dependent).
	// Set via SetChatType; not serialized directly.
	chatType ChatType
}

// SetChatType records the chat kind ("private"/"group"/"supergroup"/"channel")
// so MarshalJSON can emit the chat-type-gated rights/permissions the reference
// emits (json_store_administrator_rights, Client.cpp:17461). Producers that know
// the chat type should call this before marshaling.
func (m *ChatMember) SetChatType(ct ChatType) { m.chatType = ct }

// ChatMemberOwner represents a chat member with owner status.
type ChatMemberOwner = ChatMember

// ChatMemberAdministrator represents a chat member with admin rights.
type ChatMemberAdministrator = ChatMember

// ChatMemberMember represents a regular chat member.
type ChatMemberMember = ChatMember

// ChatMemberRestricted represents a restricted chat member.
type ChatMemberRestricted = ChatMember

// ChatMemberLeft represents a user who left the chat.
type ChatMemberLeft = ChatMember

// ChatMemberBanned represents a banned user.
type ChatMemberBanned = ChatMember

// ChatLocation represents a location to which a chat is connected.
type ChatLocation struct {
	Location *Location `json:"location"`
	Address  string    `json:"address"`
}

// ReactionType represents a type of reaction.
type ReactionType struct {
	Type          string `json:"type"`
	Emoji         string `json:"emoji,omitempty"`
	CustomEmojiID string `json:"custom_emoji_id,omitempty"`
}

// Birthdate represents a birthdate.
type Birthdate struct {
	Day   int32 `json:"day"`
	Month int32 `json:"month"`
	Year  int32 `json:"year,omitempty"`
}

// BusinessIntro represents a business intro.
type BusinessIntro struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Sticker     *Sticker `json:"sticker,omitempty"`
}

// BusinessLocation represents a business location.
type BusinessLocation struct {
	Location *Location `json:"location,omitempty"`
	Address  string    `json:"address"`
}

// BusinessOpeningHours represents business opening hours.
type BusinessOpeningHours struct {
	TimeZoneName string                         `json:"time_zone_name"`
	OpeningHours []BusinessOpeningHoursInterval `json:"opening_hours"`
}

// BusinessOpeningHoursInterval represents an opening hours interval.
type BusinessOpeningHoursInterval struct {
	OpeningMinute int32 `json:"opening_minute"`
	ClosingMinute int32 `json:"closing_minute"`
}

// AcceptedGiftTypes describes types of gifts accepted by a user.
// Reference: Client.cpp JsonAcceptedGiftTypes lines 1188-1205. All five bools
// are always emitted (non-omitempty).
type AcceptedGiftTypes struct {
	UnlimitedGifts      bool `json:"unlimited_gifts"`
	LimitedGifts        bool `json:"limited_gifts"`
	UniqueGifts         bool `json:"unique_gifts"`
	PremiumSubscription bool `json:"premium_subscription"`
	GiftsFromChannels   bool `json:"gifts_from_channels"`
}

// ChatRating represents a user's rating (private-chat getChat).
// Reference: Client.cpp JsonUserRating lines 1400-1416. level, rating and
// current_level_rating are always emitted; next_level_rating only when the
// maximum level has NOT been reached.
type ChatRating struct {
	Level              int32 `json:"level"`
	Rating             int64 `json:"rating"`
	CurrentLevelRating int64 `json:"current_level_rating"`
	NextLevelRating    int64 `json:"next_level_rating,omitempty"`
}

// UniqueGiftColors describes the custom emoji colors of upgraded gifts.
// Reference: Client.cpp JsonUniqueGiftColors lines 1418-1432.
type UniqueGiftColors struct {
	ModelCustomEmojiID    string  `json:"model_custom_emoji_id"`
	SymbolCustomEmojiID   string  `json:"symbol_custom_emoji_id"`
	LightThemeMainColor   int32   `json:"light_theme_main_color"`
	LightThemeOtherColors []int32 `json:"light_theme_other_colors"`
	DarkThemeMainColor    int32   `json:"dark_theme_main_color"`
	DarkThemeOtherColors  []int32 `json:"dark_theme_other_colors"`
}
