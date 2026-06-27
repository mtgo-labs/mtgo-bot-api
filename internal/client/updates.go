package client

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/mtgo-labs/mtgo/telegram"
	mtgotypes "github.com/mtgo-labs/mtgo/telegram/types"
	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// registerIngestion wires the mtgo update dispatcher so incoming updates
// populate the peer cache (for raw-TL InputPeer construction) and feed the
// per-bot update queue (so getUpdates/webhooks receive them). Must be called
// once after the connection is established.
func (c *Client) registerIngestion() {
	c.conn.OnRawUpdate(func(ctx *telegram.Context) {
		c.ingest(ctx)
	})
}

// ingest processes one incoming update: caches referenced peers (carrying
// access_hash, which raw TL peer construction requires) and pushes a Bot API
// Update onto the queue. Runs in mtgo's update-dispatch goroutine; both the
// peer cache and TQueue are mutex-protected so this is safe.
func (c *Client) ingest(ctx *telegram.Context) {
	if ctx == nil || ctx.Update == nil {
		return
	}
	upd := ctx.Update
	selfID, _ := strconv.ParseInt(c.botID, 10, 64)

	// 1. Populate the peer cache from the Users/Chats maps carried by the update.
	for _, u := range upd.Users {
		if u == nil || u.ID == 0 {
			continue
		}
		c.savePeer(storage.Peer{
			ID:         u.ID,
			AccessHash: u.AccessHash,
			Type:       storage.PeerTypeUser,
			Username:   u.Username,
			FirstName:  u.FirstName,
		})
	}
	for _, ch := range upd.Chats {
		if ch == nil || ch.ID == 0 {
			continue
		}
		c.saveChatPeer(ch)
	}

	// 1b. Track the bot's OWN chat membership transitions (kicked/left/joined) for
	// the check_chat_access pre-flight. mtgo populates ChatMember for both
	// UpdateChatParticipant and UpdateChannelParticipant (client.go:1995).
	if upd.ChatMember != nil && selfID != 0 {
		if cm := upd.ChatMember.NewChatMember; cm != nil && cm.User != nil && cm.User.ID == selfID {
			if chat := upd.ChatMember.Chat; chat != nil && chat.ID != 0 {
				c.saveBotMemberStatus(chat.ID, chatMemberStatusToCache(cm.Status))
			}
		}
	}

	// 2. Convert to a Bot API Update and enqueue it.
	if obj := buildUpdateObject(upd, selfID, c.msgs); obj != nil {
		c.pushUpdateObj(obj)
	}
}

// savePeer upserts a peer into the per-bot cache, ignoring store errors so a
// storage hiccup never panics the update loop.
func (c *Client) savePeer(p storage.Peer) {
	if c.store == nil {
		return
	}
	_ = c.store.SavePeer(context.Background(), p)
}

// saveChatPeer caches a chat/channel and its access flags. Legacy group chats
// need no access_hash; channels/supergroups carry it via Raw (*tg.Channel).
// It also records the bot's left/kicked state + chat-type/liveness flags used by
// the check_chat_access pre-flight (chat_access.go).
func (c *Client) saveChatPeer(ch *mtgotypes.Chat) {
	switch raw := ch.Raw.(type) {
	case *tg.Channel:
		c.savePeer(storage.Peer{ID: raw.ID, AccessHash: raw.AccessHash, Type: storage.PeerTypeChannel})
		c.saveChatFlags(raw.ID, raw.Megagroup, false, "")
		if raw.Left {
			c.saveBotMemberStatus(raw.ID, "left")
		}
	case *tg.ChannelForbidden:
		// Bot was kicked/removed from the channel/supergroup.
		c.savePeer(storage.Peer{ID: raw.ID, AccessHash: raw.AccessHash, Type: storage.PeerTypeChannel})
		c.saveChatFlags(raw.ID, raw.Megagroup, false, "")
		c.saveBotMemberStatus(raw.ID, "kicked")
	case *tg.Chat:
		// Legacy group: no access_hash in MTProto.
		c.savePeer(storage.Peer{ID: raw.ID, Type: storage.PeerTypeChat})
		migratedTo := ""
		if ic, ok := raw.MigratedTo.(*tg.InputChannel); ok && ic.ChannelID != 0 {
			migratedTo = "-100" + strconv.FormatInt(ic.ChannelID, 10) // Bot API chat_id of the supergroup
		}
		c.saveChatFlags(raw.ID, false, raw.Deactivated, migratedTo)
		if raw.Left {
			c.saveBotMemberStatus(raw.ID, "left")
		}
	case *tg.ChatForbidden:
		// Legacy group deleted or bot kicked — treat as inaccessible.
		c.savePeer(storage.Peer{ID: raw.ID, Type: storage.PeerTypeChat})
		c.saveBotMemberStatus(raw.ID, "kicked")
	default:
		// Unknown raw type; best-effort cache by chat id with no hash.
		c.savePeer(storage.Peer{ID: ch.ID, Type: storage.PeerTypeChat})
	}
}

// saveChatFlags updates the cached chat-type/liveness flags (best-effort).
func (c *Client) saveChatFlags(id int64, isMegagroup, isDeactivated bool, migratedTo string) {
	if c.store == nil {
		return
	}
	_ = c.store.SaveChatFlags(context.Background(), id, isMegagroup, isDeactivated, migratedTo)
}

// saveBotMemberStatus records the bot's own membership status (best-effort).
func (c *Client) saveBotMemberStatus(id int64, status string) {
	if c.store == nil {
		return
	}
	_ = c.store.SaveBotMemberStatus(context.Background(), id, status)
}

// chatMemberStatusToCache maps a mtgo ChatMemberStatus to the cached bot_member_status
// value used by check_chat_access. Present statuses collapse to "member" (the
// pre-flight only short-circuits on the clear kicked/left cases).
func chatMemberStatusToCache(s mtgotypes.ChatMemberStatus) string {
	switch s {
	case mtgotypes.ChatMemberStatusLeft:
		return "left"
	case mtgotypes.ChatMemberStatusBanned:
		return "kicked"
	case mtgotypes.ChatMemberStatusMember, mtgotypes.ChatMemberStatusAdministrator,
		mtgotypes.ChatMemberStatusOwner, mtgotypes.ChatMemberStatusRestricted:
		return "member"
	}
	return ""
}

// buildUpdateObject constructs the Bot API Update object. update_id is assigned
// from the TQueue EventID and stored during pushUpdateObj. Returns nil if the
// update carries no Bot-API-relevant payload. cache (nil-safe) is the
// per-bot seen-message store: each converted message is recorded so later
// pinned_message / reply_to_message references can be resolved to full content
// (mirrors the reference's get_message / get_reply_to_message_info).
func buildUpdateObject(upd *telegram.Update, botSelfID int64, cache *msgCache) map[string]any {
	if upd == nil {
		return nil
	}

	// Build user lookup map from the update's Users array.
	userMap := make(map[int64]*tg.User)
	for _, u := range upd.Users {
		if u != nil && u.ID != 0 {
			userMap[u.ID] = &tg.User{
				ID:        u.ID,
				FirstName: u.FirstName,
				LastName:  u.LastName,
				Username:  u.Username,
				Phone:     u.Phone,
			}
		}
	}

	// Build chat lookup map (keyed by Bot API chat id) from the update's Chats
	// array, so channel/group chats in updates carry title/username/is_forum
	// like the reference (peerToChat only sets id+type).
	chatMap := make(map[int64]apitypes.Chat)
	for _, ch := range upd.Chats {
		if ch == nil || ch.Raw == nil {
			continue
		}
		switch raw := ch.Raw.(type) {
		case *tg.Channel:
			ac := apitypes.Chat{
				ID:       -1000000000000 - raw.ID,
				Title:    raw.Title,
				Username: raw.Username,
				Type:     apitypes.ChatTypeSupergroup,
			}
			if raw.Forum {
				ac.IsForum = true
			}
			chatMap[ac.ID] = ac
		case *tg.Chat:
			chatMap[-raw.ID] = apitypes.Chat{ID: -raw.ID, Title: raw.Title, Type: apitypes.ChatTypeGroup}
		}
	}

	broadcast := broadcastChannels(upd.Chats)

	if upd.Message != nil {
		if m := rawMessage(upd.Message); m != nil {
			msg := convert.Message(m)
			enrichMessage(msg, userMap, chatMap)
			cacheMessageAndReply(cache, msg, m)
			if isBroadcastPeer(m.PeerID, broadcast) {
				msg.Chat.Type = apitypes.ChatTypeChannel
				return map[string]any{"channel_post": msg}
			}
			return map[string]any{"message": msg}
		}
		// Service messages (join/leave, title/photo change, TTL, voice chat…) carry
		// their data in Action; convert.Message only handles *tg.Message.
		if svc := rawMessageService(upd.Message); svc != nil {
			msg := convert.MessageService(svc, userMap)
			resolvePinnedFromCache(cache, msg)
			if isBroadcastPeer(svc.PeerID, broadcast) {
				msg.Chat.Type = apitypes.ChatTypeChannel
				return map[string]any{"channel_post": msg}
			}
			return map[string]any{"message": msg}
		}
	}
	if upd.EditedMessage != nil {
		if m := rawMessage(upd.EditedMessage); m != nil {
			msg := convert.Message(m)
			enrichMessage(msg, userMap, chatMap)
			cacheMessageAndReply(cache, msg, m)
			if isBroadcastPeer(m.PeerID, broadcast) {
				msg.Chat.Type = apitypes.ChatTypeChannel
				return map[string]any{"edited_channel_post": msg}
			}
			return map[string]any{"edited_message": msg}
		}
	}
	if upd.BusinessMessage != nil {
		if m := rawMessage(upd.BusinessMessage); m != nil {
			msg := convert.Message(m)
			enrichMessage(msg, userMap, chatMap)
			cacheMessageAndReply(cache, msg, m)
			return map[string]any{"business_message": msg}
		}
	}
	if upd.EditedBusinessMessage != nil {
		if m := rawMessage(upd.EditedBusinessMessage); m != nil {
			msg := convert.Message(m)
			enrichMessage(msg, userMap, chatMap)
			cacheMessageAndReply(cache, msg, m)
			return map[string]any{"edited_business_message": msg}
		}
	}
	if upd.DeletedBusinessMessages != nil {
		dm := upd.DeletedBusinessMessages
		ids := make([]int64, 0, len(dm.Messages))
		for _, id := range dm.Messages {
			ids = append(ids, int64(id))
		}
		return map[string]any{"deleted_business_messages": map[string]any{
			"chat":        apitypes.Chat{ID: dm.ChatID, Type: chatTypeFromID(dm.ChatID)},
			"message_ids": ids,
		}}
	}
	if upd.CallbackQuery != nil {
		cq := upd.CallbackQuery
		out := &apitypes.CallbackQuery{
			ID:            strconv.FormatInt(cq.ID, 10),
			From:          enrichedUser(cq.UserID, userMap),
			ChatInstance:  strconv.FormatInt(cq.ChatInstance, 10),
			Data:          string(cq.Data),
			GameShortName: cq.GameShortName,
		}
		if !cq.InlineMessage {
			// Minimal message so clients can edit the originating message
			// (grammY uses callbackQuery.message for editMessageText).
			out.Message = &apitypes.Message{
				MessageID: int64(cq.MessageID),
				Chat:      apitypes.Chat{ID: cq.ChatID, Type: chatTypeFromID(cq.ChatID)},
			}
		} else {
			// Inline callback (button on an inline-sent message): encode the
			// InputBotInlineMessageID into the Bot API inline_message_id string
			// so clients can editMessageText via inline_message_id.
			out.InlineMessageID = convert.InlineMessageIDFromTL(cq.InlineMessageID)
		}
		return map[string]any{"callback_query": out}
	}
	if upd.InlineQuery != nil {
		iq := upd.InlineQuery
		return map[string]any{"inline_query": &apitypes.InlineQuery{
			ID:       strconv.FormatInt(iq.ID, 10),
			From:     enrichedUser(iq.UserID, userMap),
			Location: convert.GeoPointLocation(iq.Geo),
			ChatType: convert.InlineQueryChatType(iq.PeerType),
			Query:    iq.Query,
			Offset:   iq.Offset,
		}}
	}
	if upd.ChosenInlineResult != nil {
		cir := upd.ChosenInlineResult
		return map[string]any{"chosen_inline_result": &apitypes.ChosenInlineResult{
			ResultID:        cir.ResultID,
			From:            enrichedUser(cir.UserID, userMap),
			Query:           cir.Query,
			InlineMessageID: convert.InlineMessageIDFromTL(cir.MsgID),
		}}
	}
	if upd.ChatJoinRequest != nil {
		r := upd.ChatJoinRequest
		obj := map[string]any{
			"chat":         mtgoChat(r.Chat),
			"from":         mtgoUser(r.FromUser),
			"user_chat_id": r.FromUser.ID,
			"date":         r.Date.Unix(),
		}
		if r.Bio != "" {
			obj["bio"] = r.Bio
		}
		return map[string]any{"chat_join_request": obj}
	}
	if upd.ChatMember != nil {
		cm := upd.ChatMember
		cmType := apitypes.ChatTypeSupergroup
		if cm.Chat != nil {
			cmType = mtgoChatType(cm.Chat.Type)
		}
		obj := map[string]any{
			"chat":            mtgoChat(cm.Chat),
			"from":            mtgoUser(cm.FromUser),
			"date":            cm.Date.Unix(),
			"old_chat_member": convertMtgoChatMember(cm.OldChatMember, cmType),
			"new_chat_member": convertMtgoChatMember(cm.NewChatMember, cmType),
		}
		// my_chat_member = the bot's OWN status changed (target is the bot);
		// chat_member = another user's status changed (bot must be an admin).
		key := "chat_member"
		if cm.NewChatMember != nil && cm.NewChatMember.User != nil && cm.NewChatMember.User.ID == botSelfID {
			key = "my_chat_member"
		}
		return map[string]any{key: obj}
	}
	if upd.ShippingQuery != nil {
		sq := upd.ShippingQuery
		return map[string]any{"shipping_query": map[string]any{
			"id":               strconv.FormatInt(sq.ID, 10),
			"from":             mtgoUser(sq.FromUser),
			"invoice_payload":  sq.InvoicePayload,
			"shipping_address": mtgoShippingAddress(sq.Address),
		}}
	}
	if upd.PreCheckoutQuery != nil {
		pcq := upd.PreCheckoutQuery
		obj := map[string]any{
			"id":              strconv.FormatInt(pcq.ID, 10),
			"from":            mtgoUser(pcq.FromUser),
			"currency":        pcq.Currency,
			"total_amount":    pcq.TotalAmount,
			"invoice_payload": pcq.InvoicePayload,
		}
		if pcq.ShippingOptionID != "" {
			obj["shipping_option_id"] = pcq.ShippingOptionID
		}
		if pcq.OrderInfo != nil {
			obj["order_info"] = mtgoOrderInfo(pcq.OrderInfo)
		}
		return map[string]any{"pre_checkout_query": obj}
	}
	if upd.MessageReaction != nil {
		mr := upd.MessageReaction
		return map[string]any{"message_reaction": map[string]any{
			"chat":         apitypes.Chat{ID: mr.ChatID, Type: chatTypeFromID(mr.ChatID)},
			"message_id":   mr.MessageID,
			"user":         enrichedUser(mr.UserID, userMap),
			"date":         mr.Date,
			"old_reaction": convertReactions(mr.OldReactions),
			"new_reaction": convertReactions(mr.NewReactions),
		}}
	}
	if upd.MessageReactionCount != nil {
		mrc := upd.MessageReactionCount
		reactions := make([]map[string]any, 0, len(mrc.Reactions))
		for _, r := range mrc.Reactions {
			rt := reactionType(r)
			entry := map[string]any{"type": rt, "total_count": r.Count}
			reactions = append(reactions, entry)
		}
		return map[string]any{"message_reaction_count": map[string]any{
			"chat":       apitypes.Chat{ID: mrc.ChatID, Type: chatTypeFromID(mrc.ChatID)},
			"message_id": mrc.MessageID,
			"date":       mrc.Date,
			"reactions":  reactions,
		}}
	}
	if upd.Poll != nil {
		return map[string]any{"poll": convertPollUpdate(upd.Poll)}
	}
	if upd.PollAnswer != nil {
		pa := upd.PollAnswer
		return map[string]any{"poll_answer": map[string]any{
			"poll_id":    strconv.FormatInt(pa.PollID, 10),
			"user":       enrichedUser(pa.UserID, userMap),
			"option_ids": pollOptionIDs(pa.Options),
		}}
	}
	if upd.BusinessConnection != nil {
		bc := upd.BusinessConnection
		obj := map[string]any{
			"id":           bc.ID,
			"user":         mtgoUser(bc.User),
			"user_chat_id": userIDFromUser(bc.User),
			"date":         bc.Date.Unix(),
			"is_enabled":   bc.IsEnabled,
			"can_reply":    true,
		}
		return map[string]any{"business_connection": obj}
	}
	if upd.ChatBoost != nil {
		cb := upd.ChatBoost
		obj := map[string]any{
			"chat": mtgoChat(cb.Chat),
		}
		if cb.Boost != nil {
			boost := map[string]any{
				"boost_id":        cb.Boost.ID,
				"add_date":        cb.Boost.Date.Unix(),
				"expiration_date": cb.Boost.ExpireDate.Unix(),
			}
			// Determine source type from boost flags.
			switch {
			case cb.Boost.IsGiveaway:
				src := map[string]any{"type": "giveaway"}
				if cb.Boost.GiveawayMessageID != 0 {
					src["giveaway_message_id"] = cb.Boost.GiveawayMessageID
				}
				if cb.Boost.IsUnclaimed {
					src["user_is_unclaimed"] = true
				}
				boost["source"] = src
			case cb.Boost.IsGift:
				boost["source"] = map[string]any{"type": "gift_code"}
			default:
				boost["source"] = map[string]any{"type": "premium"}
			}
			obj["boost"] = boost
		}
		return map[string]any{"chat_boost": obj}
	}
	return nil
}

// cacheMessageAndReply records msg in the seen-message cache and, when the
// replied-to message has already been seen, populates msg.ReplyToMessage with
// its full content (mirrors the reference's get_reply_to_message_info). The
// reply is resolved against the cache of PRIOR messages before msg itself is
// stored. No-op when cache is nil.
func cacheMessageAndReply(cache *msgCache, msg *apitypes.Message, raw *tg.Message) {
	if cache == nil || msg == nil {
		return
	}
	if raw != nil {
		if rh, ok := raw.ReplyTo.(*tg.MessageReplyHeader); ok && rh.ReplyToMsgID != 0 {
			if replied, ok := cache.get(msg.Chat.ID, int64(rh.ReplyToMsgID)); ok {
				msg.ReplyToMessage = replied
			}
		}
	}
	cache.put(msg.Chat.ID, msg.MessageID, msg, mediaIDsFromMessage(raw))
}

// mediaIDsFromMessage returns the downloadable media ids (document id for a
// document, photo id for a photo) carried by a raw message, so they can be
// indexed for later file_reference refresh. Other media (contact, geo, poll,
// …) carry no downloadable file and yield nothing.
func mediaIDsFromMessage(m *tg.Message) []int64 {
	if m == nil || m.Media == nil {
		return nil
	}
	switch media := m.Media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := media.Document.(*tg.Document); ok && doc.ID != 0 {
			return []int64{doc.ID}
		}
	case *tg.MessageMediaPhoto:
		if ph, ok := media.Photo.(*tg.Photo); ok && ph.ID != 0 {
			return []int64{ph.ID}
		}
	}
	return nil
}

// resolvePinnedFromCache replaces a pin service message's inaccessible
// pinned_message fallback (emitted by convert.MessageService, carrying only the
// pinned message id + chat) with the full message when the bot has seen it.
// Mirrors reference Client.cpp:1760/5005 (get_message). No-op when cache is nil
// or there is no pinned_message.
func resolvePinnedFromCache(cache *msgCache, msg *apitypes.Message) {
	if cache == nil || msg == nil || msg.PinnedMessage == nil {
		return
	}
	if pinned, ok := cache.get(msg.Chat.ID, msg.PinnedMessage.MessageID); ok {
		msg.PinnedMessage = pinned
	}
}

// userIDFromUser safely extracts the ID from a *mtgotypes.User (0 if nil).
func userIDFromUser(u *mtgotypes.User) int64 {
	if u == nil {
		return 0
	}
	return u.ID
}

// convertPollUpdate builds a Bot API Poll from a mtgo PollUpdated (tg.Poll +
// tg.PollResults). Merges per-option voter counts from PollAnswerVoters by
// matching option byte identifiers.
func convertPollUpdate(pu *mtgotypes.PollUpdated) map[string]any {
	poll := map[string]any{
		"id": strconv.FormatInt(pu.PollID, 10),
	}

	var optionBytes [][]byte // parallel to poll["options"], for results matching

	if pu.Poll != nil {
		p := pu.Poll
		if p.Question != nil {
			poll["question"] = p.Question.Text
		}
		opts := make([]map[string]any, 0, len(p.Answers))
		for _, a := range p.Answers {
			if pa, ok := a.(*tg.PollAnswer); ok {
				opt := map[string]any{"text": "", "voter_count": 0}
				if pa.Text != nil {
					opt["text"] = pa.Text.Text
				}
				opts = append(opts, opt)
				optionBytes = append(optionBytes, pa.Option)
			}
		}
		poll["options"] = opts
		poll["is_closed"] = p.Closed
		poll["is_anonymous"] = !p.PublicVoters
		poll["type"] = "regular"
		if p.Quiz {
			poll["type"] = "quiz"
		}
		poll["allows_multiple_answers"] = p.MultipleChoice
	}

	// Merge results (vote counts) if present.
	if pu.Results != nil {
		r := pu.Results
		poll["total_voter_count"] = r.TotalVoters
		if r.Results != nil && optionBytes != nil {
			// Build option_bytes → voter_count lookup from PollAnswerVoters.
			voteCounts := make(map[string]int, len(r.Results))
			for _, av := range r.Results {
				voteCounts[string(av.Option)] = int(av.Voters)
			}
			// Apply voter counts to the matching options.
			if opts, ok := poll["options"].([]map[string]any); ok {
				for i, opt := range opts {
					if i < len(optionBytes) {
						if count, found := voteCounts[string(optionBytes[i])]; found {
							opt["voter_count"] = count
						}
					}
				}
			}
		}
	}

	return poll
}

// pollOptionIDs converts raw option byte identifiers to integer indices.
// Without the poll definition we can't map bytes→indices reliably; this returns
// the byte values as indices (best-effort). When the full poll is cached, a
// proper lookup should replace this.
func pollOptionIDs(options [][]byte) []int {
	ids := make([]int, 0, len(options))
	for _, opt := range options {
		if len(opt) > 0 {
			ids = append(ids, int(opt[0]))
		}
	}
	return ids
}

// convertReactions converts a slice of mtgo Reaction into Bot API ReactionType
// objects ({type:"emoji",emoji} / {type:"custom_emoji",custom_emoji_id} /
// {type:"paid"}).
func convertReactions(reactions []mtgotypes.Reaction) []apitypes.ReactionType {
	out := make([]apitypes.ReactionType, 0, len(reactions))
	for _, r := range reactions {
		out = append(out, reactionType(r))
	}
	return out
}

// reactionType converts a single mtgo Reaction to a Bot API ReactionType.
func reactionType(r mtgotypes.Reaction) apitypes.ReactionType {
	switch {
	case r.CustomEmojiID != "":
		return apitypes.ReactionType{Type: "custom_emoji", CustomEmojiID: r.CustomEmojiID}
	case r.IsPaid:
		return apitypes.ReactionType{Type: "paid"}
	default:
		return apitypes.ReactionType{Type: "emoji", Emoji: r.Emoji}
	}
}

// convertMtgoChatMember converts a mtgo *telegram/types.ChatMember into the
// Bot API ChatMember shape. Status enum is mapped (owner->creator,
// banned->kicked); admin Can* booleans come from Privileges, restricted perms
// from Permissions. chatType drives the MarshalJSON rights/permissions gating.
func convertMtgoChatMember(m *mtgotypes.ChatMember, chatType apitypes.ChatType) *apitypes.ChatMember {
	if m == nil {
		return nil
	}
	out := &apitypes.ChatMember{
		User:        mtgoUser(m.User),
		CanBeEdited: m.CanBeEdited,
	}
	out.SetChatType(chatType)
	// The reference reads member_->tag_ for BOTH "custom_title" (creator/admin)
	// and "tag" (member/restricted); mtgo surfaces that value as m.CustomTitle
	// (from the TL rank field). MarshalJSON places it per status; until_date
	// (member/restricted/kicked) and is_member (restricted) are filled here.
	switch m.Status {
	case mtgotypes.ChatMemberStatusOwner:
		out.Status = "creator"
		out.CustomTitle = m.CustomTitle
	case mtgotypes.ChatMemberStatusAdministrator:
		out.Status = "administrator"
		out.CustomTitle = m.CustomTitle
	case mtgotypes.ChatMemberStatusMember:
		out.Status = "member"
		out.Tag = m.CustomTitle
		if !m.UntilDate.IsZero() {
			out.UntilDate = m.UntilDate.Unix()
		}
	case mtgotypes.ChatMemberStatusRestricted:
		out.Status = "restricted"
		out.Tag = m.CustomTitle
		out.IsMember = m.IsMember
		if !m.UntilDate.IsZero() {
			out.UntilDate = m.UntilDate.Unix()
		}
	case mtgotypes.ChatMemberStatusLeft:
		out.Status = "left"
	case mtgotypes.ChatMemberStatusBanned:
		out.Status = "kicked"
		if !m.UntilDate.IsZero() {
			out.UntilDate = m.UntilDate.Unix()
		}
	default:
		out.Status = string(m.Status)
	}
	if m.Privileges != nil {
		p := m.Privileges
		out.IsAnonymous = p.IsAnonymous
		out.CanManageChat = p.CanManageChat
		out.CanChangeInfo = p.CanChangeInfo
		out.CanPostMessages = p.CanPostMessages
		out.CanEditMessages = p.CanEditMessages
		out.CanDeleteMessages = p.CanDeleteMessages
		out.CanInviteUsers = p.CanInviteUsers
		out.CanRestrictMembers = p.CanRestrictMembers
		out.CanPinMessages = p.CanPinMessages
		out.CanManageTopics = p.CanManageTopics
		out.CanPromoteMembers = p.CanPromoteMembers
		out.CanManageVideoChats = p.CanManageVideoChats
		out.CanManageVoiceChats = p.CanManageVideoChats
		out.CanPostStories = p.CanPostStories
		out.CanEditStories = p.CanEditStories
	}
	// Restricted permissions (json_store_permissions, Client.cpp:17492). mtgo
	// exposes them on m.Permissions; can_send_media_messages is the OR of the six
	// media flags. MarshalJSON emits them unconditionally in order.
	if m.Permissions != nil {
		p := m.Permissions
		out.CanSendMessages = p.CanSendMessages
		out.CanSendAudios = p.CanSendAudios
		out.CanSendDocuments = p.CanSendDocuments
		out.CanSendPhotos = p.CanSendPhotos
		out.CanSendVideos = p.CanSendVideos
		out.CanSendVideoNotes = p.CanSendVideoNotes
		out.CanSendVoiceNotes = p.CanSendVoiceNotes
		out.CanSendPolls = p.CanSendPolls
		out.CanSendOtherMessages = p.CanSendOtherMessages
		out.CanAddWebPagePreviews = p.CanAddWebPagePreviews
		out.CanReactToMessages = p.CanReactToMessages
		out.CanEditTag = p.CanEditTag
		out.CanChangeInfo = p.CanChangeInfo
		out.CanInviteUsers = p.CanInviteUsers
		out.CanPinMessages = p.CanPinMessages
		out.CanManageTopics = p.CanManageTopics
		out.CanSendMediaMessages = p.CanSendAudios || p.CanSendDocuments || p.CanSendPhotos ||
			p.CanSendVideos || p.CanSendVideoNotes || p.CanSendVoiceNotes
	}
	return out
}

// mtgoChat converts a mtgo *telegram/types.Chat into a Bot API Chat value.
func mtgoChat(c *mtgotypes.Chat) apitypes.Chat {
	if c == nil {
		return apitypes.Chat{}
	}
	ch := apitypes.Chat{ID: c.ID, Type: mtgoChatType(c.Type)}
	if ch.Type == apitypes.ChatTypePrivate {
		ch.FirstName, ch.LastName, ch.Username = c.FirstName, c.LastName, c.Username
	} else {
		ch.Title, ch.Username = c.Title, c.Username
	}
	return ch
}

// mtgoChatType maps a mtgo ChatType to the Bot API ChatType.
func mtgoChatType(t mtgotypes.ChatType) apitypes.ChatType {
	switch t {
	case mtgotypes.ChatTypePrivate, mtgotypes.ChatTypeBot:
		return apitypes.ChatTypePrivate
	case mtgotypes.ChatTypeGroup:
		return apitypes.ChatTypeGroup
	case mtgotypes.ChatTypeChannel:
		return apitypes.ChatTypeChannel
	default:
		return apitypes.ChatTypeSupergroup
	}
}

// mtgoUser converts a mtgo *telegram/types.User into a Bot API *User.
func mtgoUser(u *mtgotypes.User) *apitypes.User {
	if u == nil {
		return nil
	}
	return &apitypes.User{
		ID:        u.ID,
		IsBot:     u.IsBot,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Username:  u.Username,
	}
}

// mtgoShippingAddress converts a mtgo *ShippingAddress into a Bot API map.
func mtgoShippingAddress(a *mtgotypes.ShippingAddress) map[string]any {
	if a == nil {
		return nil
	}
	return map[string]any{
		"country_code": a.CountryCode,
		"state":        a.State,
		"city":         a.City,
		"street_line1": a.StreetLine1,
		"street_line2": a.StreetLine2,
		"post_code":    a.PostCode,
	}
}

// mtgoOrderInfo converts a mtgo *OrderInfo into a Bot API map.
func mtgoOrderInfo(o *mtgotypes.OrderInfo) map[string]any {
	if o == nil {
		return nil
	}
	m := map[string]any{}
	if o.Name != "" {
		m["name"] = o.Name
	}
	if o.Phone != "" {
		m["phone_number"] = o.Phone
	}
	if o.Email != "" {
		m["email"] = o.Email
	}
	if o.ShippingAddress != nil {
		m["shipping_address"] = mtgoShippingAddress(o.ShippingAddress)
	}
	return m
}

// enrichedUser builds a *User from an ID, filling name/username from the
// update's user map when available.
func enrichedUser(id int64, userMap map[int64]*tg.User) *apitypes.User {
	u := &apitypes.User{ID: id}
	if src, ok := userMap[id]; ok {
		u.FirstName = src.FirstName
		u.LastName = src.LastName
		u.Username = src.Username
		u.IsBot = src.Bot
	}
	return u
}

// chatTypeFromID returns a best-guess Bot API chat type from a chat ID, used
// when only the ID is known (e.g. a callback query's originating chat).
func chatTypeFromID(id int64) apitypes.ChatType {
	switch {
	case id > 0:
		return apitypes.ChatTypePrivate
	case id <= -1000000000000:
		return apitypes.ChatTypeSupergroup
	default:
		return apitypes.ChatTypeGroup
	}
}

// enrichMessage fills in From (full User) and Chat (first_name/username) using
// the user data carried by the update.
func enrichMessage(msg *apitypes.Message, userMap map[int64]*tg.User, chatMap map[int64]apitypes.Chat) {
	if msg == nil {
		return
	}

	// Enrich From with full user data.
	if msg.From != nil && msg.From.ID != 0 {
		if u, ok := userMap[msg.From.ID]; ok {
			msg.From.FirstName = u.FirstName
			msg.From.LastName = u.LastName
			msg.From.Username = u.Username
		}
		// Strip bot-capability fields (can_join_groups etc.) — those only
		// belong on getMe, not a message sender. Keeps only user-info fields.
		msg.From = apitypes.UserForFrom(msg.From)
	} else if msg.Chat.Type == "private" && msg.Chat.ID > 0 {
		if u, ok := userMap[msg.Chat.ID]; ok {
			msg.From = &apitypes.User{
				ID:        u.ID,
				IsBot:     u.Bot,
				FirstName: u.FirstName,
				LastName:  u.LastName,
				Username:  u.Username,
			}
		}
	}

	// Enrich Chat for private chats — copy user fields.
	if msg.Chat.Type == "private" && msg.Chat.ID > 0 {
		if u, ok := userMap[msg.Chat.ID]; ok {
			msg.Chat.FirstName = u.FirstName
			msg.Chat.LastName = u.LastName
			msg.Chat.Username = u.Username
		}
	}

	// Enrich Chat for channel/group chats — set title/username/is_forum from the
	// update's chat data (peerToChat only sets id+type).
	if msg.Chat.ID <= 0 {
		if c, ok := chatMap[msg.Chat.ID]; ok {
			if c.Title != "" {
				msg.Chat.Title = c.Title
			}
			if c.Username != "" {
				msg.Chat.Username = c.Username
			}
			if c.IsForum {
				msg.Chat.IsForum = true
			}
		}
	}
}

// rawMessage extracts a *tg.Message from a mtgo types.Message (its Raw is a
// tg.MessageClass, either *tg.Message or *tg.MessageService).
func rawMessage(m *mtgotypes.Message) *tg.Message {
	if m == nil || m.Raw == nil {
		return nil
	}
	if msg, ok := m.Raw.(*tg.Message); ok {
		return msg
	}
	return nil
}

// rawMessageService extracts a *tg.MessageService from a mtgo types.Message,
// returning nil for regular *tg.Message updates.
func rawMessageService(m *mtgotypes.Message) *tg.MessageService {
	if m == nil || m.Raw == nil {
		return nil
	}
	if svc, ok := m.Raw.(*tg.MessageService); ok {
		return svc
	}
	return nil
}

// broadcastChannels returns the set of raw channel IDs carried by the update
// that are broadcast channels (Channel.Broadcast=true, i.e. NOT supergroups).
// Posts in these channels are delivered as Bot API "channel_post" updates, which
// are distinct from "message" (the latter covers private/group/supergroup chats).
func broadcastChannels(chats map[int64]*mtgotypes.Chat) map[int64]bool {
	out := make(map[int64]bool)
	for _, ch := range chats {
		if ch == nil {
			continue
		}
		if tch, ok := ch.Raw.(*tg.Channel); ok && tch.Broadcast {
			out[tch.ID] = true
		}
	}
	return out
}

// isBroadcastPeer reports whether peerID refers to one of the broadcast channels
// in the set — i.e. the owning message is a channel post, not a supergroup msg.
func isBroadcastPeer(peerID tg.PeerClass, broadcast map[int64]bool) bool {
	pc, ok := peerID.(*tg.PeerChannel)
	if !ok {
		return false
	}
	return broadcast[pc.ChannelID]
}

// pushUpdate serialises a Bot API Update object and pushes it onto this bot's
// queue with the given TTL (seconds from now). The queue-assigned EventID is
// also stored as update_id, so getUpdates/webhook can return the payload without
// reparsing or rewriting JSON.
func (c *Client) pushUpdate(obj map[string]any, updateType string, ttl int32) {
	if c.params.TQueue == nil {
		return
	}
	expires := int32(time.Now().Unix()) + ttl
	if _, err := c.params.TQueue.PushWithData(context.Background(), c.queueID(), expires, 0, 0, func(id tqueue.EventID) ([]byte, string, error) {
		obj["update_id"] = int64(id)
		b, err := json.Marshal(obj)
		delete(obj, "update_id")
		if err != nil {
			return nil, "", err
		}
		return b, updateType, nil
	}); err != nil {
		botlog.Error("push update failed: %v", err)
	}
}

// pushUpdateObj JSON-encodes an Update-shaped map and pushes it. The TTL is
// per-update-type, mirroring the reference add_update timeout table
// (Client.cpp:17788-18090); updates older than 24h (messages, reactions, member
// changes) are dropped instead of enqueued (Client.cpp:18005 if (left_time > 0)).
func (c *Client) pushUpdateObj(obj map[string]any) {
	if obj == nil {
		return
	}
	updateType := updateTypeKeyObj(obj)
	if !c.updateAllowed(updateType) {
		return // disallowed by allowed_updates — skip at push (Client.cpp:17706)
	}
	ttl, ok := updateExpirySeconds(obj)
	if !ok {
		return // stale (>24h old) — drop, matching the reference
	}
	c.pushUpdate(obj, updateType, ttl)
}

// updateExpirySeconds returns the TTL (seconds) for an Update-shaped object per
// the reference add_update timeout table (Client.cpp:17788-18090), or (0, false)
// if the update is stale (>24h old) and must be dropped. Date-based types
// (messages/reactions/member/join/boost) expire 24h after their own date.
func updateExpirySeconds(obj map[string]any) (int32, bool) {
	typ := updateTypeKeyObj(obj)
	now := int32(time.Now().Unix())
	switch typ {
	case "poll", "poll_answer":
		return 86400, true
	case "inline_query":
		return 30, true
	case "chosen_inline_result":
		return 600, true
	case "callback_query", "shipping_query", "pre_checkout_query":
		return 150, true
	case "message", "edited_message", "channel_post", "edited_channel_post",
		"chat_member", "my_chat_member", "chat_join_request",
		"chat_boost", "removed_chat_boost",
		"message_reaction", "message_reaction_count":
		if d := objDate(obj, typ); d > 0 {
			left := d + 86400 - now
			if left <= 0 {
				return 0, false // stale-drop
			}
			return left, true
		}
		return 86400, true
	default: // business_connection, business_messages_deleted, purchased_paid_media,
		// managed_bot, custom_event (600), etc. — safe 24h default.
		return 86400, true
	}
}

// updateTypeKeyObj returns the Update's type key (the key that isn't "update_id").
func updateTypeKeyObj(obj map[string]any) string {
	for k := range obj {
		if k != "update_id" {
			return k
		}
	}
	return ""
}

// objDate extracts the "date" field (unix seconds) of an Update's payload, or 0.
func objDate(obj map[string]any, typ string) int32 {
	sub, ok := obj[typ].(map[string]any)
	if !ok {
		return 0
	}
	switch d := sub["date"].(type) {
	case float64:
		return int32(d)
	case int:
		return int32(d)
	case int64:
		return int32(d)
	}
	return 0
}
