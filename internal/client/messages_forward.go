package client

import (
	"context"
	"encoding/json"
	"math"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("forwardmessage", (*Client).forwardMessage)
	Register("forwardmessages", (*Client).forwardMessages)
	Register("copymessage", (*Client).copyMessage)
	Register("copymessages", (*Client).copyMessages)
}

// forwardMessage forwards a single message (from_chat_id/message_id → chat_id).
// Returns the Bot API Message. Mirrors do_forward_messages single path.
func (c *Client) forwardMessage(ctx context.Context, q *server.Query) (any, error) {
	msgs, chats, err := c.doForwardOrCopy(ctx, q, false)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, NewError(500, "Internal Server Error: no message returned")
	}
	return c.botMessage(ctx, msgs[0], chats), nil
}

// forwardMessages forwards multiple messages. Returns []Message.
func (c *Client) forwardMessages(ctx context.Context, q *server.Query) (any, error) {
	msgs, chats, err := c.doForwardOrCopy(ctx, q, false)
	if err != nil {
		return nil, err
	}
	out := make([]*apitypes.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, c.botMessage(ctx, m, chats))
	}
	return out, nil
}

// copyMessage sends a copy (no forward header) of a single message, optionally with a
// replaced caption / caption-above-media. Implemented as forwardMessages with
// DropAuthor=true; when caption or show_caption_above_media is given, the copied message
// is then edited (forwardMessages can't carry caption/invert) — mirrors TDLib's
// inputMessageForwarded + messageCopyOptions → editMessage.
func (c *Client) copyMessage(ctx context.Context, q *server.Query) (any, error) {
	msgs, chats, err := c.doForwardOrCopy(ctx, q, true)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, NewError(500, "Internal Server Error: no message returned")
	}
	if q.HasArg("caption") || q.ArgBool("show_caption_above_media") {
		toPeer, perr := convert.ResolvePeer(ctx, q.Arg("chat_id"), c.store)
		if perr != nil {
			return nil, NewError(400, "Bad Request: "+perr.Error())
		}
		caption, entities, ferr := convert.FormattedText(q.Arg("caption"), q.Arg("parse_mode"), q.Arg("caption_entities"))
		if ferr != nil {
			return nil, NewError(400, "Bad Request: "+ferr.Error())
		}
		editReq := &tg.MessagesEditMessageRequest{
			Peer:        toPeer,
			ID:          msgs[0].ID,
			Message:     caption,
			Entities:    entities,
			InvertMedia: q.ArgBool("show_caption_above_media"),
		}
		return c.invokeEdit(ctx, editReq, "")
	}
	return c.botMessage(ctx, msgs[0], chats), nil
}

// copyMessages copies multiple messages. Returns []MessageId.
func (c *Client) copyMessages(ctx context.Context, q *server.Query) (any, error) {
	msgs, _, err := c.doForwardOrCopy(ctx, q, true)
	if err != nil {
		return nil, err
	}
	out := make([]*apitypes.MessageID, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, &apitypes.MessageID{MessageID: int64(m.ID)})
	}
	return out, nil
}

// doForwardOrCopy is the shared forward/copy implementation. copy=true sets
// DropAuthor to suppress the forward header (Bot API copy semantics).
func (c *Client) doForwardOrCopy(ctx context.Context, q *server.Query, copy bool) ([]*tg.Message, map[int64]apitypes.Chat, error) {
	fromChatID := q.Arg("from_chat_id")
	if fromChatID == "" {
		return nil, nil, NewError(400, "Bad Request: parameter \"from_chat_id\" is required")
	}
	toChatID := q.Arg("chat_id")
	if toChatID == "" {
		return nil, nil, NewError(400, "Bad Request: chat_id is empty")
	}
	ids := parseMessageIDs(q)
	if len(ids) == 0 {
		return nil, nil, NewError(400, "Bad Request: parameter \"message_id\" or \"message_ids\" is required")
	}
	if err := c.checkWriteAccess(ctx, q); err != nil {
		return nil, nil, err
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, nil, connError(err)
	}

	fromPeer, err := convert.ResolvePeer(ctx, fromChatID, c.store)
	if err != nil {
		return nil, nil, NewError(400, "Bad Request: "+err.Error())
	}
	toPeer, err := convert.ResolvePeer(ctx, toChatID, c.store)
	if err != nil {
		return nil, nil, NewError(400, "Bad Request: "+err.Error())
	}

	req := &tg.MessagesForwardMessagesRequest{
		FromPeer:   fromPeer,
		ID:         ids,
		RandomID:   randomIDs(len(ids)),
		ToPeer:     toPeer,
		DropAuthor: copy,
	}
	if q.ArgBool("disable_notification") {
		req.Silent = true
	}
	if q.ArgBool("protect_content") {
		req.Noforwards = true
	}
	if copy && q.ArgBool("remove_caption") {
		req.DropMediaCaptions = true
	}
	// Topic targeting: message_thread_id (forum) or direct_messages_topic_id (DM
	// topic). messages.forwardMessages has a top-level top_msg_id (flag 9, set by
	// SetFlags below), unlike the send RPCs which express the topic via reply_to.
	if topic := topicID(q); topic != 0 {
		req.TopMsgID = topic
	} else if dmtid := dmTopicID(q); dmtid != 0 {
		req.TopMsgID = dmtid
	}
	if eid, err := q.ArgInt64("message_effect_id"); err == nil && eid != 0 {
		req.Effect = eid
	}
	// video_start_timestamp → TL messages.forwardMessages video_timestamp (flag 20).
	// Traced via TDLib: forwardMessage replace_video_start_timestamp → forwardMessages.
	if vts := q.Arg("video_start_timestamp"); vts != "" {
		if v, err := strconv.ParseInt(vts, 10, 32); err == nil && v != 0 {
			req.VideoTimestamp = int32(v)
		}
	}
	req.SetFlags()

	result, err := c.rpc.MessagesForwardMessages(ctx, req)
	if err != nil {
		return nil, nil, c.sendRPCError(ctx, err, q)
	}
	msgs := extractMessagesFromUpdates(result)
	for _, m := range msgs {
		c.cachePeersFromMessage(ctx, m)
	}
	return msgs, extractChats(result), nil
}

// parseMessageIDs extracts message ids from message_id (single) or message_ids
// (JSON array or comma-separated list), as int32 for the tg request.
func parseMessageIDs(q *server.Query) []int32 {
	if raw := q.Arg("message_ids"); raw != "" {
		var arr []int64
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			return toInt32Slice(arr)
		}
		var out []int32
		for _, s := range splitCSV(raw) {
			if n, err := strconv.ParseInt(s, 10, 32); err == nil {
				out = append(out, int32(n))
			}
		}
		return out
	}
	if raw := q.Arg("message_id"); raw != "" {
		if n, err := strconv.ParseInt(raw, 10, 32); err == nil {
			return []int32{int32(n)}
		}
	}
	return nil
}

func toInt32Slice(in []int64) []int32 {
	out := make([]int32, 0, len(in))
	for _, v := range in {
		if v < 0 || v > math.MaxInt32 {
			continue
		}
		out = append(out, int32(v))
	}
	return out
}

func randomIDs(n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = randomID()
	}
	return out
}

func splitCSV(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' || r == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// extractMessagesFromUpdates extracts all sent *tg.Message values from a
// tg.UpdatesClass result (plural variant of extractMessageFromUpdates).
func extractMessagesFromUpdates(result tg.UpdatesClass) []*tg.Message {
	if result == nil {
		return nil
	}
	u, ok := result.(*tg.Updates)
	if !ok {
		return nil
	}
	var out []*tg.Message
	for _, upd := range u.Updates {
		switch vu := upd.(type) {
		case *tg.UpdateNewMessage:
			if m, ok := vu.Message.(*tg.Message); ok {
				out = append(out, m)
			}
		case *tg.UpdateNewChannelMessage:
			if m, ok := vu.Message.(*tg.Message); ok {
				out = append(out, m)
			}
		case *tg.UpdateNewScheduledMessage:
			if m, ok := vu.Message.(*tg.Message); ok {
				out = append(out, m)
			}
		}
	}
	return out
}
