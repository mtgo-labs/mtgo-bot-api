package client

import (
	"context"
	"encoding/json"
	"os"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("setchattitle", (*Client).setChatTitle)
	Register("setchatdescription", (*Client).setChatDescription)
	Register("setchatphoto", (*Client).setChatPhoto)
	Register("deletechatphoto", (*Client).deleteChatPhoto)
	Register("setchatpermissions", (*Client).setChatPermissions)
	Register("setchatstickerset", (*Client).setChatStickerSet)
	Register("deletechatstickerset", (*Client).deleteChatStickerSet)
	Register("setchatmenubutton", (*Client).setChatMenuButton)
	Register("getchatmenubutton", (*Client).getChatMenuButton)
}

// setChatTitle implements the Bot API setChatTitle method.
func (c *Client) setChatTitle(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	title := q.Arg("title")
	if title == "" {
		return nil, NewError(400, "Bad Request: parameter \"title\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	switch chatType {
	case "chat":
		_, err = c.rpc.MessagesEditChatTitle(ctx, &tg.MessagesEditChatTitleRequest{
			ChatID: id,
			Title:  title,
		})
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		_, err = c.rpc.ChannelsEditTitle(ctx, &tg.ChannelsEditTitleRequest{
			Channel: inputCh,
			Title:   title,
		})
	default:
		return nil, NewError(400, "Bad Request: only groups and supergroups are supported")
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setChatDescription implements the Bot API setChatDescription method.
func (c *Client) setChatDescription(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	description := q.Arg("description")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	switch chatType {
	case "chat":
		// Basic groups don't support descriptions at the MTProto level.
		return nil, NewError(400, "Bad Request: chat description is not supported for basic groups")
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		ic, ok := inputCh.(*tg.InputChannel)
		if !ok {
			return nil, NewError(400, "Bad Request: failed to resolve channel")
		}
		_, err = c.rpc.MessagesEditChatAbout(ctx, &tg.MessagesEditChatAboutRequest{
			Peer:  &tg.InputPeerChannel{ChannelID: channelID, AccessHash: ic.AccessHash},
			About: description,
		})
	default:
		return nil, NewError(400, "Bad Request: only groups and supergroups are supported")
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setChatPhoto implements the Bot API setChatPhoto method.
func (c *Client) setChatPhoto(ctx context.Context, q *server.Query) (any, error) {
	photoFile, ok := q.File("photo")
	if !ok {
		return nil, NewError(400, "Bad Request: there is no photo in the request")
	}
	_ = photoFile
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	f, err := os.Open(photoFile.TempPath)
	if err != nil {
		return nil, NewError(400, "Bad Request: failed to read photo")
	}
	defer func() { _ = f.Close() }()
	fileID, _ := generateFileID()
	inputFile, err := c.uploadFile(ctx, fileID, photoFile.FileName, photoFile.Size, f)
	if err != nil {
		return nil, rpcError(err)
	}
	photo := &tg.InputChatUploadedPhoto{File: inputFile}
	photo.SetFlags()

	switch chatType {
	case "chat":
		_, err = c.rpc.MessagesEditChatPhoto(ctx, &tg.MessagesEditChatPhotoRequest{
			ChatID: id,
			Photo:  photo,
		})
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		_, err = c.rpc.ChannelsEditPhoto(ctx, &tg.ChannelsEditPhotoRequest{
			Channel: inputCh,
			Photo:   photo,
		})
	default:
		return nil, NewError(400, "Bad Request: only groups and supergroups are supported")
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// deleteChatPhoto implements the Bot API deleteChatPhoto method.
func (c *Client) deleteChatPhoto(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	switch chatType {
	case "chat":
		_, err = c.rpc.MessagesEditChatPhoto(ctx, &tg.MessagesEditChatPhotoRequest{
			ChatID: id,
			Photo:  &tg.InputChatPhotoEmpty{},
		})
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		_, err = c.rpc.ChannelsEditPhoto(ctx, &tg.ChannelsEditPhotoRequest{
			Channel: inputCh,
			Photo:   &tg.InputChatPhotoEmpty{},
		})
	default:
		return nil, NewError(400, "Bad Request: only groups and supergroups are supported")
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setChatPermissions implements the Bot API setChatPermissions method.
func (c *Client) setChatPermissions(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	permissionsJSON := q.Arg("permissions")
	if permissionsJSON == "" {
		return nil, NewError(400, "Bad Request: parameter \"permissions\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	var perms map[string]bool
	if err := json.Unmarshal([]byte(permissionsJSON), &perms); err != nil {
		return nil, NewError(400, "Bad Request: invalid permissions JSON")
	}

	applyLegacyPermissionBundling(perms, q.ArgBool("use_independent_chat_permissions"))
	bannedRights := permissionsToBannedRights(perms)

	switch chatType {
	case "chat":
		_, err = c.rpc.MessagesEditChatDefaultBannedRights(ctx, &tg.MessagesEditChatDefaultBannedRightsRequest{
			Peer:         &tg.InputPeerChat{ChatID: id},
			BannedRights: bannedRights,
		})
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		ch, ok := inputCh.(*tg.InputChannel)
		if !ok {
			return nil, NewError(400, "Bad Request: failed to resolve channel")
		}
		_, err = c.rpc.MessagesEditChatDefaultBannedRights(ctx, &tg.MessagesEditChatDefaultBannedRightsRequest{
			Peer:         &tg.InputPeerChannel{ChannelID: channelID, AccessHash: ch.AccessHash},
			BannedRights: bannedRights,
		})
	default:
		return nil, NewError(400, "Bad Request: only groups and supergroups are supported")
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setChatStickerSet implements the Bot API setChatStickerSet method.
func (c *Client) setChatStickerSet(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	stickerSetName := q.Arg("sticker_set_name")
	if stickerSetName == "" {
		return nil, NewError(400, "Bad Request: parameter \"sticker_set_name\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	_, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if chatType != "channel" {
		return nil, NewError(400, "Bad Request: only supergroups are supported")
	}

	inputCh, err := c.resolveInputChannel(ctx, extractChannelID(chatIDint(chatID)))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	_, err = c.rpc.ChannelsSetStickers(ctx, &tg.ChannelsSetStickersRequest{
		Channel:    inputCh,
		Stickerset: &tg.InputStickerSetShortName{ShortName: stickerSetName},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// deleteChatStickerSet implements the Bot API deleteChatStickerSet method.
func (c *Client) deleteChatStickerSet(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	_, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if chatType != "channel" {
		return nil, NewError(400, "Bad Request: only supergroups are supported")
	}

	inputCh, err := c.resolveInputChannel(ctx, extractChannelID(chatIDint(chatID)))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	_, err = c.rpc.ChannelsSetStickers(ctx, &tg.ChannelsSetStickersRequest{
		Channel:    inputCh,
		Stickerset: &tg.InputStickerSetEmpty{},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// setChatMenuButton implements the Bot API setChatMenuButton method.
// At the MTProto level, this is per-user (not per-chat). The chat_id maps to
// the user_id of the chat partner; absent means default.
func (c *Client) setChatMenuButton(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	var userID int64
	if chatID := q.Arg("chat_id"); chatID != "" {
		id, _, err := parseChatID(chatID)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		if id > 0 {
			userID = id
		}
	}

	buttonJSON := q.Arg("menu_button")
	var button tg.BotMenuButtonClass
	if buttonJSON == "" {
		button = &tg.BotMenuButtonCommands{}
	} else {
		var parsed struct {
			Type string `json:"type"`
			Text string `json:"text"`
			URL  string `json:"url"`
		}
		if err := json.Unmarshal([]byte(buttonJSON), &parsed); err != nil {
			return nil, NewError(400, "Bad Request: invalid menu_button JSON")
		}
		switch parsed.Type {
		case "commands":
			button = &tg.BotMenuButtonCommands{}
		case "web_app":
			button = &tg.BotMenuButton{Text: parsed.Text, URL: parsed.URL}
		default:
			button = &tg.BotMenuButtonDefault{}
		}
	}

	var inputUser tg.InputUserClass = &tg.InputUser{UserID: userID}
	if userID == 0 {
		inputUser = &tg.InputUserSelf{}
	}

	_, err := c.rpc.BotsSetBotMenuButton(ctx, &tg.BotsSetBotMenuButtonRequest{
		UserID: inputUser,
		Button: button,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getChatMenuButton implements the Bot API getChatMenuButton method.
func (c *Client) getChatMenuButton(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	var userID int64
	if chatID := q.Arg("chat_id"); chatID != "" {
		id, _, err := parseChatID(chatID)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		if id > 0 {
			userID = id
		}
	}

	// No chat_id → the default menu button (reference getMenuButton(0)). An empty
	// InputUser{} is USER_ID_INVALID; the default maps to InputUserSelf.
	var inputUser tg.InputUserClass = &tg.InputUser{UserID: userID}
	if userID == 0 {
		inputUser = &tg.InputUserSelf{}
	}

	result, err := c.rpc.BotsGetBotMenuButton(ctx, &tg.BotsGetBotMenuButtonRequest{
		UserID: inputUser,
	})
	if err != nil {
		return nil, rpcError(err)
	}

	switch btn := result.(type) {
	case *tg.BotMenuButtonDefault:
		return map[string]string{"type": "default"}, nil
	case *tg.BotMenuButtonCommands:
		return map[string]string{"type": "commands"}, nil
	case *tg.BotMenuButton:
		return map[string]string{
			"type": "web_app",
			"text": btn.Text,
			"url":  btn.URL,
		}, nil
	default:
		return map[string]string{"type": "default"}, nil
	}
}
