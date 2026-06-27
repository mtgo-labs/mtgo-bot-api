package client

import (
	"context"
	"crypto/rand"
	"math/big"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

func init() {
	Register("sendmessage", (*Client).sendMessage)
}

// sendMessage implements the Bot API sendMessage method.
// Reference: telegram-bot-api/Client.cpp process_send_message_query + do_send_message.
//
// Required parameters: chat_id, text.
// Optional: parse_mode, entities, link_preview_options, disable_notification,
// protect_content, message_effect_id, reply_parameters, reply_markup.
func (c *Client) sendMessage(ctx context.Context, q *server.Query) (any, error) {
	text := q.Arg("text")
	if text == "" {
		return nil, NewError(400, "Bad Request: message text is empty")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.checkWriteAccess(ctx, q); err != nil {
		return nil, err
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
	}

	// Resolve the target peer (allow_paid_broadcast force-resolves a non-starter user,
	// mirroring TDLib's force_create_dialog).
	peer, err := c.resolveBroadcastPeer(ctx, q)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	// Build the formatted text.
	message, entities, err := convert.FormattedText(text, q.Arg("parse_mode"), q.Arg("entities"))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	var result tg.UpdatesClass
	// link_preview_options with a url → attach the webpage preview as media via
	// messages.sendMedia (messages.sendMessage has no media field). Mirrors TDLib
	// MessagesManager.cpp:22417 (get_message_content_input_media_web_page → SendMediaQuery).
	if webPage := buildInputMediaWebPage(q, message); webPage != nil {
		req := &tg.MessagesSendMediaRequest{
			Peer:     peer,
			Media:    webPage,
			Message:  message,
			RandomID: randomID(),
		}
		if entities != nil {
			req.Entities = entities
		}
		if rt := buildReplyTo(q); rt != nil {
			req.ReplyTo = rt
		}
		if q.ArgBool("disable_notification") {
			req.Silent = true
		}
		if q.ArgBool("protect_content") {
			req.Noforwards = true
		}
		if rm := q.Arg("reply_markup"); rm != "" {
			markup, mErr := convert.ReplyMarkup(rm)
			if mErr != nil {
				return nil, NewError(400, "Bad Request: "+mErr.Error())
			}
			req.ReplyMarkup = markup
		}
		if eid, eErr := q.ArgInt64("message_effect_id"); eErr == nil && eid != 0 {
			req.Effect = eid
		}
		// show_above_text → invert_media (no_webpage doesn't apply — a preview is attached).
		var noWebpageDiscard bool
		applyLinkPreview(q, &noWebpageDiscard, &req.InvertMedia)
		req.SetFlags()
		result, err = c.rpc.MessagesSendMedia(ctx, req)
	} else {
		req := &tg.MessagesSendMessageRequest{
			Peer:     peer,
			Message:  message,
			RandomID: randomID(),
		}
		if entities != nil {
			req.Entities = entities
		}

		// Optional: reply_to (reply_to_message_id / reply_parameters) and forum-topic
		// targeting (message_thread_id / direct_messages_topic_id).
		if rt := buildReplyTo(q); rt != nil {
			req.ReplyTo = rt
			req.Flags.Set(0) // reply_to flag
		}

		// Optional: disable_notification.
		if q.ArgBool("disable_notification") {
			req.Silent = true
			req.Flags.Set(5)
		}

		// Optional: protect_content.
		if q.ArgBool("protect_content") {
			req.Noforwards = true
			req.Flags.Set(14)
		}

		// Optional: reply_markup (inline keyboard).
		if rm := q.Arg("reply_markup"); rm != "" {
			markup, mErr := convert.ReplyMarkup(rm)
			if mErr != nil {
				return nil, NewError(400, "Bad Request: "+mErr.Error())
			}
			req.ReplyMarkup = markup
		}

		// Optional: message_effect_id.
		if eid, eErr := q.ArgInt64("message_effect_id"); eErr == nil && eid != 0 {
			req.Effect = eid
		}

		// link_preview_options / disable_web_page_preview → no_webpage (flag 1) +
		// show_above_text → invert_media (flag 16).
		applyLinkPreview(q, &req.NoWebpage, &req.InvertMedia)

		// Set flags for non-nil optional fields.
		req.SetFlags()

		result, err = c.rpc.MessagesSendMessage(ctx, req)
	}

	if err != nil {
		return nil, c.sendRPCError(ctx, err, q)
	}
	return c.finishSend(ctx, result, peer, message)
}

// finishSend extracts the sent Message from an UpdatesClass result, enriches it,
// and converts it to Bot API format. Shared by sendMessage's text and
// link-preview (sendMedia) paths.
func (c *Client) finishSend(ctx context.Context, result tg.UpdatesClass, peer tg.InputPeerClass, message string) (any, error) {
	msg := extractMessageFromUpdates(result)
	if msg == nil {
		return nil, NewError(500, "Internal Server Error: failed to extract message from response")
	}
	// Enrich partial messages from UpdateShortSentMessage.
	c.enrichPartialMessage(msg, peer, message)
	// Cache the peer from the sent message for future lookups.
	c.cachePeersFromMessage(ctx, msg)
	// Convert to Bot API format, enriching From (bot identity), private-chat
	// details, and channel/group chat metadata to match the reference's full
	// User/Chat output.
	return c.botMessage(ctx, msg, extractChats(result)), nil
}

// randomID generates a random int64 for the MTProto random_id field.
func randomID() int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		// Fallback to a simple counter if crypto/rand fails (shouldn't happen).
		return int64(1)
	}
	return n.Int64()
}

// inputPeerToPeer converts an InputPeerClass to a PeerClass for enriching
// partial messages from UpdateShortSentMessage.
func inputPeerToPeer(p tg.InputPeerClass) tg.PeerClass {
	switch v := p.(type) {
	case *tg.InputPeerUser:
		return &tg.PeerUser{UserID: v.UserID}
	case *tg.InputPeerChat:
		return &tg.PeerChat{ChatID: v.ChatID}
	case *tg.InputPeerChannel:
		return &tg.PeerChannel{ChannelID: v.ChannelID}
	case *tg.InputPeerSelf:
		return &tg.PeerUser{} // will be filled by caller if needed
	default:
		return nil
	}
}

// extractMessageFromUpdates extracts the sent *tg.Message from a
// tg.UpdatesClass result (MessagesSendMessage returns Updates or UpdatesShort
// or similar). We look for the message in Updates, UpdateShort doesn't contain
// messages.
func extractMessageFromUpdates(result tg.UpdatesClass) *tg.Message {
	if result == nil {
		return nil
	}
	switch u := result.(type) {
	case *tg.Updates:
		for _, update := range u.Updates {
			switch upd := update.(type) {
			case *tg.UpdateNewMessage:
				if msg, ok := upd.Message.(*tg.Message); ok {
					return msg
				}
			case *tg.UpdateNewChannelMessage:
				if msg, ok := upd.Message.(*tg.Message); ok {
					return msg
				}
			case *tg.UpdateNewScheduledMessage:
				if msg, ok := upd.Message.(*tg.Message); ok {
					return msg
				}
			case *tg.UpdateEditMessage:
				if msg, ok := upd.Message.(*tg.Message); ok {
					return msg
				}
			case *tg.UpdateEditChannelMessage:
				if msg, ok := upd.Message.(*tg.Message); ok {
					return msg
				}
			}
		}
	case *tg.UpdateShortSentMessage:
		// Minimal response: construct a partial Message for the converter.
		msg := &tg.Message{
			ID:    u.ID,
			Date:  u.Date,
			Out:   u.Out,
			Media: u.Media,
		}
		if len(u.Entities) > 0 {
			msg.Entities = u.Entities
		}
		return msg
	}
	return nil
}

// enrichPartialMessage fills in missing fields on messages from
// UpdateShortSentMessage. These responses lack FromID, PeerID, and text.
func (c *Client) enrichPartialMessage(msg *tg.Message, peer tg.InputPeerClass, text string) {
	if msg == nil {
		return
	}
	if msg.FromID == nil {
		// Only inject the bot as sender for private chats: UpdateShortSentMessage
		// (the private-chat send response) lacks FromID. For channels/groups a nil
		// from_id means the message has no individual sender — e.g. a broadcast
		// channel post — so leave it nil and let the converter attribute the post
		// to the channel via sender_chat (matches the reference).
		if _, ok := peer.(*tg.InputPeerUser); ok {
			botUID, _ := strconv.ParseInt(c.botID, 10, 64)
			if botUID > 0 {
				msg.FromID = &tg.PeerUser{UserID: botUID}
			}
		}
	}
	if msg.PeerID == nil {
		msg.PeerID = inputPeerToPeer(peer)
	}
	if msg.Message == "" {
		msg.Message = text
	}
}

// cachePeersFromMessage extracts peer information from a sent message and
// caches it for future peer resolution.
func (c *Client) cachePeersFromMessage(ctx context.Context, msg *tg.Message) {
	if msg == nil || c.store == nil {
		return
	}
	// Cache the chat peer from PeerID.
	if msg.PeerID != nil {
		switch peer := msg.PeerID.(type) {
		case *tg.PeerUser:
			// We don't have the access_hash here; skip caching users from
			// sent messages (the bot already knows its own chats).
			_ = peer
		case *tg.PeerChat:
			_ = c.store.SavePeer(ctx, storagePeer(peer.ChatID, 0, "chat"))
		case *tg.PeerChannel:
			// Channels from sent messages don't carry access_hash in the
			// message itself. Skip for now.
			_ = peer
		}
	}
}

// storagePeer is a helper to build a storage.Peer.
func storagePeer(id, accessHash int64, peerType string) storage.Peer {
	return storage.Peer{
		ID:         id,
		AccessHash: accessHash,
		Type:       storage.PeerType(peerType),
	}
}

// forceResolveUser resolves and caches a user's access hash on demand, mirroring
// TDLib's force_create_dialog (the allow_paid_broadcast gate): it lets a send
// proceed to a user the bot hasn't messaged. Best-effort — on failure (e.g. the bot
// has no context for the user) the caller falls back to the hash=0 InputPeerUser,
// which Telegram resolves for users the bot has interacted with.
func (c *Client) forceResolveUser(ctx context.Context, userID int64) {
	if c.store == nil {
		return
	}
	if _, err := c.store.GetPeer(ctx, userID); err == nil {
		return // already cached
	}
	resp, err := c.rpc.UsersGetFullUser(ctx, &tg.UsersGetFullUserRequest{
		ID: &tg.InputUser{UserID: userID},
	})
	if err != nil {
		return
	}
	uuf, ok := resp.(*tg.UsersUserFull)
	if !ok {
		return
	}
	for _, u := range uuf.Users {
		if user, ok := u.(*tg.User); ok && user.ID == userID && user.AccessHash != 0 {
			_ = c.store.SavePeer(ctx, storage.Peer{
				ID:         userID,
				AccessHash: user.AccessHash,
				Type:       storage.PeerTypeUser,
				FirstName:  user.FirstName,
			})
			return
		}
	}
}

// resolveBroadcastPeer resolves the send-target peer, applying allow_paid_broadcast
// (mirrors TDLib's force_create_dialog): for a positive user_id not yet known to the
// bot, fetch and cache the access hash so the send can reach a non-starter user.
func (c *Client) resolveBroadcastPeer(ctx context.Context, q *server.Query) (tg.InputPeerClass, error) {
	chatID := q.Arg("chat_id")
	if q.ArgBool("allow_paid_broadcast") {
		if id, err := strconv.ParseInt(chatID, 10, 64); err == nil && id > 0 {
			c.forceResolveUser(ctx, id)
		}
	}
	return convert.ResolvePeer(ctx, chatID, c.store)
}
