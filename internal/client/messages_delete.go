package client

import (
	"context"
	"math"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("deletemessage", (*Client).deleteMessage)
	Register("deletemessages", (*Client).deleteMessages)
}

// deleteMessage implements the Bot API deleteMessage method.
// Reference: telegram-bot-api/Client.cpp process_delete_message_query.
//
// Required parameters: chat_id, message_id.
func (c *Client) deleteMessage(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		// deleteMessage uses a distinct chat_id wording (verified vs
		// api.telegram.org), unlike most methods which emit "chat_id is empty".
		return nil, NewError(400, "Bad Request: chat identifier is not specified")
	}
	msgID, err := q.ArgInt64("message_id")
	if err != nil {
		return nil, NewError(400, "Bad Request: parameter \"message_id\" is required")
	}
	if msgID > math.MaxInt32 {
		return nil, NewError(400, "Bad Request: message_id is too large")
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
	}

	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	// Determine if this is a channel or a regular chat.
	switch peer := peer.(type) {
	case *tg.InputPeerChannel:
		channel := peer
		inputCh := &tg.InputChannel{ChannelID: channel.ChannelID, AccessHash: channel.AccessHash}
		_, err = c.rpc.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: inputCh,
			ID:      []int32{int32(msgID)},
		})
	default:
		_, err = c.rpc.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
			ID: []int32{int32(msgID)},
		})
	}

	if err != nil {
		return nil, rpcError(err)
	}

	return true, nil
}

// deleteMessages implements the Bot API deleteMessages method (plural).
// Reference: telegram-bot-api/Client.cpp process_delete_messages_query.
//
// Required parameters: chat_id, message_ids (1-100 identifiers). Messages are
// always revoked (Bot API semantics).
func (c *Client) deleteMessages(ctx context.Context, q *server.Query) (any, error) {
	ids := parseMessageIDs(q)
	if len(ids) == 0 {
		return nil, NewError(400, "Bad Request: message identifiers are not specified")
	}
	if len(ids) > 100 {
		return nil, NewError(400, "Bad Request: too many message_ids (max 100)")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat identifier is not specified")
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
	}

	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	switch p := peer.(type) {
	case *tg.InputPeerChannel:
		_, err = c.rpc.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash},
			ID:      ids,
		})
	default:
		_, err = c.rpc.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
			ID:     ids,
			Revoke: true,
		})
	}

	if err != nil {
		return nil, rpcError(err)
	}

	return true, nil
}
