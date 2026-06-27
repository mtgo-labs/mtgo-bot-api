package client

import (
	"context"

	"github.com/mtgo-labs/mtgo/tg"
	"github.com/mtgo-labs/mtgo/tgerr"

	"github.com/mtgo-labs/mtgo-bot-api/internal/fileid"
)

// Signed-dialog-id constants for peer reconstruction from a Bot API chat_id
// (= MTProto dialog_id): positive → private chat, <= -1e12 → channel/supergroup,
// otherwise basic group. Mirrors the dialog_id packing in TDLib (DialogId.cpp).
const (
	dialogChannelMark      int64 = 1000000000000 // -dialog_id = mark + channel_id
	dialogChannelThreshold int64 = -1000000000000
)

// isFileReferenceExpired reports whether err is the MTProto FILE_REFERENCE_EXPIRED
// error (rpc code 400), which signals that the file_reference in a download
// location has expired and must be refreshed from its source.
func isFileReferenceExpired(err error) bool {
	if e, ok := tgerr.As(err); ok {
		return e.Code == 400 && e.Message == "FILE_REFERENCE_EXPIRED"
	}
	return false
}

// tryRefreshFileReference best-effort refreshes an expired file_reference by
// re-fetching the message the file originated from — mirroring TDLib's
// FileSourceMessage refresh: messages.getMessages for private/basic-group
// messages, channels.getMessages for channel/supergroup messages, then reading
// the fresh file_reference off the re-fetched document/photo. It relies on the
// in-memory message index; a miss (message never seen / evicted) or a fetch
// failure yields no refresh and the caller surfaces the original error. This
// matches the official server only to the extent the source is recoverable — a
// true cold-start (file_id whose message the bot never cached) still fails,
// exactly as on the official server when TDLib has no file source. Returns
// (ref, true) only when a fresh reference was actually obtained.
func (c *Client) tryRefreshFileReference(ctx context.Context, d fileid.Decoded) ([]byte, bool) {
	chatID, msgID, ok := c.msgs.sourceByMediaID(d.ID)
	if !ok {
		return nil, false
	}
	msgs, err := c.fetchSourceMessages(ctx, chatID, msgID)
	if err != nil {
		return nil, false
	}
	if ref := extractFreshReference(msgs, msgID, d.ID); ref != nil {
		return ref, true
	}
	return nil, false
}

// fetchSourceMessages re-fetches a single message by (chatID, msgID). chatID is
// the signed Bot API dialog id: positive → private chat (messages.getMessages),
// <= -1e12 → channel/supergroup (channels.getMessages, needs the channel access
// hash from the peer cache), otherwise basic group (messages.getMessages).
func (c *Client) fetchSourceMessages(ctx context.Context, chatID, msgID int64) ([]tg.MessageClass, error) {
	in := []tg.InputMessageClass{&tg.InputMessageID{ID: int32(msgID)}}
	if chatID <= dialogChannelThreshold {
		channelID := -chatID - dialogChannelMark
		peer, err := c.store.GetPeer(ctx, channelID)
		if err != nil {
			return nil, err
		}
		res, err := c.rpc.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: channelID, AccessHash: peer.AccessHash},
			ID:      in,
		})
		if err != nil {
			return nil, err
		}
		return messagesClassToList(res), nil
	}
	res, err := c.rpc.MessagesGetMessages(ctx, &tg.MessagesGetMessagesRequest{ID: in})
	if err != nil {
		return nil, err
	}
	return messagesClassToList(res), nil
}

// messagesClassToList extracts the []MessageClass from a messages.getMessages /
// channels.getMessages result. MessagesMessagesNotModified carries none.
func messagesClassToList(res tg.MessagesClass) []tg.MessageClass {
	switch r := res.(type) {
	case *tg.MessagesMessages:
		return r.Messages
	case *tg.MessagesMessagesSlice:
		return r.Messages
	case *tg.MessagesChannelMessages:
		return r.Messages
	}
	return nil
}

// extractFreshReference finds the document/photo with mediaID in the re-fetched
// message msgID and returns its fresh file_reference (nil if absent).
func extractFreshReference(msgs []tg.MessageClass, msgID, mediaID int64) []byte {
	for _, mc := range msgs {
		m, ok := mc.(*tg.Message)
		if !ok || m.ID != int32(msgID) {
			continue
		}
		switch media := m.Media.(type) {
		case *tg.MessageMediaDocument:
			if doc, ok := media.Document.(*tg.Document); ok && doc.ID == mediaID {
				return doc.FileReference
			}
		case *tg.MessageMediaPhoto:
			if ph, ok := media.Photo.(*tg.Photo); ok && ph.ID == mediaID {
				return ph.FileReference
			}
		}
	}
	return nil
}
