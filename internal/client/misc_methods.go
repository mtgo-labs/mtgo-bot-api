package client

import (
	"context"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	// M15 misc methods.
	Register("sendgame", (*Client).sendGame)
	Register("editmessagelivelocation", (*Client).editMessageLiveLocation)
	Register("stopmessagelivelocation", (*Client).stopMessageLiveLocation)
}

// editMessageLiveLocation implements the Bot API editMessageLiveLocation method.
// Uses messages.editMessage with InputMediaGeoLive.
func (c *Client) editMessageLiveLocation(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	messageID := q.Arg("message_id")
	// Coordinates validated per-field, mirroring the reference get_location helper
	// (Client.cpp:12003/12007), invoked for editMessageLiveLocation (Client.cpp:14268).
	latStr := q.Arg("latitude")
	if latStr == "" {
		return nil, NewError(400, "Bad Request: latitude is empty")
	}
	lonStr := q.Arg("longitude")
	if lonStr == "" {
		return nil, NewError(400, "Bad Request: longitude is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, connError(err)
	}
	lat, _ := strconv.ParseFloat(latStr, 64)
	lon, _ := strconv.ParseFloat(lonStr, 64)
	geo := &tg.InputGeoPoint{Lat: lat, Long: lon}
	if ha := q.Arg("horizontal_accuracy"); ha != "" {
		if v, err := strconv.ParseFloat(ha, 64); err == nil {
			geo.AccuracyRadius = int32(v)
			geo.Flags.Set(0)
		}
	}
	livePeriod := int32(0)
	if lp := q.Arg("live_period"); lp != "" {
		if v, err := strconv.ParseInt(lp, 10, 32); err == nil {
			livePeriod = int32(v)
		}
	}
	media := &tg.InputMediaGeoLive{
		GeoPoint: geo,
		Period:   livePeriod,
	}
	if heading := q.Arg("heading"); heading != "" {
		if v, err := strconv.ParseInt(heading, 10, 32); err == nil {
			media.Heading = int32(v)
			media.Flags.Set(2)
		}
	}
	media.SetFlags()

	inlineMessageID := q.Arg("inline_message_id")
	if inlineMessageID != "" {
		// Inline path: use messages.editInlineBotMessage with InputMediaGeoLive.
		inputID, err := convert.InlineMessageIDFromStr(inlineMessageID)
		if err != nil {
			return nil, NewError(400, "Bad Request: invalid inline_message_id")
		}
		req := &tg.MessagesEditInlineBotMessageRequest{
			ID:    inputID,
			Media: media,
		}
		req.SetFlags()
		_, err = c.rpc.MessagesEditInlineBotMessage(ctx, req)
		if err != nil {
			return nil, rpcError(err)
		}
		return true, nil
	}
	if chatID == "" || messageID == "" {
		return nil, NewError(400, "Bad Request: chat_id and message_id or inline_message_id required")
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	msgID, err := strconv.ParseInt(messageID, 10, 32)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid message_id")
	}
	req := &tg.MessagesEditMessageRequest{
		Peer:  peer,
		ID:    int32(msgID),
		Media: media,
	}
	// Return the edited Message for the chat path (parity with the reference). SetFlags
	// sets the media flag (14); the old code set flag 1 (NoWebpage) by mistake.
	req.SetFlags()
	return c.invokeEdit(ctx, req, businessConnID(q))
}

// stopMessageLiveLocation implements the Bot API stopMessageLiveLocation method.
func (c *Client) stopMessageLiveLocation(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	messageID := q.Arg("message_id")
	if messageID == "" {
		return nil, NewError(400, "Bad Request: message identifier is not specified")
	}
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
	msgID, err := strconv.ParseInt(messageID, 10, 32)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid message_id")
	}
	media := &tg.InputMediaGeoLive{Stopped: true}
	media.SetFlags()
	req := &tg.MessagesEditMessageRequest{
		Peer:  peer,
		ID:    int32(msgID),
		Media: media,
	}
	// Return the edited Message for the chat path (parity with the reference). SetFlags
	// sets the media flag (14).
	req.SetFlags()
	return c.invokeEdit(ctx, req, businessConnID(q))
}

// sendGame sends a game. Reference: Client.cpp process_send_game_query.
// Params: chat_id, game_short_name.
func (c *Client) sendGame(ctx context.Context, q *server.Query) (any, error) {
	gameShortName := q.Arg("game_short_name")
	if gameShortName == "" {
		return nil, NewError(400, "Bad Request: parameter \"game_short_name\" is required")
	}
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
	media := &tg.InputMediaGame{
		ID: &tg.InputGameShortName{
			BotID:     &tg.InputUserSelf{},
			ShortName: gameShortName,
		},
	}
	req := &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		RandomID: randomID(),
	}
	req.SetFlags()
	result, err := c.rpc.MessagesSendMedia(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	msg := extractMessageFromUpdates(result)
	if msg == nil {
		return true, nil
	}
	c.cachePeersFromMessage(ctx, msg)
	return c.botMessage(ctx, msg, extractChats(result)), nil
}

// deleteMessageReaction (deletemessagereaction) is implemented in stories_misc.go
// alongside deleteAllMessageReactions — see that handler for the per-message,
// per-sender delete (messages.deleteParticipantReaction).
