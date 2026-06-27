package convert

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// Chat converts a tg.Chat or tg.Channel (from a Message) into a minimal
// types.Chat suitable for embedding in Message responses.
func Chat(peer tg.PeerClass) *apitypes.Chat {
	if peer == nil {
		return nil
	}
	switch p := peer.(type) {
	case *tg.PeerUser:
		return &apitypes.Chat{ID: p.UserID, Type: apitypes.ChatTypePrivate}
	case *tg.PeerChat:
		return &apitypes.Chat{ID: p.ChatID, Type: apitypes.ChatTypeGroup}
	case *tg.PeerChannel:
		return &apitypes.Chat{ID: -1000000000000 - p.ChannelID, Type: apitypes.ChatTypeSupergroup}
	default:
		return nil
	}
}

// ChatFullInfoFromChatFull converts a tg.ChatFull (basic group) into the Bot
// API ChatFullInfo response.
//
// Reference: Client.cpp JsonChat::store lines 1595-1623 (group case) +
// 1729-1799 (common is_full block).
func ChatFullInfoFromChatFull(cf *tg.ChatFull, chat *tg.Chat) *apitypes.ChatFullInfo {
	if cf == nil {
		return nil
	}
	var chatID int64
	if chat != nil {
		chatID = -chat.ID
	}
	info := &apitypes.ChatFullInfo{
		Chat: apitypes.Chat{
			ID:   chatID,
			Type: apitypes.ChatTypeGroup,
		},
		Description: cf.About,
		// Reference always emits accepted_gift_types for groups (line 1620),
		// all five bools false.
		AcceptedGiftTypes: &apitypes.AcceptedGiftTypes{},
	}
	if chat != nil {
		info.Title = chat.Title
		info.HasProtectedContent = chat.Noforwards
		if chat.DefaultBannedRights != nil {
			info.Permissions = BannedRightsToPermissions(chat.DefaultBannedRights)
			// all_members_are_administrators is emitted unconditionally for basic
			// groups (including false); *bool keeps it omitted for other types.
			allAdmins := permissionsAreFullyOpen(info.Permissions)
			info.AllMembersAreAdministrators = &allAdmins
		}
	}
	// Basic groups have no custom accent color; TDLib computes default as
	// chat_id % 7. Max_reaction_count comes from ChatFull.ReactionsLimit.
	info.AvailableReactions = chatReactionsToBotAPI(cf.AvailableReactions)
	info.MaxReactionCount = cf.ReactionsLimit
	if chat != nil {
		info.AccentColorID = int32(chat.ID % 7)
	}
	// Chat photo (basic groups carry ChatPhoto on ChatFull).
	// Basic groups (PeerChat) have no access_hash — pass 0.
	if cf.ChatPhoto != nil {
		if photo, ok := cf.ChatPhoto.(*tg.Photo); ok {
			info.Photo = photoToChatPhoto(photo, chatID, 0)
		}
	}
	// Invite link from the exported invite.
	if cf.ExportedInvite != nil {
		if link, ok := cf.ExportedInvite.(*tg.ChatInviteExported); ok {
			info.InviteLink = link.Link
		}
	}
	if cf.PinnedMsgID != 0 {
		info.PinnedMessage = &apitypes.Message{MessageID: int64(cf.PinnedMsgID)}
	}
	if cf.TTLPeriod != 0 {
		info.MessageAutoDeleteTime = cf.TTLPeriod
	}
	return info
}

// permissionsAreFullyOpen reports whether every permission in the Bot API
// ChatPermissions struct is true. Used to compute
// all_members_are_administrators for basic groups (Client.cpp lines
// 1612-1619).
func permissionsAreFullyOpen(p *apitypes.ChatPermissions) bool {
	if p == nil {
		return false
	}
	return p.CanSendMessages && p.CanSendAudios && p.CanSendDocuments &&
		p.CanSendPhotos && p.CanSendVideos && p.CanSendVideoNotes &&
		p.CanSendVoiceNotes && p.CanSendPolls && p.CanSendOtherMessages &&
		p.CanAddWebPagePreviews && p.CanReactToMessages && p.CanEditTag &&
		p.CanChangeInfo && p.CanInviteUsers && p.CanPinMessages
}

// ChatFullInfoFromChannelFull converts a tg.ChannelFull + tg.Channel into the
// Bot API ChatFullInfo response for supergroups/channels.
//
// Reference: Client.cpp JsonChat::store lines 1624-1724 (supergroup case) +
// 1729-1799 (common is_full block).
func ChatFullInfoFromChannelFull(cf *tg.ChannelFull, ch *tg.Channel) *apitypes.ChatFullInfo {
	if cf == nil || ch == nil {
		return nil
	}

	chatID := -(1000000000000 + ch.ID)
	chatType := apitypes.ChatTypeChannel
	isSupergroup := ch.Megagroup
	if isSupergroup {
		chatType = apitypes.ChatTypeSupergroup
	}

	info := &apitypes.ChatFullInfo{
		Chat: apitypes.Chat{
			ID:       chatID,
			Type:     chatType,
			Title:    ch.Title,
			Username: ch.Username,
			IsForum:  ch.Forum,
		},
		Description:         cf.About,
		SlowModeDelay:       cf.SlowmodeSeconds,
		HasProtectedContent: ch.Noforwards,
		HasVisibleHistory:   !cf.HiddenPrehistory,
	}

	// Flags the reference only emits when true. Set them unconditionally here;
	// the omitempty tags drop the false ones from the JSON.
	info.JoinToSendMessages = isSupergroup && ch.JoinToSend
	info.JoinByRequest = isSupergroup && ch.JoinRequest
	info.HasHiddenMembers = isSupergroup && cf.ParticipantsHidden
	info.HasAggressiveAntiSpamEnabled = cf.Antispam
	info.CanSetStickerSet = cf.CanSetStickers
	// is_direct_messages + parent_chat are set below from channel flags.

	// Accent + profile colors. Source: tg.Channel.Color / .ProfileColor
	// (*tg.PeerColor{Color, BackgroundEmojiID}). NOTE: ch.Level is the chat
	// boost level, NOT the accent color — the previous mapping was a bug.
	// Fallback: TDLib computes default as channel_id % 7.
	if pc, ok := ch.Color.(*tg.PeerColor); ok && pc != nil {
		info.AccentColorID = pc.Color
		info.BackgroundCustomEmojiID = strconv.FormatInt(pc.BackgroundEmojiID, 10)
	} else {
		info.AccentColorID = int32(ch.ID % 7)
	}
	if pp, ok := ch.ProfileColor.(*tg.PeerColor); ok && pp != nil && pp.Color != 0 {
		info.ProfileAccentColorID = pp.Color
		info.ProfileBackgroundCustomEmojiID = strconv.FormatInt(pp.BackgroundEmojiID, 10)
	}

	// Emoji status (*tg.EmojiStatus{DocumentID, Until}).
	if es, ok := ch.EmojiStatus.(*tg.EmojiStatus); ok && es != nil && es.DocumentID != 0 {
		info.EmojiStatusCustomEmojiID = strconv.FormatInt(es.DocumentID, 10)
		if es.Until != 0 {
			info.EmojiStatusExpirationDate = int64(es.Until)
		}
	}

	// Max reaction count comes from ChannelFull.ReactionsLimit.
	info.MaxReactionCount = cf.ReactionsLimit

	// Paid media: only for channels (not supergroups), per Client.cpp 1711.
	if cf.PaidMediaAllowed && !isSupergroup {
		info.CanSendPaidMedia = true
	}

	// Paid messages in stars.
	if cf.SendPaidMessagesStars > 0 {
		info.PaidMessageStarCount = int32(cf.SendPaidMessagesStars)
	}

	// Guard bot (minimal user stub; full User would need a users.getUsers call).
	if cf.GuardBotID != 0 {
		info.GuardBot = &apitypes.User{ID: cf.GuardBotID, IsBot: true}
	}

	// AcceptedGiftTypes is ALWAYS emitted by the reference (line 1714-1717).
	// For supergroups/channels all 5 bools = can_send_gift (we have no direct
	// source field, so default to false until a tg source is confirmed).
	info.AcceptedGiftTypes = &apitypes.AcceptedGiftTypes{}

	// Chat photo.
	if cf.ChatPhoto != nil {
		if photo, ok := cf.ChatPhoto.(*tg.Photo); ok {
			info.Photo = photoToChatPhoto(photo, chatID, ch.AccessHash)
		}
	}

	// Active usernames (active only).
	if len(ch.Usernames) > 0 {
		for _, u := range ch.Usernames {
			if u.Active {
				info.ActiveUsernames = append(info.ActiveUsernames, u.Username)
			}
		}
	}

	// Default permissions (supergroups only per Client.cpp 1678-1679).
	if isSupergroup && ch.DefaultBannedRights != nil {
		info.Permissions = BannedRightsToPermissions(ch.DefaultBannedRights)
	}

	// Linked chat.
	if cf.LinkedChatID != 0 {
		info.LinkedChatID = cf.LinkedChatID
	}

	// Sticker set.
	if cf.Stickerset != nil {
		if ss, ok := cf.Stickerset.(*tg.StickerSet); ok && ss.ShortName != "" {
			info.StickerSetName = ss.ShortName
		}
	}
	// Custom emoji sticker set (ChannelFull.Emojiset).
	if cf.Emojiset != nil {
		if es, ok := cf.Emojiset.(*tg.StickerSet); ok && es.ShortName != "" {
			info.CustomEmojiStickerSetName = es.ShortName
		}
	}

	// Available reactions.
	info.AvailableReactions = chatReactionsToBotAPI(cf.AvailableReactions)

	// Pinned message.
	if cf.PinnedMsgID != 0 {
		info.PinnedMessage = &apitypes.Message{MessageID: int64(cf.PinnedMsgID)}
	}

	// TTL period (message_auto_delete_time).
	if cf.TTLPeriod != 0 {
		info.MessageAutoDeleteTime = cf.TTLPeriod
	}

	// Boosts.
	if cf.BoostsUnrestrict != 0 {
		info.UnrestrictBoostCount = cf.BoostsUnrestrict
	}

	// Location.
	if cf.Location != nil {
		info.Location = channelLocationToBotAPI(cf.Location)
	}

	return info
}

// chatReactionsToBotAPI converts a tg.ChatReactionsClass (the
// available_reactions field on ChannelFull/ChatFull) into a Bot API
// []ReactionType. Returns nil when no reactions are configured (the JSON tag
// is omitempty so nil drops the field, matching the reference's null check).
func chatReactionsToBotAPI(r tg.ChatReactionsClass) []apitypes.ReactionType {
	if r == nil {
		return nil
	}
	switch v := r.(type) {
	case *tg.ChatReactionsNone:
		return nil
	case *tg.ChatReactionsAll:
		// All reactions allowed — the reference would emit every available
		// reaction type. We can't enumerate them here without an extra RPC,
		// so return nil (field omitted) rather than a misleading subset.
		return nil
	case *tg.ChatReactionsSome:
		var out []apitypes.ReactionType
		for _, reaction := range v.Reactions {
			switch rt := reaction.(type) {
			case *tg.ReactionEmoji:
				out = append(out, apitypes.ReactionType{Type: "emoji", Emoji: rt.Emoticon})
			case *tg.ReactionCustomEmoji:
				out = append(out, apitypes.ReactionType{Type: "custom_emoji", CustomEmojiID: strconv.FormatInt(rt.DocumentID, 10)})
			}
		}
		return out
	}
	return nil
}

// channelLocationToBotAPI converts a tg.ChannelLocationClass to the Bot API
// ChatLocation pointer.
func channelLocationToBotAPI(loc tg.ChannelLocationClass) *apitypes.ChatLocation {
	if loc == nil {
		return nil
	}
	switch v := loc.(type) {
	case *tg.ChannelLocation:
		if v.GeoPoint == nil {
			return &apitypes.ChatLocation{Address: v.Address}
		}
		if gp, ok := v.GeoPoint.(*tg.GeoPoint); ok && gp != nil {
			gloc := geoLocation(gp)
			return &apitypes.ChatLocation{
				Address:  v.Address,
				Location: &gloc,
			}
		}
		return &apitypes.ChatLocation{Address: v.Address}
	}
	return nil
}

// ChatFullInfoFromUserFull converts a tg.UserFull + tg.User into the Bot API
// ChatFullInfo response for private chats (getChat with a positive chat_id).
//
// Reference: Client.cpp JsonChat::store lines 1530-1594 (private case) +
// 1729-1799 (common is_full block).
func ChatFullInfoFromUserFull(uf *tg.UserFull, u *tg.User) *apitypes.ChatFullInfo {
	if uf == nil || u == nil {
		return nil
	}
	info := &apitypes.ChatFullInfo{
		Chat: apitypes.Chat{
			ID:        u.ID,
			Type:      apitypes.ChatTypePrivate,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Username:  u.Username,
		},
	}

	// can_send_gift: true for regular (non-bot) users.
	if !u.Bot {
		info.CanSendGift = true
	}

	// Active usernames (collectible usernames array — active only).
	for _, un := range u.Usernames {
		if un.Active {
			info.ActiveUsernames = append(info.ActiveUsernames, un.Username)
		}
	}
	// Ensure primary username is included if not already in the active list.
	if u.Username != "" && !slices.Contains(info.ActiveUsernames, u.Username) {
		info.ActiveUsernames = append([]string{u.Username}, info.ActiveUsernames...)
	}

	// Bio — only for non-bot users. TDLib (UserManager.cpp:10293-10353) leaves
	// bio_object as nullptr for bots; the about text goes into botInfo instead.
	if uf.About != "" && !u.Bot {
		info.Bio = uf.About
	}

	// Privacy flags.
	if uf.PrivateForwardName != "" {
		info.HasPrivateForwards = true
	}
	if uf.VoiceMessagesForbidden {
		info.HasRestrictedVoiceAndVideo = true
	}

	// Business info.
	if uf.BusinessIntro != nil {
		info.BusinessIntro = businessIntroToBotAPI(uf.BusinessIntro)
	}
	if uf.BusinessLocation != nil {
		info.BusinessLocation = businessLocationToBotAPI(uf.BusinessLocation)
	}
	if uf.BusinessWorkHours != nil {
		info.BusinessOpeningHours = businessWorkHoursToBotAPI(uf.BusinessWorkHours)
	}

	// Accepted gift types. Bots don't accept gifts → all false (the reference's
	// gift_settings.accepted_gift_types is all-false for a bot). For a regular
	// user, invert from DisallowedGiftsSettings (nil = no restrictions = all
	// accepted). Reference: Client.cpp JsonAcceptedGiftTypes + line 1571/9619.
	if u.Bot {
		info.AcceptedGiftTypes = &apitypes.AcceptedGiftTypes{}
	} else {
		info.AcceptedGiftTypes = disallowedToAcceptedGifts(uf.DisallowedGifts)
	}

	// Birthdate.
	if uf.Birthday != nil {
		info.Birthdate = &apitypes.Birthdate{
			Day:   uf.Birthday.Day,
			Month: uf.Birthday.Month,
			Year:  uf.Birthday.Year,
		}
	}

	// Personal chat (channel).
	if uf.PersonalChannelID != 0 {
		info.PersonalChat = &apitypes.Chat{
			ID:   -(1000000000000 + uf.PersonalChannelID),
			Type: apitypes.ChatTypeChannel,
		}
	}

	// Rating (StarsRating → ChatRating).
	if uf.StarsRating != nil {
		info.Rating = &apitypes.ChatRating{
			Level:              uf.StarsRating.Level,
			Rating:             uf.StarsRating.Stars,
			CurrentLevelRating: uf.StarsRating.CurrentLevelStars,
			NextLevelRating:    uf.StarsRating.NextLevelStars,
		}
	}

	// Paid message star count.
	if uf.SendPaidMessagesStars > 0 {
		info.PaidMessageStarCount = int32(uf.SendPaidMessagesStars)
	}

	// Profile photo (prefer personal photo, fall back to profile photo).
	var photo *tg.Photo
	if uf.PersonalPhoto != nil {
		photo, _ = uf.PersonalPhoto.(*tg.Photo)
	}
	if photo == nil && uf.ProfilePhoto != nil {
		photo, _ = uf.ProfilePhoto.(*tg.Photo)
	}
	if photo != nil {
		info.Photo = photoToChatPhoto(photo, u.ID, u.AccessHash)
	}

	// Pinned message.
	if uf.PinnedMsgID != 0 {
		info.PinnedMessage = &apitypes.Message{MessageID: int64(uf.PinnedMsgID)}
	}

	// TTL (message auto-delete time).
	if uf.TTLPeriod != 0 {
		info.MessageAutoDeleteTime = uf.TTLPeriod
	}

	// Accent color from tg.User.Color (*tg.PeerColor).
	// If no custom color is set, TDLib computes default as user_id % 7.
	if pc, ok := u.Color.(*tg.PeerColor); ok && pc != nil {
		info.AccentColorID = pc.Color
		if pc.BackgroundEmojiID != 0 {
			info.BackgroundCustomEmojiID = strconv.FormatInt(pc.BackgroundEmojiID, 10)
		}
	} else {
		info.AccentColorID = int32(u.ID % 7)
	}

	// max_reaction_count — always emitted by the reference for all chat types.
	// For private chats, the official API returns 11.
	info.MaxReactionCount = 11

	return info
}

// disallowedToAcceptedGifts inverts tg.DisallowedGiftsSettings into the Bot API
// AcceptedGiftTypes. If nil, all gifts are accepted (all true).
func disallowedToAcceptedGifts(d *tg.DisallowedGiftsSettings) *apitypes.AcceptedGiftTypes {
	if d == nil {
		return &apitypes.AcceptedGiftTypes{
			UnlimitedGifts:      true,
			LimitedGifts:        true,
			UniqueGifts:         true,
			PremiumSubscription: true,
			GiftsFromChannels:   true,
		}
	}
	return &apitypes.AcceptedGiftTypes{
		UnlimitedGifts:      !d.DisallowUnlimitedStargifts,
		LimitedGifts:        !d.DisallowLimitedStargifts,
		UniqueGifts:         !d.DisallowUniqueStargifts,
		PremiumSubscription: !d.DisallowPremiumGifts,
		GiftsFromChannels:   !d.DisallowStargiftsFromChannels,
	}
}

// businessIntroToBotAPI converts tg.BusinessIntro to Bot API BusinessIntro.
func businessIntroToBotAPI(bi *tg.BusinessIntro) *apitypes.BusinessIntro {
	if bi == nil {
		return nil
	}
	return &apitypes.BusinessIntro{
		Title:       bi.Title,
		Description: bi.Description,
	}
}

// businessLocationToBotAPI converts tg.BusinessLocation to Bot API BusinessLocation.
func businessLocationToBotAPI(bl *tg.BusinessLocation) *apitypes.BusinessLocation {
	if bl == nil {
		return nil
	}
	loc := &apitypes.BusinessLocation{Address: bl.Address}
	if gp, ok := bl.GeoPoint.(*tg.GeoPoint); ok && gp != nil {
		gloc := geoLocation(gp)
		loc.Location = &gloc
	}
	return loc
}

// businessWorkHoursToBotAPI converts tg.BusinessWorkHours to Bot API BusinessOpeningHours.
func businessWorkHoursToBotAPI(bw *tg.BusinessWorkHours) *apitypes.BusinessOpeningHours {
	if bw == nil {
		return nil
	}
	out := &apitypes.BusinessOpeningHours{
		TimeZoneName: bw.TimezoneID,
	}
	for _, w := range bw.WeeklyOpen {
		out.OpeningHours = append(out.OpeningHours, apitypes.BusinessOpeningHoursInterval{
			OpeningMinute: w.StartMinute,
			ClosingMinute: w.EndMinute,
		})
	}
	return out
}

// photoToChatPhoto converts a tg.Photo to the Bot API ChatPhoto.
// chatID and chatAccessHash identify the dialog (user/channel/chat) whose
// photo this is. They are embedded in the file_id so the Bot API client can
// later download the photo via inputPeerPhotoFileLocation.
func photoToChatPhoto(p *tg.Photo, chatID, chatAccessHash int64) *apitypes.ChatPhoto {
	if p == nil {
		return nil
	}
	return &apitypes.ChatPhoto{
		SmallFileID:       fileid.EncodeDialogPhoto(p.DCID, p.ID, chatID, chatAccessHash, false),
		SmallFileUniqueID: fileid.EncodeDialogPhotoUnique(p.ID, false),
		BigFileID:         fileid.EncodeDialogPhoto(p.DCID, p.ID, chatID, chatAccessHash, true),
		BigFileUniqueID:   fileid.EncodeDialogPhotoUnique(p.ID, true),
	}
}

// BannedRightsToPermissions inverts tg.ChatBannedRights into Bot API
// ChatPermissions.
func BannedRightsToPermissions(br *tg.ChatBannedRights) *apitypes.ChatPermissions {
	if br == nil {
		return nil
	}
	p := &apitypes.ChatPermissions{
		CanSendMessages:       !br.SendMessages,
		CanSendAudios:         !br.SendAudios,
		CanSendDocuments:      !br.SendDocs,
		CanSendPhotos:         !br.SendPhotos,
		CanSendVideos:         !br.SendVideos,
		CanSendVideoNotes:     !br.SendRoundvideos,
		CanSendVoiceNotes:     !br.SendVoices,
		CanSendPolls:          !br.SendPolls,
		CanSendOtherMessages:  !br.SendInline && !br.SendGifs && !br.SendStickers && !br.SendGames,
		CanAddWebPagePreviews: !br.EmbedLinks,
		CanReactToMessages:    !br.SendReactions,
		CanEditTag:            !br.EditRank,
		CanChangeInfo:         !br.ChangeInfo,
		CanInviteUsers:        !br.InviteUsers,
		CanPinMessages:        !br.PinMessages,
		CanManageTopics:       !br.ManageTopics,
	}
	// can_send_media_messages is computed: true if any of the 6 media types
	// are allowed (Client.cpp json_store_permissions lines 17493-17497).
	p.CanSendMediaMessages = p.CanSendAudios || p.CanSendDocuments ||
		p.CanSendPhotos || p.CanSendVideos ||
		p.CanSendVideoNotes || p.CanSendVoiceNotes
	return p
}

// ChatMemberFromParticipant converts a channel participant into a Bot API
// ChatMember. The user map resolves user details.
func ChatMemberFromParticipant(p tg.ChannelParticipantClass, users map[int64]*tg.User) *apitypes.ChatMember {
	if p == nil {
		return nil
	}
	switch part := p.(type) {
	case *tg.ChannelParticipant:
		return &apitypes.ChatMember{
			Status:   "member",
			User:     resolveUser(part.UserID, users),
			IsMember: true,
			Tag:      part.Rank,
		}
	case *tg.ChannelParticipantSelf:
		return &apitypes.ChatMember{
			Status:   "member",
			User:     resolveUser(part.UserID, users),
			IsMember: true,
			Tag:      part.Rank,
		}
	case *tg.ChannelParticipantCreator:
		m := &apitypes.ChatMember{
			Status:      "creator",
			User:        resolveUser(part.UserID, users),
			IsAnonymous: part.AdminRights != nil && part.AdminRights.Anonymous,
			IsMember:    true,
		}
		m.CustomTitle = part.Rank
		return m
	case *tg.ChannelParticipantAdmin:
		m := &apitypes.ChatMember{
			Status:      "administrator",
			User:        resolveUser(part.UserID, users),
			CanBeEdited: part.CanEdit,
			IsMember:    true,
		}
		m.CustomTitle = part.Rank
		if part.AdminRights != nil {
			fillAdminRights(m, part.AdminRights)
		}
		return m
	case *tg.ChannelParticipantBanned:
		status := "restricted"
		if part.Left {
			status = "kicked"
		}
		m := &apitypes.ChatMember{
			Status:   status,
			IsMember: !part.Left,
			Tag:      part.Rank,
		}
		if peer, ok := part.Peer.(*tg.PeerUser); ok {
			m.User = resolveUser(peer.UserID, users)
		}
		if part.BannedRights != nil {
			fillBannedRights(m, part.BannedRights)
			m.UntilDate = int64(part.BannedRights.UntilDate)
		}
		m.CustomTitle = part.Rank
		return m
	default:
		return nil
	}
}

// ChatMemberFromChatParticipant converts a basic group participant.
func ChatMemberFromChatParticipant(p tg.ChatParticipantClass, users map[int64]*tg.User) *apitypes.ChatMember {
	if p == nil {
		return nil
	}
	switch part := p.(type) {
	case *tg.ChatParticipant:
		return &apitypes.ChatMember{
			Status:   "member",
			User:     resolveUser(part.UserID, users),
			IsMember: true,
			Tag:      part.Rank,
		}
	case *tg.ChatParticipantCreator:
		m := &apitypes.ChatMember{
			Status:   "creator",
			User:     resolveUser(part.UserID, users),
			IsMember: true,
		}
		m.CustomTitle = part.Rank
		return m
	case *tg.ChatParticipantAdmin:
		m := &apitypes.ChatMember{
			Status:   "administrator",
			User:     resolveUser(part.UserID, users),
			IsMember: true,
		}
		m.CustomTitle = part.Rank
		return m
	default:
		return nil
	}
}

func resolveUser(id int64, users map[int64]*tg.User) *apitypes.User {
	if u, ok := users[id]; ok {
		return User(u)
	}
	return &apitypes.User{ID: id}
}

func fillAdminRights(m *apitypes.ChatMember, r *tg.ChatAdminRights) {
	m.CanManageChat = r.Other
	m.CanDeleteMessages = r.DeleteMessages
	m.CanManageVideoChats = r.ManageCall
	m.CanManageVoiceChats = r.ManageCall // legacy alias
	m.CanRestrictMembers = r.BanUsers
	m.CanPromoteMembers = r.AddAdmins
	m.CanChangeInfo = r.ChangeInfo
	m.CanInviteUsers = r.InviteUsers
	m.CanPostMessages = r.PostMessages
	m.CanEditMessages = r.EditMessages
	m.CanPinMessages = r.PinMessages
	m.CanPostStories = r.PostStories
	m.CanEditStories = r.EditStories
	m.CanDeleteStories = r.DeleteStories
	m.IsAnonymous = r.Anonymous
	m.CanManageTopics = r.ManageTopics
	m.CanManageDirectMessages = r.ManageDirectMessages
	m.CanManageTags = r.ManageRanks
}

func fillBannedRights(m *apitypes.ChatMember, r *tg.ChatBannedRights) {
	m.CanSendMessages = !r.SendMessages
	m.CanSendAudios = !r.SendAudios
	m.CanSendDocuments = !r.SendDocs
	m.CanSendPhotos = !r.SendPhotos
	m.CanSendVideos = !r.SendVideos
	m.CanSendVideoNotes = !r.SendRoundvideos
	m.CanSendVoiceNotes = !r.SendVoices
	m.CanSendPolls = !r.SendPolls
	m.CanSendOtherMessages = !r.SendInline && !r.SendGifs && !r.SendStickers && !r.SendGames
	m.CanAddWebPagePreviews = !r.EmbedLinks
	m.CanChangeInfo = !r.ChangeInfo
	m.CanInviteUsers = !r.InviteUsers
	m.CanPinMessages = !r.PinMessages
	m.CanManageTopics = !r.ManageTopics
}

// AdminRightsFromParams converts Bot API promoteChatMember parameters into
// tg.ChatAdminRights.
func AdminRightsFromParams(params map[string]bool) *tg.ChatAdminRights {
	r := &tg.ChatAdminRights{}
	if v, ok := params["can_manage_chat"]; ok {
		r.Other = v
	}
	if v, ok := params["can_delete_messages"]; ok {
		r.DeleteMessages = v
	}
	if v, ok := params["can_manage_video_chats"]; ok {
		r.ManageCall = v
	}
	if v, ok := params["can_restrict_members"]; ok {
		r.BanUsers = v
	}
	if v, ok := params["can_promote_members"]; ok {
		r.AddAdmins = v
	}
	if v, ok := params["can_change_info"]; ok {
		r.ChangeInfo = v
	}
	if v, ok := params["can_invite_users"]; ok {
		r.InviteUsers = v
	}
	if v, ok := params["can_post_messages"]; ok {
		r.PostMessages = v
	}
	if v, ok := params["can_edit_messages"]; ok {
		r.EditMessages = v
	}
	if v, ok := params["can_pin_messages"]; ok {
		r.PinMessages = v
	}
	if v, ok := params["can_post_stories"]; ok {
		r.PostStories = v
	}
	if v, ok := params["can_edit_stories"]; ok {
		r.EditStories = v
	}
	if v, ok := params["can_delete_stories"]; ok {
		r.DeleteStories = v
	}
	if v, ok := params["is_anonymous"]; ok {
		r.Anonymous = v
	}
	if v, ok := params["can_manage_topics"]; ok {
		r.ManageTopics = v
	}
	if v, ok := params["can_manage_direct_messages"]; ok {
		r.ManageDirectMessages = v
	}
	if v, ok := params["can_manage_tags"]; ok {
		r.ManageRanks = v
	}
	r.SetFlags()
	return r
}

// UserProfilePhotos converts a tg.PhotosClass into Bot API UserProfilePhotos.
func UserProfilePhotos(result tg.PhotosClass) *apitypes.UserProfilePhotos {
	if result == nil {
		return &apitypes.UserProfilePhotos{TotalCount: 0, Photos: [][]apitypes.PhotoSize{}}
	}
	switch r := result.(type) {
	case *tg.PhotosPhotosSlice:
		photos := make([][]apitypes.PhotoSize, 0, len(r.Photos))
		for _, p := range r.Photos {
			if photo, ok := p.(*tg.Photo); ok {
				photos = append(photos, Photo(photo))
			}
		}
		return &apitypes.UserProfilePhotos{
			TotalCount: int(r.Count),
			Photos:     photos,
		}
	case *tg.PhotosPhotos:
		photos := make([][]apitypes.PhotoSize, 0, len(r.Photos))
		for _, p := range r.Photos {
			if photo, ok := p.(*tg.Photo); ok {
				photos = append(photos, Photo(photo))
			}
		}
		return &apitypes.UserProfilePhotos{
			TotalCount: len(photos),
			Photos:     photos,
		}
	default:
		return &apitypes.UserProfilePhotos{TotalCount: 0, Photos: [][]apitypes.PhotoSize{}}
	}
}

// CustomEmojiStickers converts a TLObject response from MessagesGetCustomEmojiDocuments
// into a JSON-serializable sticker list.
//
// The RPC returns *tg.GenericVector (Items []TLObject), where each item is a
// *tg.Document carrying DocumentAttributeCustomEmoji. We type-assert and delegate
// to StickersFromDocuments.
func CustomEmojiStickers(result tg.TLObject) any {
	if result == nil {
		return []apitypes.Sticker{}
	}
	vec, ok := result.(*tg.GenericVector)
	if !ok {
		return []apitypes.Sticker{}
	}
	docs := make([]tg.DocumentClass, 0, len(vec.Items))
	for _, item := range vec.Items {
		if doc, ok := item.(*tg.Document); ok {
			docs = append(docs, doc)
		}
	}
	return StickersFromDocuments(docs, "")
}

// SavedMusic converts a tg.SavedMusicClass into a Bot API Audio array.
func SavedMusic(result tg.SavedMusicClass) any {
	if result == nil {
		return []any{}
	}
	switch r := result.(type) {
	case *tg.UsersSavedMusic:
		audios := make([]any, 0, len(r.Documents))
		for _, doc := range r.Documents {
			if d, ok := doc.(*tg.Document); ok {
				audios = append(audios, Audio(d))
			}
		}
		return audios
	default:
		return []any{}
	}
}

// Messages converts a tg.MessagesClass into a Bot API Message array.
func Messages(result tg.MessagesClass) any {
	if result == nil {
		return []any{}
	}
	var msgs []tg.MessageClass
	switch r := result.(type) {
	case *tg.MessagesMessages:
		msgs = r.Messages
	case *tg.MessagesMessagesSlice:
		msgs = r.Messages
	case *tg.MessagesChannelMessages:
		msgs = r.Messages
	default:
		return []any{}
	}
	out := make([]any, 0, len(msgs))
	for _, m := range msgs {
		if msg, ok := m.(*tg.Message); ok {
			out = append(out, Message(msg))
		}
	}
	return out
}

// SecureValueErrors converts a JSON array of Bot API PassportElementError
// objects into tg.SecureValueErrorClass slices.
func SecureValueErrors(jsonStr string) ([]tg.SecureValueErrorClass, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("invalid errors JSON: %w", err)
	}
	result := make([]tg.SecureValueErrorClass, 0, len(raw))
	for _, r := range raw {
		var base struct {
			Source string `json:"source"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(r, &base); err != nil {
			return nil, err
		}
		var errObj struct {
			Source            string   `json:"source"`
			Type              string   `json:"type"`
			FieldHash         string   `json:"field_hash"`
			DataHash          string   `json:"data_hash"`
			Message           string   `json:"message"`
			FileHash          string   `json:"file_hash"`
			FrontSide         string   `json:"front_side"`
			ReverseSide       string   `json:"reverse_side"`
			Selfie            string   `json:"selfie"`
			TranslationHashes []string `json:"translation_hashes"`
			ElementHashes     []string `json:"element_hashes"`
		}
		if err := json.Unmarshal(r, &errObj); err != nil {
			return nil, err
		}
		// Convert Bot API field_hash (string) to TL hash (int64).
		// Bot API uses a string hash; TL uses int64. We parse as 0 for now.
		h := int64(0)
		switch base.Source {
		case "data":
			result = append(result, &tg.SecureValueErrorData{
				Type:     passportTypeFromBotAPI(base.Type),
				DataHash: []byte(errObj.DataHash),
				Field:    errObj.FieldHash,
				Text:     errObj.Message,
			})
		case "front_side":
			result = append(result, &tg.SecureValueErrorFrontSide{
				Type:     passportTypeFromBotAPI(base.Type),
				FileHash: []byte(errObj.FileHash),
				Text:     errObj.Message,
			})
		case "reverse_side":
			result = append(result, &tg.SecureValueErrorReverseSide{
				Type:     passportTypeFromBotAPI(base.Type),
				FileHash: []byte(errObj.FileHash),
				Text:     errObj.Message,
			})
		case "selfie":
			result = append(result, &tg.SecureValueErrorSelfie{
				Type:     passportTypeFromBotAPI(base.Type),
				FileHash: []byte(errObj.FileHash),
				Text:     errObj.Message,
			})
		case "file":
			result = append(result, &tg.SecureValueError{
				Type: passportTypeFromBotAPI(base.Type),
				Hash: []byte(errObj.FileHash),
				Text: errObj.Message,
			})
		case "files":
			result = append(result, &tg.SecureValueError{
				Type: passportTypeFromBotAPI(base.Type),
				Hash: []byte(errObj.FileHash),
				Text: errObj.Message,
			})
		case "translation_file":
			result = append(result, &tg.SecureValueErrorTranslationFile{
				Type:     passportTypeFromBotAPI(base.Type),
				FileHash: []byte(errObj.FileHash),
				Text:     errObj.Message,
			})
		case "translation_files":
			result = append(result, &tg.SecureValueErrorTranslationFile{
				Type:     passportTypeFromBotAPI(base.Type),
				FileHash: []byte(errObj.FileHash),
				Text:     errObj.Message,
			})
		default:
			_ = h
		}
	}
	return result, nil
}

// passportTypeFromBotAPI converts Bot API passport type string to tg.SecureValueTypeClass.
func passportTypeFromBotAPI(t string) tg.SecureValueTypeClass {
	switch t {
	case "personal_details":
		return &tg.SecureValueTypePersonalDetails{}
	case "passport":
		return &tg.SecureValueTypePassport{}
	case "driver_license":
		return &tg.SecureValueTypeDriverLicense{}
	case "identity_card":
		return &tg.SecureValueTypeIdentityCard{}
	case "internal_passport":
		return &tg.SecureValueTypeInternalPassport{}
	case "address":
		return &tg.SecureValueTypeAddress{}
	case "utility_bill":
		return &tg.SecureValueTypeUtilityBill{}
	case "bank_statement":
		return &tg.SecureValueTypeBankStatement{}
	case "rental_agreement":
		return &tg.SecureValueTypeRentalAgreement{}
	case "passport_registration":
		return &tg.SecureValueTypePassportRegistration{}
	case "temporary_registration":
		return &tg.SecureValueTypeTemporaryRegistration{}
	case "phone_number":
		return &tg.SecureValueTypePhone{}
	case "email":
		return &tg.SecureValueTypeEmail{}
	default:
		return &tg.SecureValueTypePersonalDetails{}
	}
}

// StarTransactions converts a PaymentsStarsStatus into Bot API StarTransactions.
func StarTransactions(result *tg.PaymentsStarsStatus) any {
	if result == nil {
		return map[string]any{"transactions": []any{}}
	}
	transactions := make([]any, 0, len(result.History))
	for _, tx := range result.History {
		t := map[string]any{
			"id":     tx.ID,
			"amount": tx.Amount,
		}
		if tx.Date != 0 {
			t["date"] = tx.Date
		}
		transactions = append(transactions, t)
	}
	return map[string]any{
		"transactions": transactions,
	}
}

// StarGifts converts a StarGiftsClass into the Bot API AvailableGifts response.
// Mirrors TDLib's getAvailableGifts: gifts that are sold out are excluded
// (reference process_get_available_gifts_query → td_api::getAvailableGifts).
// Gift field order matches JsonGift (Client.cpp:1824-1859).
//
// setNames maps sticker-set IDs to their short_name; the client layer resolves
// these (gift custom-emoji docs reference their set by ID) so each gift sticker
// can carry its set_name like the reference.
func StarGifts(result tg.StarGiftsClass, setNames map[int64]string) any {
	out := apitypes.Gifts{Gifts: []apitypes.Gift{}}
	if result == nil {
		return out
	}
	r, ok := result.(*tg.PaymentsStarGifts)
	if !ok {
		return out
	}
	for _, sg := range r.Gifts {
		gift, ok := sg.(*tg.StarGift)
		if !ok {
			continue
		}
		// TDLib getAvailableGifts excludes sold-out gifts.
		if gift.SoldOut {
			continue
		}
		var sticker *apitypes.Sticker
		if doc, ok := gift.Sticker.(*tg.Document); ok {
			setName := ""
			if id, ok := StickerDocSetID(doc); ok {
				setName = setNames[id]
			}
			sticker = StickerFromDocument(doc, nil, setName)
		}
		g := apitypes.Gift{
			ID:               strconv.FormatInt(gift.ID, 10),
			Sticker:          sticker,
			StarCount:        gift.Stars,
			UpgradeStarCount: gift.UpgradeStars,
		}
		if gift.AvailabilityTotal > 0 {
			g.RemainingCount = gift.AvailabilityRemains
			g.TotalCount = gift.AvailabilityTotal
		}
		if gift.PerUserTotal > 0 {
			g.PersonalRemainingCount = gift.PerUserRemains
			g.PersonalTotalCount = gift.PerUserTotal
		}
		if gift.RequirePremium {
			g.IsPremium = true
		}
		if gift.PeerColorAvailable {
			g.HasColors = true
		}
		if gift.UpgradeVariants > 0 {
			g.UniqueGiftVariantCount = gift.UpgradeVariants
		}
		// TODO(parity): publisher_chat (ReleasedBy peer → Chat) and background
		// (StarGiftBackground converter) are not yet emitted.
		out.Gifts = append(out.Gifts, g)
	}
	return out
}

// SavedStarGifts converts a PaymentsSavedStarGifts into Bot API response.
func SavedStarGifts(result *tg.PaymentsSavedStarGifts) any {
	if result == nil {
		return map[string]any{"gifts": []any{}}
	}
	gifts := make([]any, 0, len(result.Gifts))
	for _, g := range result.Gifts {
		gifts = append(gifts, map[string]any{
			"owned_gift_id": g.SavedID,
		})
	}
	return map[string]any{
		"gifts":       gifts,
		"next_offset": result.NextOffset,
	}
}

// UserBoosts converts a PremiumBoostsList into Bot API response.
func UserBoosts(result *tg.PremiumBoostsList) any {
	if result == nil {
		return map[string]any{"boosts": []any{}}
	}
	boosts := make([]any, 0, len(result.Boosts))
	for _, b := range result.Boosts {
		boost := map[string]any{
			"boost_id": b.ID,
			"date":     b.Date,
		}
		if b.UserID != 0 {
			boost["user_id"] = b.UserID
		}
		boosts = append(boosts, boost)
	}
	return map[string]any{
		"boosts":      boosts,
		"next_offset": result.NextOffset,
	}
}

// (convertSticker removed: StarGifts now uses the full StickerFromDocument
// converter for rich gift stickers — emoji/type/custom_emoji_id/dimensions.)
