package client

import (
	"context"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("editmessagetext", (*Client).editMessageText)
	Register("editmessagecaption", (*Client).editMessageCaption)
	Register("editmessagereplymarkup", (*Client).editMessageReplyMarkup)
	Register("editmessagemedia", (*Client).editMessageMedia)
}

// editMessageText edits the text of a message (chat_id + message_id + text).
// Uses raw tg.MessagesEditMessage. Mirrors do_edit_message_caption/text.
func (c *Client) editMessageText(ctx context.Context, q *server.Query) (any, error) {
	text := q.Arg("text")
	richJSON := q.Arg("rich_message")
	if text == "" && richJSON == "" {
		return nil, NewError(400, "Bad Request: message text is empty")
	}
	peer, id, err := c.resolveTargetMessage(ctx, q)
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesEditMessageRequest{
		Peer: peer,
		ID:   id,
	}
	// rich_message (Bot API 10.1) is mutually exclusive with text and maps to the
	// MTProto rich_message field (flag 23). Reference: Client.cpp:14208 (if/else).
	if richJSON != "" {
		richMsg, err := parseInputRichMessage(richJSON)
		if err != nil {
			return nil, err
		}
		req.RichMessage = richMsg
	} else {
		parsed, entities, ferr := convert.FormattedText(text, q.Arg("parse_mode"), q.Arg("entities"))
		if ferr != nil {
			return nil, NewError(400, "Bad Request: "+ferr.Error())
		}
		req.Message = parsed
		if len(entities) > 0 {
			req.Entities = entities
		}
	}
	// link_preview_options / disable_web_page_preview → no_webpage (flag 1) +
	// show_above_text → invert_media (flag 16).
	applyLinkPreview(q, &req.NoWebpage, &req.InvertMedia)
	req.SetFlags()
	return c.invokeEdit(ctx, req, businessConnID(q))
}

// editMessageCaption edits the caption of a media message. The new caption is
// carried in the "caption" parameter (mapped to Message for the tg request).
func (c *Client) editMessageCaption(ctx context.Context, q *server.Query) (any, error) {
	if q.Arg("message_id") == "" {
		return nil, NewError(400, "Bad Request: message identifier is not specified")
	}
	peer, id, err := c.resolveTargetMessage(ctx, q)
	if err != nil {
		return nil, err
	}
	caption := q.Arg("caption")
	req := &tg.MessagesEditMessageRequest{
		Peer: peer,
		ID:   id,
	}
	if caption != "" {
		parsed, entities, ferr := convert.FormattedText(caption, q.Arg("caption_parse_mode"), q.Arg("caption_entities"))
		if ferr != nil {
			return nil, NewError(400, "Bad Request: "+ferr.Error())
		}
		req.Message = parsed
		if len(entities) > 0 {
			req.Entities = entities
		}
	}
	req.InvertMedia = q.ArgBool("show_caption_above_media")
	req.SetFlags()
	return c.invokeEdit(ctx, req, businessConnID(q))
}

// editMessageReplyMarkup edits only the inline keyboard of a message.
func (c *Client) editMessageReplyMarkup(ctx context.Context, q *server.Query) (any, error) {
	if q.Arg("message_id") == "" {
		return nil, NewError(400, "Bad Request: message identifier is not specified")
	}
	peer, id, err := c.resolveTargetMessage(ctx, q)
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesEditMessageRequest{
		Peer: peer,
		ID:   id,
	}
	if rm := q.Arg("reply_markup"); rm != "" {
		markup, err := convert.ReplyMarkup(rm)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		req.ReplyMarkup = markup
	}
	req.SetFlags()
	return c.invokeEdit(ctx, req, businessConnID(q))
}

// resolveTargetMessage resolves chat_id + message_id into a peer and id,
// ensuring the connection is live. Shared by all editMessage* handlers.
func (c *Client) resolveTargetMessage(ctx context.Context, q *server.Query) (tg.InputPeerClass, int32, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, 0, NewError(400, "Bad Request: chat_id is empty")
	}
	msgID := q.Arg("message_id")
	if msgID == "" {
		return nil, 0, NewError(400, "Bad Request: parameter \"message_id\" is required")
	}
	id, err := parseInt32(msgID)
	if err != nil || id <= 0 {
		return nil, 0, NewError(400, "Bad Request: message_id must be a positive integer")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, 0, connError(err)
	}
	peer, perr := convert.ResolvePeer(ctx, chatID, c.store)
	if perr != nil {
		return nil, 0, NewError(400, "Bad Request: "+perr.Error())
	}
	return peer, id, nil
}

// invokeEdit runs the edit RPC and converts the result to a Bot API Message.
// When connID is non-empty the edit runs on behalf of that business connection
// (invokeWithBusinessConnection) and the returned Message carries the connection id.
func (c *Client) invokeEdit(ctx context.Context, req *tg.MessagesEditMessageRequest, connID string) (any, error) {
	var result tg.UpdatesClass
	if connID != "" {
		obj, err := c.invokeBusiness(ctx, connID, req)
		if err != nil {
			return nil, rpcError(err)
		}
		upd, ok := obj.(tg.UpdatesClass)
		if !ok {
			return nil, NewError(500, "Internal Server Error: unexpected edit result")
		}
		result = upd
	} else {
		var err error
		result, err = c.rpc.MessagesEditMessage(ctx, req)
		if err != nil {
			return nil, rpcError(err)
		}
	}
	msg := extractMessageFromUpdates(result)
	if msg == nil {
		return nil, NewError(500, "Internal Server Error: failed to extract edited message")
	}
	c.cachePeersFromMessage(ctx, msg)
	out := c.botMessage(ctx, msg, extractChats(result))
	if connID != "" {
		out.BusinessConnectionID = connID
	}
	return out, nil
}

// parseInt32 parses a string into int32.
func parseInt32(s string) (int32, error) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}

// editMessageMedia replaces the media of a message. The new media is passed
// as a JSON object in the "media" parameter (type + media fields).
// Reference: Client.cpp process_edit_message_media_query.
func (c *Client) editMessageMedia(ctx context.Context, q *server.Query) (any, error) {
	mediaJSON := q.Arg("media")
	if mediaJSON == "" {
		return nil, NewError(400, "Bad Request: parameter \"media\" is required")
	}
	peer, id, err := c.resolveTargetMessage(ctx, q)
	if err != nil {
		return nil, err
	}
	media, _, err := c.parseMediaDescriptor(ctx, q, mediaJSON)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	req := &tg.MessagesEditMessageRequest{
		Peer:  peer,
		ID:    id,
		Media: media,
	}
	if rm := q.Arg("reply_markup"); rm != "" {
		markup, err := convert.ReplyMarkup(rm)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		req.ReplyMarkup = markup
	}
	req.SetFlags()
	return c.invokeEdit(ctx, req, businessConnID(q))
}
