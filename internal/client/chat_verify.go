package client

import (
	"context"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

func init() {
	Register("verifychat", (*Client).verifyChat)
	Register("verifyuser", (*Client).verifyUser)
	Register("removechatverification", (*Client).removeChatVerification)
	Register("removeuserverification", (*Client).removeUserVerification)
}

// verifyChat implements the Bot API verifyChat method.
func (c *Client) verifyChat(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	peer, err := resolvePeerForVerify(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	req := &tg.BotsSetCustomVerificationRequest{
		Enabled: true,
		Peer:    peer,
	}
	if desc := q.Arg("custom_description"); desc != "" {
		req.CustomDescription = desc
		req.Flags.Set(2)
	}
	req.Flags.Set(1) // enabled flag

	_, err = c.rpc.BotsSetCustomVerification(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// verifyUser implements the Bot API verifyUser method.
func (c *Client) verifyUser(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}


	req := &tg.BotsSetCustomVerificationRequest{
		Enabled: true,
		Peer:    &tg.InputPeerUser{UserID: uid},
	}
	if desc := q.Arg("custom_description"); desc != "" {
		req.CustomDescription = desc
		req.Flags.Set(2)
	}
	req.Flags.Set(1)

	_, err = c.rpc.BotsSetCustomVerification(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// removeChatVerification implements the Bot API removeChatVerification method.
func (c *Client) removeChatVerification(ctx context.Context, q *server.Query) (any, error) {
	senderChatID := q.Arg("sender_chat_id")
	if senderChatID == "" {
		return nil, NewError(400, "Bad Request: sender_chat_id is empty")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	peer, err := resolvePeerForVerify(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	req := &tg.BotsSetCustomVerificationRequest{
		Enabled: false,
		Peer:    peer,
	}

	_, err = c.rpc.BotsSetCustomVerification(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// removeUserVerification implements the Bot API removeUserVerification method.
func (c *Client) removeUserVerification(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}


	req := &tg.BotsSetCustomVerificationRequest{
		Enabled: false,
		Peer:    &tg.InputPeerUser{UserID: uid},
	}

	_, err = c.rpc.BotsSetCustomVerification(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// resolvePeerForVerify resolves a chat_id to an InputPeerClass for verification
// methods. Unlike regular peer resolution, this supports both users and chats.
func resolvePeerForVerify(ctx context.Context, chatID string, store *storage.Store) (tg.InputPeerClass, error) {
	if chatID == "" {
		return nil, &Error{Code: 400, Description: "Bad Request: chat_id is empty"}
	}
	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, err
	}
	switch chatType {
	case "user":
		if store != nil {
			if p, err := store.GetPeer(ctx, id); err == nil {
				return &tg.InputPeerUser{UserID: id, AccessHash: p.AccessHash}, nil
			}
		}
		return nil, &Error{Code: 400, Description: "Bad Request: user not found in cache"}
	case "chat":
		return &tg.InputPeerChat{ChatID: id}, nil
	case "channel":
		channelID := extractChannelID(id)
		if store != nil {
			if p, err := store.GetPeer(ctx, channelID); err == nil {
				return &tg.InputPeerChannel{ChannelID: channelID, AccessHash: p.AccessHash}, nil
			}
		}
		return &tg.InputPeerChannel{ChannelID: channelID}, nil
	default:
		return nil, &Error{Code: 400, Description: "Bad Request: invalid chat_id"}
	}
}
