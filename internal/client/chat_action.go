package client

import (
	"context"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"
	"github.com/mtgo-labs/mtgo/tgerr"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("sendchataction", (*Client).sendChatAction)
}

// sendChatAction implements the Bot API sendChatAction method.
// Reference: telegram-bot-api/Client.cpp process_send_chat_action_query.
//
// Required parameters: chat_id, action.
func (c *Client) sendChatAction(ctx context.Context, q *server.Query) (any, error) {
	action := q.Arg("action")
	if action == "" {
		return nil, NewError(400, "Bad Request: wrong parameter action in request")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
	}

	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	tgAction := mapChatAction(action)
	if tgAction == nil {
		return nil, NewError(400, "Bad Request: unknown action type")
	}

	req := &tg.MessagesSetTypingRequest{
		Peer:   peer,
		Action: tgAction,
	}
	// Forum-topic targeting: message_thread_id → top-level top_msg_id (flag 0).
	if topic := topicID(q); topic != 0 {
		req.TopMsgID = topic
		req.SetFlags()
	}
	if connID := businessConnID(q); connID != "" {
		if _, err := c.invokeBusiness(ctx, connID, req); err != nil {
			return nil, rpcError(err)
		}
	} else if _, err := c.rpc.MessagesSetTyping(ctx, req); err != nil {
		// USER_IS_BOT is silently ignored — TDLib returns success when sending
		// typing indicators to bot users (the action is a no-op for bots).
		if rpcErr, ok := tgerr.As(err); ok && rpcErr.Message == "USER_IS_BOT" {
			return true, nil
		}
		return nil, rpcError(err)
	}

	return true, nil
}

// mapChatAction maps a Bot API action string to a tg.SendMessageActionClass.
func mapChatAction(action string) tg.SendMessageActionClass {
	switch strings.ToLower(action) {
	case "typing":
		return &tg.SendMessageTypingAction{}
	case "upload_photo":
		return &tg.SendMessageUploadPhotoAction{Progress: 0}
	case "record_video":
		return &tg.SendMessageRecordVideoAction{}
	case "upload_video":
		return &tg.SendMessageUploadVideoAction{Progress: 0}
	case "record_voice":
		return &tg.SendMessageRecordAudioAction{}
	case "upload_voice":
		return &tg.SendMessageUploadAudioAction{Progress: 0}
	case "upload_document":
		return &tg.SendMessageUploadDocumentAction{Progress: 0}
	case "choose_sticker":
		return &tg.SendMessageChooseStickerAction{}
	case "find_location":
		return &tg.SendMessageGeoLocationAction{}
	case "record_video_note":
		return &tg.SendMessageRecordRoundAction{}
	case "upload_video_note":
		return &tg.SendMessageUploadRoundAction{Progress: 0}
	default:
		return nil
	}
}
