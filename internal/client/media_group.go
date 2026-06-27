package client

import (
	"context"
	"encoding/json"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("sendmediagroup", (*Client).sendMediaGroup)
}

// sendMediaGroup sends a group of photos/videos as an album.
// Reference: Client.cpp process_send_media_group_query.
//
// Required parameters: chat_id, media (JSON array of media descriptors).
// Optional: disable_notification, protect_content, reply_to_message_id.
func (c *Client) sendMediaGroup(ctx context.Context, q *server.Query) (any, error) {
	mediaJSON := q.Arg("media")
	if mediaJSON == "" {
		return nil, NewError(400, "Bad Request: parameter \"media\" is required")
	}

	var items []inputMediaDescriptor
	if err := json.Unmarshal([]byte(mediaJSON), &items); err != nil {
		return nil, NewError(400, "Bad Request: invalid media JSON: "+err.Error())
	}
	if len(items) < 2 || len(items) > 10 {
		return nil, NewError(400, "Bad Request: media must contain 2-10 items")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.checkWriteAccess(ctx, q); err != nil {
		return nil, err
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}

	peer, err := c.resolveBroadcastPeer(ctx, q)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	multiMedia := make([]*tg.InputSingleMedia, 0, len(items))
	for _, item := range items {
		media, caption, err := c.parseMediaDescriptor(ctx, q, mediaDescriptorJSON(item))
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		sm := &tg.InputSingleMedia{
			Media:    media,
			RandomID: randomID(),
			Message:  caption,
		}
		sm.SetFlags()
		multiMedia = append(multiMedia, sm)
	}

	req := &tg.MessagesSendMultiMediaRequest{
		Peer:       peer,
		MultiMedia: multiMedia,
	}
	// reply_to_message_id / reply_parameters + topic (message_thread_id /
	// direct_messages_topic_id). messages.sendMultiMedia has no top-level topic
	// field, so the topic goes via reply_to's top_msg_id (handled in buildReplyTo).
	if rt := buildReplyTo(q); rt != nil {
		req.ReplyTo = rt
		req.Flags.Set(0)
	}
	if q.ArgBool("disable_notification") {
		req.Silent = true
		req.Flags.Set(5)
	}
	if q.ArgBool("protect_content") {
		req.Noforwards = true
		req.Flags.Set(14)
	}
	if eid, err := q.ArgInt64("message_effect_id"); err == nil && eid != 0 {
		req.Effect = eid
	}
	req.SetFlags()

	var result tg.UpdatesClass
	connID := businessConnID(q)
	if connID != "" {
		obj, err := c.invokeBusiness(ctx, connID, req)
		if err != nil {
			return nil, rpcError(err)
		}
		upd, ok := obj.(tg.UpdatesClass)
		if !ok {
			return nil, NewError(500, "Internal Server Error: unexpected media-group result")
		}
		result = upd
	} else {
		var err error
		result, err = c.rpc.MessagesSendMultiMedia(ctx, req)
		if err != nil {
			return nil, c.sendRPCError(ctx, err, q)
		}
	}

	msgs := extractMessagesFromUpdates(result)
	chats := extractChats(result)
	out := make([]*apitypes.Message, 0, len(msgs))
	for _, m := range msgs {
		c.cachePeersFromMessage(ctx, m)
		bm := c.botMessage(ctx, m, chats)
		if connID != "" {
			bm.BusinessConnectionID = connID
		}
		out = append(out, bm)
	}
	if len(out) == 0 {
		return nil, NewError(500, "Internal Server Error: no messages returned")
	}
	return out, nil
}

// mediaDescriptorJSON marshals a descriptor back to JSON for parseMediaDescriptor.
func mediaDescriptorJSON(d inputMediaDescriptor) string {
	b, _ := json.Marshal(d)
	return string(b)
}
