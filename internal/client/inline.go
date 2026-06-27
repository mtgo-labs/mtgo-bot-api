package client

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("answerinlinequery", (*Client).answerInlineQuery)
	Register("answercallbackquery", (*Client).answerCallbackQuery)
	Register("setgamescore", (*Client).setGameScore)
	Register("getgamehighscores", (*Client).getGameHighScores)
	Register("savepreparedinlinemessage", (*Client).savePreparedInlineMessage)
	Register("answerwebappquery", (*Client).answerWebAppQuery)
	Register("answerguestquery", (*Client).answerGuestQuery)
	Register("savepreparedkeyboardbutton", (*Client).savePreparedKeyboardButton)
}

// answerInlineQuery implements the Bot API answerInlineQuery method.
func (c *Client) answerInlineQuery(ctx context.Context, q *server.Query) (any, error) {
	inlineQueryID := q.Arg("inline_query_id")
	resultsJSON := q.Arg("results")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	queryID, _ := strconv.ParseInt(inlineQueryID, 10, 64)

	var results []tg.InputBotInlineResultClass
	var err error
	if resultsJSON != "" {
		results, err = convert.InlineQueryResults(resultsJSON)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
	}

	req := &tg.MessagesSetInlineBotResultsRequest{
		QueryID: queryID,
		Results: results,
	}

	if cacheTime := q.Arg("cache_time"); cacheTime != "" {
		if ct, err := strconv.ParseInt(cacheTime, 10, 32); err == nil {
			req.CacheTime = int32(ct)
		}
	}
	if nextOffset := q.Arg("next_offset"); nextOffset != "" {
		req.NextOffset = nextOffset
	}
	if q.ArgBool("is_personal") {
		req.Private = true
		req.Flags.Set(1)
	}
	if q.ArgBool("gallery") {
		req.Gallery = true
		req.Flags.Set(0)
	}
	if buttonJSON := q.Arg("button"); buttonJSON != "" {
		// Bot API 10.1 `button` (InlineQueryResultsButton): {text, start_parameter|web_app.url}.
		// Maps to switch_pm (flag 3) or switch_webview (flag 4); supersedes switch_pm_text.
		// Mirrors Client.cpp get_inline_query_results_button (Client.cpp:10788-10834).
		var btn struct {
			Text           string `json:"text"`
			StartParameter string `json:"start_parameter"`
			WebApp         struct {
				URL string `json:"url"`
			} `json:"web_app"`
		}
		if err := json.Unmarshal([]byte(buttonJSON), &btn); err != nil {
			return nil, NewError(400, "Bad Request: can't parse button JSON")
		}
		switch {
		case btn.StartParameter != "" && btn.WebApp.URL != "":
			return nil, NewError(400, "Bad Request: InlineQueryResultsButton must have exactly one optional field")
		case btn.StartParameter != "":
			req.SwitchPm = &tg.InlineBotSwitchPm{Text: btn.Text, StartParam: btn.StartParameter}
		case btn.WebApp.URL != "":
			req.SwitchWebview = &tg.InlineBotWebView{Text: btn.Text, URL: btn.WebApp.URL}
		default:
			return nil, NewError(400, "Bad Request: InlineQueryResultsButton must have exactly one optional field")
		}
	} else if switchPM := q.Arg("switch_pm_text"); switchPM != "" {
		req.SwitchPm = &tg.InlineBotSwitchPm{
			Text:       switchPM,
			StartParam: q.Arg("switch_pm_parameter"),
		}
	}

	req.SetFlags()

	_, err = c.rpc.MessagesSetInlineBotResults(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// answerCallbackQuery implements the Bot API answerCallbackQuery method.
func (c *Client) answerCallbackQuery(ctx context.Context, q *server.Query) (any, error) {
	callbackQueryID := q.Arg("callback_query_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	queryID, _ := strconv.ParseInt(callbackQueryID, 10, 64)

	req := &tg.MessagesSetBotCallbackAnswerRequest{
		QueryID: queryID,
	}
	if text := q.Arg("text"); text != "" {
		req.Message = text
		req.Flags.Set(0)
	}
	if q.ArgBool("show_alert") {
		req.Alert = true
		req.Flags.Set(1)
	}
	if url := q.Arg("url"); url != "" {
		req.URL = url
		req.Flags.Set(2)
	}
	if cacheTime := q.Arg("cache_time"); cacheTime != "" {
		if ct, err := strconv.ParseInt(cacheTime, 10, 32); err == nil {
			req.CacheTime = int32(ct)
			req.Flags.Set(3)
		}
	}
	req.SetFlags()

	_, err := c.rpc.MessagesSetBotCallbackAnswer(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setGameScore implements the Bot API setGameScore method.
func (c *Client) setGameScore(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	scoreStr := q.Arg("score")
	if scoreStr == "" {
		return nil, NewError(400, "Bad Request: parameter \"score\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	score, err := strconv.ParseInt(scoreStr, 10, 32)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid score")
	}

	editMessage := true
	if q.HasArg("disable_edit_message") {
		editMessage = !q.ArgBool("disable_edit_message")
	} else if q.HasArg("edit_message") {
		editMessage = q.ArgBool("edit_message")
	}

	inlineMessageID := q.Arg("inline_message_id")
	if inlineMessageID != "" {
		// Inline message path.
		inputID, err := convert.InlineMessageIDFromStr(inlineMessageID)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		req := &tg.MessagesSetInlineGameScoreRequest{
			ID:          inputID,
			UserID:      &tg.InputUser{UserID: uid},
			Score:       int32(score),
			EditMessage: editMessage,
			Force:       q.ArgBool("force"),
		}
		req.SetFlags()
		_, err = c.rpc.MessagesSetInlineGameScore(ctx, req)
		if err != nil {
			return nil, rpcError(err)
		}
	} else {
		// Regular message path.
		chatID := q.Arg("chat_id")
		messageID := q.Arg("message_id")
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
		req := &tg.MessagesSetGameScoreRequest{
			Peer:        peer,
			ID:          int32(msgID),
			UserID:      &tg.InputUser{UserID: uid},
			Score:       int32(score),
			EditMessage: editMessage,
			Force:       q.ArgBool("force"),
		}
		req.SetFlags()
		result, err := c.rpc.MessagesSetGameScore(ctx, req)
		if err != nil {
			return nil, rpcError(err)
		}
		msg := extractMessageFromUpdates(result)
		if msg != nil {
			return c.botMessage(ctx, msg, extractChats(result)), nil
		}
	}
	return true, nil
}

// getGameHighScores implements the Bot API getGameHighScores method.
func (c *Client) getGameHighScores(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	inlineMessageID := q.Arg("inline_message_id")
	if inlineMessageID != "" {
		inputID, err := convert.InlineMessageIDFromStr(inlineMessageID)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		result, err := c.rpc.MessagesGetInlineGameHighScores(ctx, &tg.MessagesGetInlineGameHighScoresRequest{
			ID:     inputID,
			UserID: &tg.InputUser{UserID: uid},
		})
		if err != nil {
			return nil, rpcError(err)
		}
		users := extractUsersFromScores(result.Users)
		return convert.HighScores(result.Scores, users), nil
	}

	chatID := q.Arg("chat_id")
	messageID := q.Arg("message_id")
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

	result, err := c.rpc.MessagesGetGameHighScores(ctx, &tg.MessagesGetGameHighScoresRequest{
		Peer:   peer,
		ID:     int32(msgID),
		UserID: &tg.InputUser{UserID: uid},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	users := extractUsersFromScores(result.Users)
	return convert.HighScores(result.Scores, users), nil
}

// savePreparedInlineMessage implements the Bot API savePreparedInlineMessage method.
func (c *Client) savePreparedInlineMessage(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	resultJSON := q.Arg("result")
	if resultJSON == "" {
		return nil, NewError(400, "Bad Request: parameter \"result\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}


	results, err := convert.InlineQueryResults("[" + resultJSON + "]")
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if len(results) == 0 {
		return nil, NewError(400, "Bad Request: invalid result")
	}

	// allow_*_chats → peer_types vector (MTProto flag 0). Mirrors TDLib
	// TargetDialogTypes::get_input_peer_types: allow_group_chats expands to
	// Chat+Megagroup; all four true → nil (flag 0 unset = all allowed); all
	// false → 400 (Client.cpp:15015-15020 + TargetDialogTypes.cpp:54-56).
	allowUser := q.ArgBool("allow_user_chats")
	allowBot := q.ArgBool("allow_bot_chats")
	allowGroup := q.ArgBool("allow_group_chats")
	allowChannel := q.ArgBool("allow_channel_chats")
	if !allowUser && !allowBot && !allowGroup && !allowChannel {
		return nil, NewError(400, "Bad Request: at least one chat type must be allowed")
	}
	var peerTypes []tg.InlineQueryPeerTypeClass
	if !(allowUser && allowBot && allowGroup && allowChannel) {
		if allowUser {
			peerTypes = append(peerTypes, &tg.InlineQueryPeerTypePm{})
		}
		if allowBot {
			peerTypes = append(peerTypes, &tg.InlineQueryPeerTypeBotPm{})
		}
		if allowGroup {
			peerTypes = append(peerTypes, &tg.InlineQueryPeerTypeChat{}, &tg.InlineQueryPeerTypeMegagroup{})
		}
		if allowChannel {
			peerTypes = append(peerTypes, &tg.InlineQueryPeerTypeBroadcast{})
		}
	}

	req := &tg.MessagesSavePreparedInlineMessageRequest{
		Result:    results[0],
		UserID:    &tg.InputUser{UserID: uid},
		PeerTypes: peerTypes,
	}
	req.SetFlags()

	resp, err := c.rpc.MessagesSavePreparedInlineMessage(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return map[string]any{
		"id":              resp.ID,
		"expiration_date": resp.ExpireDate,
	}, nil
}

// answerWebAppQuery implements the Bot API answerWebAppQuery method.
// Uses messages.sendWebViewResultMessage at the MTProto level.
func (c *Client) answerWebAppQuery(ctx context.Context, q *server.Query) (any, error) {
	resultJSON := q.Arg("result")
	if resultJSON == "" {
		return nil, NewError(400, "Bad Request: result isn't specified")
	}
	webAppQueryID := q.Arg("web_app_query_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	results, err := convert.InlineQueryResults("[" + resultJSON + "]")
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if len(results) == 0 {
		return nil, NewError(400, "Bad Request: invalid result")
	}

	resp, err := c.rpc.MessagesSendWebViewResultMessage(ctx, &tg.MessagesSendWebViewResultMessageRequest{
		BotQueryID: webAppQueryID,
		Result:     results[0],
	})
	if err != nil {
		return nil, rpcError(err)
	}

	if resp.MsgID != nil {
		return convert.InlineMessageIDFromTL(resp.MsgID), nil
	}
	return "", nil
}

// answerGuestQuery implements the Bot API answerGuestQuery method.
// Uses messages.setBotGuestChatResult at the MTProto level.
func (c *Client) answerGuestQuery(ctx context.Context, q *server.Query) (any, error) {
	resultJSON := q.Arg("result")
	if resultJSON == "" {
		return nil, NewError(400, "Bad Request: result isn't specified")
	}
	guestQueryID := q.Arg("guest_query_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	queryID, err := strconv.ParseInt(guestQueryID, 10, 64)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid guest_query_id")
	}

	results, err := convert.InlineQueryResults("[" + resultJSON + "]")
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if len(results) == 0 {
		return nil, NewError(400, "Bad Request: invalid result")
	}

	msgID, err := c.rpc.MessagesSetBotGuestChatResult(ctx, &tg.MessagesSetBotGuestChatResultRequest{
		QueryID: queryID,
		Result:  results[0],
	})
	if err != nil {
		return nil, rpcError(err)
	}

	return convert.InlineMessageIDFromTL(msgID), nil
}

// savePreparedKeyboardButton implements the Bot API savePreparedKeyboardButton method.
// Uses bots.requestWebViewButton at the MTProto level.
func (c *Client) savePreparedKeyboardButton(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	buttonJSON := q.Arg("button")
	if buttonJSON == "" {
		return nil, NewError(400, "Bad Request: parameter \"button\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}


	button, err := convert.KeyboardButtonFromJSON(buttonJSON)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	resp, err := c.rpc.BotsRequestWebViewButton(ctx, &tg.BotsRequestWebViewButtonRequest{
		UserID: &tg.InputUser{UserID: uid},
		Button: button,
	})
	if err != nil {
		return nil, rpcError(err)
	}

	return resp.WebappReqID, nil
}

// extractUsersFromScores collects users from a []UserClass slice.
func extractUsersFromScores(users []tg.UserClass) map[int64]*tg.User {
	m := make(map[int64]*tg.User)
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			m[user.ID] = user
		}
	}
	return m
}
