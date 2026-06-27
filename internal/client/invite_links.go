package client

import (
	"context"
	"strconv"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("exportchatinvitelink", (*Client).exportChatInviteLink)
	Register("createchatinvitelink", (*Client).createChatInviteLink)
	Register("editchatinvitelink", (*Client).editChatInviteLink)
	Register("revokechatinvitelink", (*Client).revokeChatInviteLink)
	Register("createchatsubscriptioninvitelink", (*Client).createChatSubscriptionInviteLink)
	Register("editchatsubscriptioninvitelink", (*Client).editChatSubscriptionInviteLink)
	Register("approvechatjoinrequest", (*Client).approveChatJoinRequest)
	Register("declinechatjoinrequest", (*Client).declineChatJoinRequest)
	Register("answerchatjoinrequestquery", (*Client).answerChatJoinRequestQuery)
	Register("sendchatjoinrequestwebapp", (*Client).sendChatJoinRequestWebApp)
}

// resolvePeerForChat resolves a Bot API chat_id into an InputPeerClass.
// Shared by all invite-link / join-request handlers.
func resolvePeerForChat(ctx context.Context, c *Client, chatID string) (tg.InputPeerClass, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	return peer, nil
}

// exportedToInviteLink converts the RPC result (ExportedChatInviteClass) into a
// Bot API ChatInviteLink. The concrete result is *tg.ChatInviteExported.
func exportedToInviteLink(res tg.ExportedChatInviteClass) (*apitypes.ChatInviteLink, error) {
	exp, ok := res.(*tg.ChatInviteExported)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected invite link response type")
	}
	return convert.ChatInviteLinkFromExported(exp), nil
}

// exportChatInviteLink exports/replaces the primary chat invite link.
// Reference: Client.cpp process_export_chat_invite_link_query.
func (c *Client) exportChatInviteLink(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	res, err := c.rpc.MessagesExportChatInvite(ctx, &tg.MessagesExportChatInviteRequest{
		Peer: peer,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return exportedToInviteLink(res)
}

// createChatInviteLink creates a new (non-primary) invite link.
// Reference: Client.cpp process_create_chat_invite_link_query.
// Params: chat_id, name, expire_date, member_limit, creates_join_request.
func (c *Client) createChatInviteLink(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesExportChatInviteRequest{
		Peer:          peer,
		Title:         q.Arg("name"),
		RequestNeeded: q.ArgBool("creates_join_request"),
	}
	if v := q.Arg("expire_date"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 32); e == nil {
			req.ExpireDate = int32(n)
		}
	}
	if v := q.Arg("member_limit"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 32); e == nil && n > 0 {
			req.UsageLimit = int32(n)
		}
	}
	res, err := c.rpc.MessagesExportChatInvite(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return exportedToInviteLink(res)
}

// createChatSubscriptionInviteLink creates a subscription-based invite link.
// Reference: Client.cpp process_create_chat_subscription_invite_link_query.
// Params: chat_id, name, subscription_period (seconds), subscription_price (stars).
func (c *Client) createChatSubscriptionInviteLink(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	period := int32(0)
	if v := q.Arg("subscription_period"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 32); e == nil {
			period = int32(n)
		}
	}
	price := int64(0)
	if v := q.Arg("subscription_price"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			price = n
		}
	}
	res, err := c.rpc.MessagesExportChatInvite(ctx, &tg.MessagesExportChatInviteRequest{
		Peer:                peer,
		Title:               q.Arg("name"),
		SubscriptionPricing: &tg.StarsSubscriptionPricing{Period: period, Amount: price},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return exportedToInviteLink(res)
}

// editChatInviteLink edits an existing invite link.
// Reference: Client.cpp process_edit_chat_invite_link_query.
// Params: chat_id, invite_link, name, expire_date, member_limit, creates_join_request.
func (c *Client) editChatInviteLink(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	inviteLink := q.Arg("invite_link")
	if inviteLink == "" {
		return nil, NewError(400, "Bad Request: parameter \"invite_link\" is required")
	}
	req := &tg.MessagesEditExportedChatInviteRequest{
		Peer:          peer,
		Link:          inviteLink,
		Title:         q.Arg("name"),
		RequestNeeded: q.ArgBool("creates_join_request"),
	}
	if v := q.Arg("expire_date"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 32); e == nil {
			req.ExpireDate = int32(n)
		}
	}
	if v := q.Arg("member_limit"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 32); e == nil && n > 0 {
			req.UsageLimit = int32(n)
		}
	}
	res, err := c.rpc.MessagesEditExportedChatInvite(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return exportedToInviteLink(res)
}

// editChatSubscriptionInviteLink edits a subscription invite link (name only).
// Reference: Client.cpp process_edit_chat_subscription_invite_link_query.
// Params: chat_id, invite_link, name.
func (c *Client) editChatSubscriptionInviteLink(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	inviteLink := q.Arg("invite_link")
	if inviteLink == "" {
		return nil, NewError(400, "Bad Request: parameter \"invite_link\" is required")
	}
	res, err := c.rpc.MessagesEditExportedChatInvite(ctx, &tg.MessagesEditExportedChatInviteRequest{
		Peer:  peer,
		Link:  inviteLink,
		Title: q.Arg("name"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return exportedToInviteLink(res)
}

// revokeChatInviteLink revokes an invite link.
// Reference: Client.cpp process_revoke_chat_invite_link_query.
// Params: chat_id, invite_link.
func (c *Client) revokeChatInviteLink(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	inviteLink := q.Arg("invite_link")
	if inviteLink == "" {
		return nil, NewError(400, "Bad Request: parameter \"invite_link\" is required")
	}
	res, err := c.rpc.MessagesEditExportedChatInvite(ctx, &tg.MessagesEditExportedChatInviteRequest{
		Peer:    peer,
		Link:    inviteLink,
		Revoked: true,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return exportedToInviteLink(res)
}

// approveChatJoinRequest approves a pending join request.
// Reference: Client.cpp process_approve_chat_join_request_query.
// Params: chat_id, user_id.
func (c *Client) approveChatJoinRequest(ctx context.Context, q *server.Query) (any, error) {
	return c.hideChatJoinRequest(ctx, q, true)
}

// declineChatJoinRequest declines a pending join request.
// Reference: Client.cpp process_decline_chat_join_request_query.
// Params: chat_id, user_id.
func (c *Client) declineChatJoinRequest(ctx context.Context, q *server.Query) (any, error) {
	return c.hideChatJoinRequest(ctx, q, false)
}

// hideChatJoinRequest is the shared approve/decline path.
// approved=true → accept the request; approved=false → decline it.
func (c *Client) hideChatJoinRequest(ctx context.Context, q *server.Query, approved bool) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.MessagesHideChatJoinRequest(ctx, &tg.MessagesHideChatJoinRequestRequest{
		Approved: approved,
		Peer:     peer,
		UserID:   &tg.InputUser{UserID: uid},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// answerChatJoinRequestQuery answers a chat join request query.
// Reference: Client.cpp:14985, TDLib: bots.setJoinChatResults.
// Params: chat_join_request_query_id, result (approve/decline/queue).
func (c *Client) answerChatJoinRequestQuery(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	queryID, err := strconv.ParseInt(q.Arg("chat_join_request_query_id"), 10, 64)
	if err != nil {
		return nil, NewError(400, "Bad Request: parameter \"chat_join_request_query_id\" is required")
	}
	resultStr := strings.ToLower(strings.TrimSpace(q.Arg("result")))
	var result tg.JoinChatBotResultClass
	switch resultStr {
	case "approve":
		result = &tg.JoinChatBotResultApproved{}
	case "decline":
		result = &tg.JoinChatBotResultDeclined{}
	case "queue":
		result = &tg.JoinChatBotResultQueued{}
	default:
		return nil, NewError(400, "Bad Request: invalid query result specified")
	}
	_, err = c.rpc.BotsSetJoinChatResults(ctx, &tg.BotsSetJoinChatResultsRequest{
		QueryID: queryID,
		Result:  result,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// sendChatJoinRequestWebApp sends a web app URL for a chat join request query.
// Reference: Client.cpp:15004, TDLib: bots.setJoinChatResults with JoinChatBotResultWebView.
// Params: chat_join_request_query_id, web_app_url.
func (c *Client) sendChatJoinRequestWebApp(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	webAppURL := q.Arg("web_app_url")
	if webAppURL == "" {
		return nil, NewError(400, "Bad Request: parameter \"web_app_url\" is required")
	}
	queryID, err := strconv.ParseInt(q.Arg("chat_join_request_query_id"), 10, 64)
	if err != nil {
		return nil, NewError(400, "Bad Request: parameter \"chat_join_request_query_id\" is required")
	}
	_, err = c.rpc.BotsSetJoinChatResults(ctx, &tg.BotsSetJoinChatResultsRequest{
		QueryID: queryID,
		Result:  &tg.JoinChatBotResultWebView{URL: webAppURL},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}
