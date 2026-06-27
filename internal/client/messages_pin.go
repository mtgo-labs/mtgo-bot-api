package client

import (
	"context"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("pinchatmessage", (*Client).pinChatMessage)
	Register("unpinchatmessage", (*Client).unpinChatMessage)
	Register("unpinallchatmessages", (*Client).unpinAllChatMessages)
}

// pinChatMessage pins a message in a chat. Uses raw tg.MessagesUpdatePinnedMessage
// with Unpin=false. Mirrors do_pin_message.
//
// Gotcha: the TL request field is "Unpin", not "Pinned" (see AGENTS.md).
func (c *Client) pinChatMessage(ctx context.Context, q *server.Query) (any, error) {
	peer, id, err := c.resolveTargetMessage(ctx, q)
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesUpdatePinnedMessageRequest{
		Peer:  peer,
		ID:    id,
		Unpin: false,
	}
	if q.ArgBool("disable_notification") {
		req.Silent = true
	}
	req.SetFlags()
	if connID := businessConnID(q); connID != "" {
		if _, err := c.invokeBusiness(ctx, connID, req); err != nil {
			return nil, rpcError(err)
		}
	} else if _, err := c.rpc.MessagesUpdatePinnedMessage(ctx, req); err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// unpinChatMessage unpins a single message (Unpin=true).
func (c *Client) unpinChatMessage(ctx context.Context, q *server.Query) (any, error) {
	peer, id, err := c.resolveTargetMessage(ctx, q)
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesUpdatePinnedMessageRequest{
		Peer:  peer,
		ID:    id,
		Unpin: true,
	}
	req.SetFlags()
	if connID := businessConnID(q); connID != "" {
		if _, err := c.invokeBusiness(ctx, connID, req); err != nil {
			return nil, rpcError(err)
		}
	} else if _, err := c.rpc.MessagesUpdatePinnedMessage(ctx, req); err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// unpinAllChatMessages unpins all messages in a chat via
// tg.MessagesUnpinAllMessages.
func (c *Client) unpinAllChatMessages(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	req := &tg.MessagesUnpinAllMessagesRequest{Peer: peer}
	req.SetFlags()
	if _, err := c.rpc.MessagesUnpinAllMessages(ctx, req); err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}
