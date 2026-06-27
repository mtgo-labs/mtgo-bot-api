// Package convert translates between raw tg (MTProto) types and Bot API
// JSON types. peer.go handles chat_id → InputPeerClass resolution.
package convert

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

// ResolvePeer converts a Bot API chat_id parameter (string) into a raw
// tg.InputPeerClass suitable for RPC calls. The chat_id can be:
//
//   - A positive integer: user ID (looked up in peer cache for access_hash)
//   - A negative integer: legacy group chat (InputPeerChat)
//   - A negative integer with -100 prefix: channel/supergroup (InputPeerChannel)
//   - A @username string: looked up in peer cache
//
// This mirrors the peer resolution logic in telegram-bot-api/Client.cpp where
// TDLib handles the mapping internally. Since we use raw tg, we must construct
// the InputPeer ourselves.
//
// The peer cache (store) is required for user access hashes and username
// lookups. If a peer is not cached, an error is returned asking the user to
// send a message to the bot first (which populates the cache via update
// ingestion).
func ResolvePeer(ctx context.Context, chatID string, store *storage.Store) (tg.InputPeerClass, error) {
	if chatID == "" {
		return nil, errors.New("chat_id is empty")
	}

	// @username lookup.
	if strings.HasPrefix(chatID, "@") {
		username := chatID[1:]
		if username == "" {
			return nil, errors.New("invalid username")
		}
		if store == nil {
			return nil, fmt.Errorf("peer @%s not found in cache", username)
		}
		p, err := store.GetPeerByUsername(ctx, username)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("peer @%s not found in cache: send a message to the bot first", username)
		}
		if err != nil {
			return nil, fmt.Errorf("peer lookup failed: %w", err)
		}
		return peerToInputPeer(p)
	}

	// Integer chat_id.
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat_id: %q is not an integer or @username", chatID)
	}

	if id == 0 {
		return nil, errors.New("chat not found")
	}

	// Negative IDs: -100xxxxx = channel/supergroup, -xxxx = legacy chat.
	if id < 0 {
		s := strconv.FormatInt(id, 10)
		if strings.HasPrefix(s, "-100") {
			channelID, _ := strconv.ParseInt(s[4:], 10, 64)
			if channelID <= 0 {
				return nil, fmt.Errorf("invalid channel chat_id: %d", id)
			}
			// Channels may have an access_hash in the peer cache; if not,
			// we can still construct InputPeerChannel with hash=0 which works
			// for some operations (e.g. sending to a channel the bot owns).
			if store != nil {
				if p, err := store.GetPeer(ctx, channelID); err == nil {
					return peerToInputPeer(p)
				}
			}
			return &tg.InputPeerChannel{ChannelID: channelID}, nil
		}
		// Legacy group chat.
		chatID := -id
		return &tg.InputPeerChat{ChatID: chatID}, nil
	}

	// Positive ID: user. Try the peer cache for access_hash; fall back to
	// hash=0 when not cached. This matches resolveInputUser's behavior in
	// chat_info.go and unblocks send operations on cold-start (Telegram
	// accepts AccessHash=0 for users the bot has previously interacted with).
	if store != nil {
		if p, err := store.GetPeer(ctx, id); err == nil {
			return peerToInputPeer(p)
		}
	}
	return &tg.InputPeerUser{UserID: id}, nil
}

// peerToInputPeer converts a cached storage.Peer into the appropriate
// tg.InputPeerClass.
func peerToInputPeer(p storage.Peer) (tg.InputPeerClass, error) {
	switch p.Type {
	case storage.PeerTypeUser:
		return &tg.InputPeerUser{UserID: p.ID, AccessHash: p.AccessHash}, nil
	case storage.PeerTypeChat:
		return &tg.InputPeerChat{ChatID: p.ID}, nil
	case storage.PeerTypeChannel:
		return &tg.InputPeerChannel{ChannelID: p.ID, AccessHash: p.AccessHash}, nil
	default:
		return nil, fmt.Errorf("unknown peer type %q for id %d", p.Type, p.ID)
	}
}
