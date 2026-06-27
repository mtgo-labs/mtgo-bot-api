package client

import (
	"context"
	"strconv"

	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

// checkWriteAccess mirrors the official server's pre-flight check_chat_access
// (telegram-bot-api/Client.cpp:8705-8773) for Write access: if the cached peer
// state shows the bot cannot write to the chat, return the same friendly error
// the official server returns BEFORE any MTProto call.
//
// It returns nil (fall through to the RPC) whenever the bot's status is unknown
// — a chat the bot has never seen, or whose membership isn't cached. This means
// it can NEVER block a legitimate send: it only short-circuits on positive
// knowledge that the bot is kicked / has left / the chat is deleted.
func (c *Client) checkWriteAccess(ctx context.Context, q *server.Query) error {
	if c.store == nil {
		return nil
	}
	id, chatType, err := parseChatID(q.Arg("chat_id"))
	if err != nil {
		return nil // invalid/@username chat_id — let the handler validate/resolve
	}
	var storeID int64
	switch chatType {
	case "channel":
		storeID = extractChannelID(id)
	case "chat":
		storeID = -id
	default: // user
		storeID = id
	}
	p, err := c.store.GetPeer(ctx, storeID)
	if err != nil {
		return nil // not cached → fall through (cold-start safety)
	}

	switch chatType {
	case "chat":
		if p.IsDeactivated {
			if p.MigratedTo != "" {
				if mid, e := strconv.ParseInt(p.MigratedTo, 10, 64); e == nil {
					return &Error{
						Code:        400,
						Description: "Bad Request: group chat was upgraded to a supergroup chat",
						Params:      &response.Parameters{MigrateToChatID: mid},
					}
				}
			}
			return NewError(403, "Forbidden: the group chat was deleted")
		}
		switch p.BotMemberStatus {
		case "kicked":
			return NewError(403, "Forbidden: bot was kicked from the group chat")
		case "left":
			return NewError(403, "Forbidden: bot is not a member of the group chat")
		}
	case "channel":
		suffix := "channel"
		if p.IsMegagroup {
			suffix = "supergroup"
		}
		switch p.BotMemberStatus {
		case "kicked":
			return NewError(403, "Forbidden: bot was kicked from the "+suffix+" chat")
		case "left":
			return NewError(403, "Forbidden: bot is not a member of the "+suffix+" chat")
		}
	case "user":
		if p.IsDeactivated {
			return NewError(403, "Forbidden: user is deactivated")
		}
	}
	return nil
}

// chatTypeHint returns the chat-type suffix ("channel"|"supergroup"|"group") for
// the cold-start chat-access fallback, preferring the cached megagroup flag.
func (c *Client) chatTypeHint(ctx context.Context, q *server.Query) string {
	id, chatType, err := parseChatID(q.Arg("chat_id"))
	if err != nil {
		return ""
	}
	switch chatType {
	case "channel":
		if c.store != nil {
			if p, e := c.store.GetPeer(ctx, extractChannelID(id)); e == nil && p.IsMegagroup {
				return "supergroup"
			}
		}
		return "channel"
	case "chat":
		return "group"
	}
	return ""
}

// sendRpcError wraps rpcError with the cold-start chat-access fallback
// (rpcErrorWithChat) for the send paths.
func (c *Client) sendRPCError(ctx context.Context, err error, q *server.Query) *Error {
	return rpcErrorWithChat(err, c.chatTypeHint(ctx, q))
}
