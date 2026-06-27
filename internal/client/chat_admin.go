package client

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("promotechatmember", (*Client).promoteChatMember)
	Register("restrictchatmember", (*Client).restrictChatMember)
	Register("setchatadministratorcustomtitle", (*Client).setChatAdministratorCustomTitle)
	Register("banchatmember", (*Client).banChatMember)
	Register("unbanchatmember", (*Client).unbanChatMember)
	Register("banchatsenderchat", (*Client).banChatSenderChat)
	Register("unbanchatsenderchat", (*Client).unbanChatSenderChat)
	Register("setchatmembertag", (*Client).setChatMemberTag)
}

// promoteChatMember implements the Bot API promoteChatMember method.
// Only works for supergroups/channels.
func (c *Client) promoteChatMember(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	_, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if chatType != "channel" {
		return nil, NewError(400, "Bad Request: only supergroups and channels are supported")
	}


	inputCh, err := c.resolveInputChannel(ctx, extractChannelID(chatIDint(chatID)))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	rights := extractAdminRights(q)
	req := &tg.ChannelsEditAdminRequest{
		Channel:     inputCh,
		UserID:      &tg.InputUser{UserID: uid},
		AdminRights: rights,
	}
	if rank := q.Arg("custom_title"); rank != "" {
		req.Rank = rank
		req.Flags.Set(0)
	}
	req.SetFlags()

	_, err = c.rpc.ChannelsEditAdmin(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// restrictChatMember implements the Bot API restrictChatMember method.
// Only works for supergroups.
func (c *Client) restrictChatMember(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	permissionsJSON := q.Arg("permissions")
	if permissionsJSON == "" {
		return nil, NewError(400, "Bad Request: parameter \"permissions\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	_, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if chatType != "channel" {
		return nil, NewError(400, "Bad Request: only supergroups are supported")
	}


	var perms map[string]bool
	if err := json.Unmarshal([]byte(permissionsJSON), &perms); err != nil {
		return nil, NewError(400, "Bad Request: invalid permissions JSON")
	}

	inputCh, err := c.resolveInputChannel(ctx, extractChannelID(chatIDint(chatID)))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	applyLegacyPermissionBundling(perms, q.ArgBool("use_independent_chat_permissions"))
	bannedRights := permissionsToBannedRights(perms)
	if untilStr := q.Arg("until_date"); untilStr != "" {
		if until, err := strconv.ParseInt(untilStr, 10, 64); err == nil && until > 0 {
			bannedRights.UntilDate = int32(until)
			bannedRights.Flags.Set(17)
		}
	}
	bannedRights.SetFlags()

	_, err = c.rpc.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
		Channel:      inputCh,
		Participant:  &tg.InputPeerUser{UserID: uid},
		BannedRights: bannedRights,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setChatAdministratorCustomTitle implements the Bot API setChatAdministratorCustomTitle method.
func (c *Client) setChatAdministratorCustomTitle(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	customTitle := q.Arg("custom_title")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	switch chatType {
	case "chat":
		// For basic groups, use messages.editChatParticipantRank.
		peer := &tg.InputPeerChat{ChatID: id}
		userPeer := &tg.InputPeerUser{UserID: uid}
		_, err = c.rpc.MessagesEditChatParticipantRank(ctx, &tg.MessagesEditChatParticipantRankRequest{
			Peer:        peer,
			Participant: userPeer,
			Rank:        customTitle,
		})
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		// For supergroups, we need to re-promote with the new rank.
		// Fetch current admin rights first.
		_, err = c.rpc.ChannelsEditAdmin(ctx, &tg.ChannelsEditAdminRequest{
			Channel:     inputCh,
			UserID:      &tg.InputUser{UserID: uid},
			AdminRights: &tg.ChatAdminRights{}, // minimal — preserves existing rights on Telegram side
			Rank:        customTitle,
		})
	default:
		return nil, NewError(400, "Bad Request: only groups and supergroups are supported")
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// banChatMember implements the Bot API banChatMember method.
func (c *Client) banChatMember(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, err
	}

	bannedRights := &tg.ChatBannedRights{
		ViewMessages: true,
	}
	bannedRights.Flags.Set(0) // view_messages flag

	if untilStr := q.Arg("until_date"); untilStr != "" {
		if until, err := strconv.ParseInt(untilStr, 10, 64); err == nil && until > 0 {
			bannedRights.UntilDate = int32(until)
			bannedRights.Flags.Set(17)
		}
	}
	bannedRights.SetFlags()

	switch chatType {
	case "chat":
		// For basic groups, use messages.deleteChatUser.
		_, err = c.rpc.MessagesDeleteChatUser(ctx, &tg.MessagesDeleteChatUserRequest{
			ChatID: id,
			UserID: &tg.InputUser{UserID: uid},
		})
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		_, err = c.rpc.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
			Channel:      inputCh,
			Participant:  &tg.InputPeerUser{UserID: uid},
			BannedRights: bannedRights,
		})
		// revoke_messages: also delete the banned user's messages (channels only).
		if err == nil && q.ArgBool("revoke_messages") {
			_, err = c.rpc.ChannelsDeleteParticipantHistory(ctx, &tg.ChannelsDeleteParticipantHistoryRequest{
				Channel:     inputCh,
				Participant: &tg.InputPeerUser{UserID: uid},
			})
		}
	default:
		return nil, NewError(400, "Bad Request: invalid chat_id")
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// unbanChatMember implements the Bot API unbanChatMember method.
func (c *Client) unbanChatMember(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	_, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if chatType != "channel" {
		return nil, NewError(400, "Bad Request: only supergroups and channels are supported")
	}


	inputCh, err := c.resolveInputChannel(ctx, extractChannelID(chatIDint(chatID)))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	// only_if_banned: skip the unban (no-op, success) when the user is not currently banned.
	if q.ArgBool("only_if_banned") {
		resp, gerr := c.rpc.ChannelsGetParticipant(ctx, &tg.ChannelsGetParticipantRequest{
			Channel:     inputCh,
			Participant: &tg.InputPeerUser{UserID: uid},
		})
		if gerr == nil {
			if _, banned := resp.(*tg.ChannelParticipantBanned); !banned {
				return true, nil
			}
		}
		// On lookup error, fall through and attempt the unban.
	}

	// Empty ChatBannedRights = unbanned.
	_, err = c.rpc.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
		Channel:      inputCh,
		Participant:  &tg.InputPeerUser{UserID: uid},
		BannedRights: &tg.ChatBannedRights{},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// banChatSenderChat implements the Bot API banChatSenderChat method.
func (c *Client) banChatSenderChat(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	senderChatID := q.Arg("sender_chat_id")
	if senderChatID == "" {
		return nil, NewError(400, "Bad Request: parameter \"sender_chat_id\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	_, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if chatType != "channel" {
		return nil, NewError(400, "Bad Request: only supergroups and channels are supported")
	}

	scID, scType, err := parseChatID(senderChatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid sender_chat_id")
	}

	inputCh, err := c.resolveInputChannel(ctx, extractChannelID(chatIDint(chatID)))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	var participant tg.InputPeerClass
	switch scType {
	case "channel":
		participant = &tg.InputPeerChannel{ChannelID: extractChannelID(scID)}
	case "chat":
		participant = &tg.InputPeerChat{ChatID: scID}
	default:
		return nil, NewError(400, "Bad Request: sender_chat_id must be a chat or channel")
	}

	bannedRights := &tg.ChatBannedRights{ViewMessages: true}
	bannedRights.Flags.Set(0)
	bannedRights.SetFlags()

	if untilStr := q.Arg("until_date"); untilStr != "" {
		if until, err := strconv.ParseInt(untilStr, 10, 64); err == nil && until > 0 {
			bannedRights.UntilDate = int32(until)
			bannedRights.Flags.Set(17)
		}
	}

	_, err = c.rpc.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
		Channel:      inputCh,
		Participant:  participant,
		BannedRights: bannedRights,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// unbanChatSenderChat implements the Bot API unbanChatSenderChat method.
func (c *Client) unbanChatSenderChat(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	senderChatID := q.Arg("sender_chat_id")
	if senderChatID == "" {
		return nil, NewError(400, "Bad Request: parameter \"sender_chat_id\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	_, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if chatType != "channel" {
		return nil, NewError(400, "Bad Request: only supergroups and channels are supported")
	}

	scID, scType, err := parseChatID(senderChatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid sender_chat_id")
	}

	inputCh, err := c.resolveInputChannel(ctx, extractChannelID(chatIDint(chatID)))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	var participant tg.InputPeerClass
	switch scType {
	case "channel":
		participant = &tg.InputPeerChannel{ChannelID: extractChannelID(scID)}
	case "chat":
		participant = &tg.InputPeerChat{ChatID: scID}
	default:
		return nil, NewError(400, "Bad Request: sender_chat_id must be a chat or channel")
	}

	// Empty banned rights = unbanned.
	_, err = c.rpc.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
		Channel:      inputCh,
		Participant:  participant,
		BannedRights: &tg.ChatBannedRights{},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setChatMemberTag implements the Bot API setChatMemberTag method.
func (c *Client) setChatMemberTag(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	tag := q.Arg("tag")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	switch chatType {
	case "chat":
		peer := &tg.InputPeerChat{ChatID: id}
		userPeer := &tg.InputPeerUser{UserID: uid}
		_, err = c.rpc.MessagesEditChatParticipantRank(ctx, &tg.MessagesEditChatParticipantRankRequest{
			Peer:        peer,
			Participant: userPeer,
			Rank:        tag,
		})
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		_, err = c.rpc.ChannelsEditAdmin(ctx, &tg.ChannelsEditAdminRequest{
			Channel:     inputCh,
			UserID:      &tg.InputUser{UserID: uid},
			AdminRights: &tg.ChatAdminRights{},
			Rank:        tag,
		})
	default:
		return nil, NewError(400, "Bad Request: only groups and supergroups are supported")
	}
	if err != nil {
		// Method-aware: RANK_* errors rewrite to TAG_* here (vs CUSTOM_TITLE_*
		// elsewhere); see internal/client/errors.go rpcErrorWith.
		return nil, rpcErrorWith(err, "", q.Method)
	}
	return true, nil
}

// extractAdminRights reads promoteChatMember parameters from the query and
// builds a tg.ChatAdminRights.
func extractAdminRights(q *server.Query) *tg.ChatAdminRights {
	r := &tg.ChatAdminRights{}
	if q.ArgBool("can_manage_chat") {
		r.Other = true
	}
	if q.ArgBool("can_delete_messages") {
		r.DeleteMessages = true
	}
	// can_manage_voice_chats is a deprecated alias for can_manage_video_chats/can_manage_calls
	// (Client.cpp:15920 reads them as `voice || video`). Both map to the same admin right.
	if q.ArgBool("can_manage_video_chats") || q.ArgBool("can_manage_voice_chats") {
		r.ManageCall = true
	}
	if q.ArgBool("can_restrict_members") {
		r.BanUsers = true
	}
	if q.ArgBool("can_promote_members") {
		r.AddAdmins = true
	}
	if q.ArgBool("can_change_info") {
		r.ChangeInfo = true
	}
	if q.ArgBool("can_invite_users") {
		r.InviteUsers = true
	}
	if q.ArgBool("can_post_messages") {
		r.PostMessages = true
	}
	if q.ArgBool("can_edit_messages") {
		r.EditMessages = true
	}
	if q.ArgBool("can_pin_messages") {
		r.PinMessages = true
	}
	if q.ArgBool("can_post_stories") {
		r.PostStories = true
	}
	if q.ArgBool("can_edit_stories") {
		r.EditStories = true
	}
	if q.ArgBool("can_delete_stories") {
		r.DeleteStories = true
	}
	if q.ArgBool("is_anonymous") {
		r.Anonymous = true
	}
	if q.ArgBool("can_manage_topics") {
		r.ManageTopics = true
	}
	if q.ArgBool("can_manage_direct_messages") {
		r.ManageDirectMessages = true
	}
	if q.ArgBool("can_manage_tags") {
		r.ManageRanks = true
	}
	r.SetFlags()
	return r
}

// applyLegacyPermissionBundling replicates the official get_chat_permissions legacy
// bundling (Client.cpp ~12049-12160). When use_independent_chat_permissions is false
// (the default), allowing "other messages" or link previews implies all media types
// and text, and polls/media imply text. No-op when independent is true.
func applyLegacyPermissionBundling(perms map[string]bool, independent bool) {
	if independent {
		return
	}
	if perms["can_send_other_messages"] || perms["can_add_web_page_previews"] {
		perms["can_send_audios"] = true
		perms["can_send_documents"] = true
		perms["can_send_photos"] = true
		perms["can_send_videos"] = true
		perms["can_send_video_notes"] = true
		perms["can_send_voice_notes"] = true
		perms["can_send_messages"] = true
	}
	if perms["can_send_polls"] {
		perms["can_send_messages"] = true
	}
	// Legacy aggregate can_send_media_messages expands to all media types + text.
	if perms["can_send_media_messages"] {
		perms["can_send_audios"] = true
		perms["can_send_documents"] = true
		perms["can_send_photos"] = true
		perms["can_send_videos"] = true
		perms["can_send_video_notes"] = true
		perms["can_send_voice_notes"] = true
		perms["can_send_messages"] = true
	}
}

// permissionsToBannedRights converts Bot API ChatPermissions (positive) into
// tg.ChatBannedRights (negative/inverted).
func permissionsToBannedRights(p map[string]bool) *tg.ChatBannedRights {
	br := &tg.ChatBannedRights{}
	if v, ok := p["can_send_messages"]; ok {
		br.SendMessages = !v
		br.Flags.Set(1)
	}
	if v, ok := p["can_send_audios"]; ok {
		br.SendAudios = !v
		br.Flags.Set(22)
	}
	if v, ok := p["can_send_documents"]; ok {
		br.SendDocs = !v
		br.Flags.Set(19)
	}
	if v, ok := p["can_send_photos"]; ok {
		br.SendPhotos = !v
		br.Flags.Set(19)
	}
	if v, ok := p["can_send_videos"]; ok {
		br.SendVideos = !v
		br.Flags.Set(20)
	}
	if v, ok := p["can_send_video_notes"]; ok {
		br.SendRoundvideos = !v
		br.Flags.Set(21)
	}
	if v, ok := p["can_send_voice_notes"]; ok {
		br.SendVoices = !v
		br.Flags.Set(22)
	}
	if v, ok := p["can_send_polls"]; ok {
		br.SendPolls = !v
		br.Flags.Set(8)
	}
	if v, ok := p["can_send_other_messages"]; ok {
		br.SendInline = !v
		br.SendGifs = !v
		br.SendStickers = !v
		br.SendGames = !v
		br.Flags.Set(6)
		br.Flags.Set(4)
		br.Flags.Set(3)
		br.Flags.Set(5)
	}
	if v, ok := p["can_add_web_page_previews"]; ok {
		br.EmbedLinks = !v
		br.Flags.Set(7)
	}
	if v, ok := p["can_change_info"]; ok {
		br.ChangeInfo = !v
		br.Flags.Set(10)
	}
	if v, ok := p["can_invite_users"]; ok {
		br.InviteUsers = !v
		br.Flags.Set(15)
	}
	if v, ok := p["can_pin_messages"]; ok {
		br.PinMessages = !v
		br.Flags.Set(17)
	}
	if v, ok := p["can_manage_topics"]; ok {
		br.ManageTopics = !v
		br.Flags.Set(18)
	}
	br.SetFlags()
	return br
}

// chatIDint parses a chat_id string to int64.
func chatIDint(chatID string) int64 {
	id, _ := strconv.ParseInt(chatID, 10, 64)
	return id
}
