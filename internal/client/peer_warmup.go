package client

import (
	"context"

	"github.com/mtgo-labs/mtgo/tg"

	botlog "github.com/mtgo-labs/mtgo-bot-api/internal/log"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

// warmPeerCache fetches the bot's dialogs and caches every user/chat/channel
// access hash so the first outbound call to a chat the bot is a member of — but
// has never received an update from — doesn't fail with PEER_ID_INVALID /
// CHANNEL_PRIVATE on a cold cache. Mirrors TDLib seeding its dialog cache from
// messages.getDialogs. Best-effort: a failure only leaves the cache cold (live
// updates and on-demand resolution still populate it). Pagination is a
// refinement — a bot's dialog set is usually small enough for one call.
func (c *Client) warmPeerCache(ctx context.Context) {
	res, err := c.rpc.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      200,
	})
	if err != nil {
		botlog.Warn("client %s: warm-up getDialogs failed: %v", c.botID, err)
		return
	}
	var users []tg.UserClass
	var chats []tg.ChatClass
	switch d := res.(type) {
	case *tg.MessagesDialogs:
		users, chats = d.Users, d.Chats
	case *tg.MessagesDialogsSlice:
		users, chats = d.Users, d.Chats
	default:
		return
	}
	for _, u := range users {
		if uu, ok := u.(*tg.User); ok {
			c.savePeer(storage.Peer{ID: uu.ID, AccessHash: uu.AccessHash, Type: storage.PeerTypeUser, Username: uu.Username, FirstName: uu.FirstName})
		}
	}
	for _, ch := range chats {
		switch raw := ch.(type) {
		case *tg.Channel:
			c.savePeer(storage.Peer{ID: raw.ID, AccessHash: raw.AccessHash, Type: storage.PeerTypeChannel})
			c.saveChatFlags(raw.ID, raw.Megagroup, false, "")
		case *tg.ChannelForbidden:
			c.savePeer(storage.Peer{ID: raw.ID, AccessHash: raw.AccessHash, Type: storage.PeerTypeChannel})
			c.saveBotMemberStatus(raw.ID, "kicked")
		case *tg.Chat:
			c.savePeer(storage.Peer{ID: raw.ID, Type: storage.PeerTypeChat})
			c.saveChatFlags(raw.ID, false, raw.Deactivated, "")
		case *tg.ChatForbidden:
			c.savePeer(storage.Peer{ID: raw.ID, Type: storage.PeerTypeChat})
			c.saveBotMemberStatus(raw.ID, "kicked")
		}
	}
	botlog.Info("client %s: peer cache warmed (%d users, %d chats)", c.botID, len(users), len(chats))
}
