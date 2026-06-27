package client

import (
	"context"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// botMessage converts a raw sent *tg.Message to the Bot API Message and enriches
// From (the bot's own identity), private-chat details, and channel/group chat
// metadata, matching the reference's full User/Chat output. Use for every
// send/edit/forward result: the send RPC returns only IDs, and the reference
// reconstructs the full sender/chat from the bot's cached identity, the peer
// cache, and the Updates.Chats carried by the RPC response.
//
// chats is the channel/group map built from the RPC's Updates.Chats via
// extractChats (may be nil); it carries the title/username/type/is_forum that
// peerToChat cannot derive from a bare PeerClass.
func (c *Client) botMessage(ctx context.Context, msg *tg.Message, chats map[int64]apitypes.Chat) *apitypes.Message {
	out := convert.Message(msg)
	c.enrichSentMessage(ctx, out, chats)
	return out
}

// enrichSentMessage fills From (the bot's own identity), private-chat
// first_name/username, and channel/group chat title/username/type/is_forum on a
// converted outgoing message. It is a no-op when the bot identity or peer is
// not yet cached. Mirrors the reference, which always emits the full sender
// User, the recipient's name/username in private chats, and full chat metadata
// for channels/groups.
func (c *Client) enrichSentMessage(ctx context.Context, out *apitypes.Message, chats map[int64]apitypes.Chat) {
	if out == nil {
		return
	}
	c.mu.Lock()
	me := c.me
	c.mu.Unlock()

	// From: an outgoing message's sender is the bot itself.
	if out.From != nil && out.From.ID != 0 && me != nil && out.From.ID == me.ID {
		out.From = apitypes.UserForFrom(me)
	}
	// Forward sender: when the original sender is the bot itself (e.g. forwarding
	// our own message), fill the full identity on both forward_origin.sender_user
	// and the legacy forward_from. Other senders stay as ID stubs (their full
	// identity would need the forward result's Users map).
	if me != nil {
		if out.ForwardOrigin != nil && out.ForwardOrigin.SenderUser != nil &&
			out.ForwardOrigin.SenderUser.ID == me.ID {
			out.ForwardOrigin.SenderUser = apitypes.UserForFrom(me)
		}
		if out.ForwardFrom != nil && out.ForwardFrom.ID == me.ID {
			out.ForwardFrom = apitypes.UserForFrom(me)
		}
	}
	// Chat: private-chat details from the peer cache.
	if out.Chat.Type == apitypes.ChatTypePrivate && out.Chat.ID > 0 && c.store != nil {
		if p, err := c.store.GetPeer(ctx, out.Chat.ID); err == nil {
			if p.Username != "" {
				out.Chat.Username = p.Username
			}
			if p.FirstName != "" {
				out.Chat.FirstName = p.FirstName
			}
		}
	}
	// Chat: channel/group metadata from the RPC's Updates.Chats. peerToChat only
	// knows id+type-supergroup (it cannot see the channel's Megagroup/Broadcast
	// flag or title/username); the Updates result carries the full channel, so we
	// correct the type and fill the metadata here.
	if out.Chat.ID <= 0 && chats != nil {
		ch, ok := chats[out.Chat.ID]
		if !ok {
			return
		}
		out.Chat.Type = ch.Type
		if ch.Title != "" {
			out.Chat.Title = ch.Title
		}
		if ch.Username != "" {
			out.Chat.Username = ch.Username
		}
		if ch.IsForum {
			out.Chat.IsForum = true
		}
		// Broadcast-channel posts have no individual sender — MTProto omits
		// from_id, so From is nil. The channel itself is the sender: set
		// sender_chat to the channel (mirrors the reference). Supergroups and
		// non-broadcast cases keep From (already set when the bot posted under
		// its own identity) and skip sender_chat.
		if out.From == nil && ch.Type == apitypes.ChatTypeChannel {
			sender := ch
			out.SenderChat = &sender
		}
	}
}

// extractChats builds a Bot API chat map (keyed by Bot API chat id) from the
// channels and basic chats carried by a send/edit RPC's Updates result. Used to
// enrich a sent message's Chat with title/username/type/is_forum, which
// peerToChat cannot derive from a bare PeerClass. Returns nil when the result
// carries no chats.
func extractChats(result tg.UpdatesClass) map[int64]apitypes.Chat {
	u, ok := result.(*tg.Updates)
	if !ok || len(u.Chats) == 0 {
		return nil
	}
	m := make(map[int64]apitypes.Chat, len(u.Chats))
	for _, ch := range u.Chats {
		switch c := ch.(type) {
		case *tg.Channel:
			chatType := apitypes.ChatTypeChannel
			if c.Megagroup {
				chatType = apitypes.ChatTypeSupergroup
			}
			chatID := -1000000000000 - c.ID
			m[chatID] = apitypes.Chat{
				ID:       chatID,
				Type:     chatType,
				Title:    c.Title,
				Username: c.Username,
				IsForum:  c.Forum,
			}
		case *tg.Chat:
			m[-c.ID] = apitypes.Chat{
				ID:    -c.ID,
				Type:  apitypes.ChatTypeGroup,
				Title: c.Title,
			}
		}
	}
	return m
}
