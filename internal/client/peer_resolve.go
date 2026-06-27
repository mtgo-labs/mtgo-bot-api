package client

import (
	"context"
	"strconv"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

// usernamePeerArgs are the Bot API request args that hold a chat/peer and may
// be supplied as an @username. Scanning is limited to these so a @-prefixed
// non-peer value (e.g. message text) never triggers a username lookup.
var usernamePeerArgs = []string{"chat_id", "from_chat_id", "sender_chat_id"}

// warmUsernamePeers resolves any @username-valued peer arg live via
// contacts.resolveUsername and rewrites it to the numeric Bot API chat_id,
// caching the resolved peer (with its access hash). This mirrors the official
// server, which resolves @username via TDLib even for chats the bot has never
// interacted with (public channels/users/bots). Per-client (this bot's session,
// so multi-bot safe) and best-effort: an unresolved username is left untouched
// for the handler to surface as a 400. Already-cached usernames skip the RPC.
//
// Rewriting to the numeric chat_id (rather than only warming the cache) ensures
// every peer-resolution path works — both convert.ResolvePeer (which accepts
// @username) and parseChatID (which rejects it).
func (c *Client) warmUsernamePeers(ctx context.Context, q *server.Query) {
	var anyUsername bool
	for _, key := range usernamePeerArgs {
		if strings.HasPrefix(q.Args[key], "@") {
			anyUsername = true
			break
		}
	}
	if !anyUsername {
		return
	}
	if err := c.ensureConnected(ctx); err != nil {
		return // the handler will surface the connection error
	}
	for _, key := range usernamePeerArgs {
		v := q.Args[key]
		if !strings.HasPrefix(v, "@") {
			continue
		}
		username := v[1:]
		if username == "" {
			continue
		}
		p, ok := c.cachedOrResolveUsername(ctx, username)
		if !ok {
			continue // leave as @username; the handler surfaces a 400
		}
		q.Args[key] = peerBotAPIChatID(p)
	}
}

// cachedOrResolveUsername returns the cached peer for a username, resolving live
// (and caching) on a miss.
func (c *Client) cachedOrResolveUsername(ctx context.Context, username string) (storage.Peer, bool) {
	if c.store != nil {
		if p, err := c.store.GetPeerByUsername(ctx, username); err == nil {
			return p, true
		}
	}
	return c.resolveUsername(ctx, username)
}

// resolveUsername resolves a @username (without the leading @) live via
// contacts.resolveUsername, caches the resulting peer, and returns it. Returns
// ok=false if the username can't be resolved.
func (c *Client) resolveUsername(ctx context.Context, username string) (storage.Peer, bool) {
	res, err := c.rpc.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
	if err != nil {
		return storage.Peer{}, false
	}
	p, ok := peerFromResolved(res)
	if !ok {
		return storage.Peer{}, false
	}
	c.savePeer(p)
	return p, true
}

// peerFromResolved extracts the primary peer (user/channel/chat) — with its
// access hash — from a contacts.resolveUsername result.
func peerFromResolved(res *tg.ContactsResolvedPeer) (storage.Peer, bool) {
	switch peer := res.Peer.(type) {
	case *tg.PeerUser:
		for _, u := range res.Users {
			if user, ok := u.(*tg.User); ok && user.ID == peer.UserID {
				return storage.Peer{ID: user.ID, AccessHash: user.AccessHash, Type: storage.PeerTypeUser, Username: user.Username}, true
			}
		}
	case *tg.PeerChannel:
		for _, ch := range res.Chats {
			if channel, ok := ch.(*tg.Channel); ok && channel.ID == peer.ChannelID {
				return storage.Peer{ID: channel.ID, AccessHash: channel.AccessHash, Type: storage.PeerTypeChannel, Username: channel.Username}, true
			}
		}
	case *tg.PeerChat:
		for _, ch := range res.Chats {
			if chat, ok := ch.(*tg.Chat); ok && chat.ID == peer.ChatID {
				return storage.Peer{ID: chat.ID, Type: storage.PeerTypeChat}, true
			}
		}
	}
	return storage.Peer{}, false
}

// peerBotAPIChatID returns the signed Bot API chat_id string for a cached peer:
// positive for users, -id for basic groups, -1e12-id for channels/supergroups.
func peerBotAPIChatID(p storage.Peer) string {
	switch p.Type {
	case storage.PeerTypeChannel:
		return strconv.FormatInt(-1_000_000_000_000-p.ID, 10)
	case storage.PeerTypeChat:
		return strconv.FormatInt(-p.ID, 10)
	default: // user
		return strconv.FormatInt(p.ID, 10)
	}
}
