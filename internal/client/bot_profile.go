package client

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"
	"github.com/mtgo-labs/mtgo/tgerr"
	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func init() {
	Register("getmycommands", (*Client).getMyCommands)
	Register("setmycommands", (*Client).setMyCommands)
	Register("deletemycommands", (*Client).deleteMyCommands)
	Register("getmyname", (*Client).getMyName)
	Register("setmyname", (*Client).setMyName)
	Register("getmydescription", (*Client).getMyDescription)
	Register("setmydescription", (*Client).setMyDescription)
	Register("getmyshortdescription", (*Client).getMyShortDescription)
	Register("setmyshortdescription", (*Client).setMyShortDescription)
	Register("getmydefaultadministratorrights", (*Client).getMyDefaultAdministratorRights)
	Register("setmydefaultadministratorrights", (*Client).setMyDefaultAdministratorRights)
	Register("setmyprofilephoto", (*Client).setMyProfilePhoto)
	Register("removemyprofilephoto", (*Client).removeMyProfilePhoto)
	Register("getuserprofilephotos", (*Client).getUserProfilePhotos)
	Register("getuserprofileaudios", (*Client).getUserProfileAudios)
	Register("setpassportdataerrors", (*Client).setPassportDataErrors)
	Register("getcustomemojistickers", (*Client).getCustomEmojiStickers)
	Register("setuseremojistatus", (*Client).setUserEmojiStatus)
	Register("getuserpersonalchatmessages", (*Client).getUserPersonalChatMessages)
	Register("getmanagedbottoken", (*Client).getManagedBotToken)
	Register("replacemanagedbottoken", (*Client).replaceManagedBotToken)
	Register("getmanagedbotaccesssettings", (*Client).getManagedBotAccessSettings)
	Register("setmanagedbotaccesssettings", (*Client).setManagedBotAccessSettings)
}

// getMyCommands implements the Bot API getMyCommands method.
// Uses bots.getBotCommands which returns Vector<BotCommand>.
func (c *Client) getMyCommands(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	scope, langCode := parseBotCommandScope(q)
	result, err := c.rpc.BotsGetBotCommands(ctx, &tg.BotsGetBotCommandsRequest{
		Scope:    scope,
		LangCode: langCode,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return extractBotCommands(result), nil
}

// setMyCommands implements the Bot API setMyCommands method.
func (c *Client) setMyCommands(ctx context.Context, q *server.Query) (any, error) {
	commandsJSON := q.Arg("commands")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	// Empty commands = set empty list (deletes all commands for the scope).
	// Reference: Client.cpp get_bot_commands returns empty vector for empty arg.
	var commands []*tg.BotCommand
	if commandsJSON != "" {
		var err error
		commands, err = parseBotCommands(commandsJSON)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
	}
	scope, langCode := parseBotCommandScope(q)
	_, err := c.rpc.BotsSetBotCommands(ctx, &tg.BotsSetBotCommandsRequest{
		Scope:    scope,
		LangCode: langCode,
		Commands: commands,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// deleteMyCommands implements the Bot API deleteMyCommands method.
func (c *Client) deleteMyCommands(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	scope, langCode := parseBotCommandScope(q)
	_, err := c.rpc.BotsResetBotCommands(ctx, &tg.BotsResetBotCommandsRequest{
		Scope:    scope,
		LangCode: langCode,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getMyName implements the Bot API getMyName method.
func (c *Client) getMyName(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	result, err := c.rpc.BotsGetBotInfo(ctx, &tg.BotsGetBotInfoRequest{
		LangCode: q.Arg("language_code"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	info, ok := result.(*tg.BotsBotInfo)
	if !ok {
		return map[string]string{"name": ""}, nil
	}
	return map[string]string{"name": info.Name}, nil
}

// setMyName implements the Bot API setMyName method.
func (c *Client) setMyName(ctx context.Context, q *server.Query) (any, error) {
	name := q.Arg("name")
	// TDLib validates the name via setBotName → BOT_TITLE_INVALID for empty/invalid.
	if name == "" {
		return nil, NewError(400, "Bad Request: BOT_TITLE_INVALID")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	_, err := c.rpc.BotsSetBotInfo(ctx, &tg.BotsSetBotInfoRequest{
		LangCode: q.Arg("language_code"),
		Name:     q.Arg("name"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getMyDescription implements the Bot API getMyDescription method.
func (c *Client) getMyDescription(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	result, err := c.rpc.BotsGetBotInfo(ctx, &tg.BotsGetBotInfoRequest{
		LangCode: q.Arg("language_code"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	info, ok := result.(*tg.BotsBotInfo)
	if !ok {
		return map[string]string{"description": ""}, nil
	}
	return map[string]string{"description": info.Description}, nil
}

// setMyDescription implements the Bot API setMyDescription method.
func (c *Client) setMyDescription(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	_, err := c.rpc.BotsSetBotInfo(ctx, &tg.BotsSetBotInfoRequest{
		LangCode:    q.Arg("language_code"),
		Description: q.Arg("description"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getMyShortDescription implements the Bot API getMyShortDescription method.
func (c *Client) getMyShortDescription(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	result, err := c.rpc.BotsGetBotInfo(ctx, &tg.BotsGetBotInfoRequest{
		LangCode: q.Arg("language_code"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	info, ok := result.(*tg.BotsBotInfo)
	if !ok {
		return map[string]string{"short_description": ""}, nil
	}
	return map[string]string{"short_description": info.About}, nil
}

// setMyShortDescription implements the Bot API setMyShortDescription method.
func (c *Client) setMyShortDescription(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	_, err := c.rpc.BotsSetBotInfo(ctx, &tg.BotsSetBotInfoRequest{
		LangCode: q.Arg("language_code"),
		About:    q.Arg("short_description"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getMyDefaultAdministratorRights implements the Bot API getMyDefaultAdministratorRights method.
// Uses users.getFullUser to fetch the bot's own UserFull, which contains
// bot_group_admin_rights and bot_broadcast_admin_rights fields.
// MTProto: users.getFullUser#b60f5918 → UserFull.bot_group_admin_rights / bot_broadcast_admin_rights
func (c *Client) getMyDefaultAdministratorRights(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	resp, err := c.rpc.UsersGetFullUser(ctx, &tg.UsersGetFullUserRequest{
		ID: &tg.InputUserSelf{},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	// users.getFullUser returns *tg.UsersUserFull { FullUser, Chats, Users };
	// unwrap to the concrete *tg.UserFull (mirrors getChatPrivate in chat_info.go).
	uuf, ok := resp.(*tg.UsersUserFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected response type")
	}
	concrete, ok := uuf.FullUser.(*tg.UserFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected UserFull type")
	}
	forChannels := q.ArgBool("for_channels")
	var rights *tg.ChatAdminRights
	if forChannels {
		rights = concrete.BotBroadcastAdminRights
	} else {
		rights = concrete.BotGroupAdminRights
	}
	// adminRightsToMap(nil, …) yields all-false for the context's fields when no
	// default rights are configured.
	return adminRightsToMap(rights, forChannels), nil
}

// setMyDefaultAdministratorRights implements the Bot API setMyDefaultAdministratorRights method.
// Uses bots.setBotGroupDefaultAdminRights or bots.setBotBroadcastDefaultAdminRights.
func (c *Client) setMyDefaultAdministratorRights(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	rights := extractAdminRights(q)
	forChannels := q.ArgBool("for_channels")
	if forChannels {
		_, err := c.rpc.BotsSetBotBroadcastDefaultAdminRights(ctx, &tg.BotsSetBotBroadcastDefaultAdminRightsRequest{
			AdminRights: rights,
		})
		if err != nil {
			// TDLib ignores RIGHTS_NOT_MODIFIED and returns success.
			if rpcErr, ok := tgerr.As(err); ok && rpcErr.Message == "RIGHTS_NOT_MODIFIED" {
				return true, nil
			}
			return nil, rpcError(err)
		}
	} else {
		_, err := c.rpc.BotsSetBotGroupDefaultAdminRights(ctx, &tg.BotsSetBotGroupDefaultAdminRightsRequest{
			AdminRights: rights,
		})
		if err != nil {
			if rpcErr, ok := tgerr.As(err); ok && rpcErr.Message == "RIGHTS_NOT_MODIFIED" {
				return true, nil
			}
			return nil, rpcError(err)
		}
	}
	return true, nil
}

// adminRightsToMap converts a tg.ChatAdminRights to the Bot API response format,
// emitting only the fields relevant to the chat context, matching the reference
// json_store_administrator_rights (Client.cpp:17461): channel rights include
// can_post_messages/can_edit_messages/can_manage_direct_messages and exclude
// can_pin_messages/can_manage_topics/can_manage_tags; supergroup rights are the
// inverse. A nil r yields all-false for the context's fields.
func adminRightsToMap(r *tg.ChatAdminRights, isChannel bool) map[string]any {
	if r == nil {
		r = &tg.ChatAdminRights{}
	}
	m := map[string]any{
		"can_manage_chat":        r.Other,
		"can_change_info":        r.ChangeInfo,
		"can_delete_messages":    r.DeleteMessages,
		"can_invite_users":       r.InviteUsers,
		"can_restrict_members":   r.BanUsers,
		"can_promote_members":    r.AddAdmins,
		"can_manage_video_chats": r.ManageCall,
		"can_post_stories":       r.PostStories,
		"can_edit_stories":       r.EditStories,
		"can_delete_stories":     r.DeleteStories,
		"is_anonymous":           r.Anonymous,
	}
	if isChannel {
		m["can_post_messages"] = r.PostMessages
		m["can_edit_messages"] = r.EditMessages
		m["can_manage_direct_messages"] = r.ManageDirectMessages
	} else {
		m["can_pin_messages"] = r.PinMessages
		m["can_manage_topics"] = r.ManageTopics
		m["can_manage_tags"] = r.ManageRanks
	}
	return m
}

// setMyProfilePhoto implements the Bot API setMyProfilePhoto method.
// Uses photos.uploadProfilePhoto + photos.updateProfilePhoto.
// Requires a file_id from a prior upload or a multipart file.
func (c *Client) setMyProfilePhoto(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	photoFile, ok := q.File("photo")
	if !ok {
		return nil, NewError(400, "Bad Request: photo isn't specified")
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
	photoReq := &tg.PhotosUploadProfilePhotoRequest{
		File: inputFile,
	}
	photoReq.SetFlags()
	_, err = c.rpc.PhotosUploadProfilePhoto(ctx, photoReq)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// removeMyProfilePhoto implements the Bot API removeMyProfilePhoto method.
func (c *Client) removeMyProfilePhoto(ctx context.Context, q *server.Query) (any, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	_, err := c.rpc.PhotosDeletePhotos(ctx, &tg.PhotosDeletePhotosRequest{
		ID: []tg.InputPhotoClass{&tg.InputPhoto{}},
	})
	if err != nil {
		// TDLib catches BOT_METHOD_INVALID for bots and returns success.
		if rpcErr, ok := tgerr.As(err); ok && rpcErr.Message == "BOT_METHOD_INVALID" {
			return true, nil
		}
		return nil, rpcError(err)
	}
	return true, nil
}

// getUserProfilePhotos implements the Bot API getUserProfilePhotos method.
func (c *Client) getUserProfilePhotos(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	offset := int32(0)
	if o := q.Arg("offset"); o != "" {
		if v, err := strconv.ParseInt(o, 10, 32); err == nil {
			offset = int32(v)
		}
	}
	limit := int32(100)
	if l := q.Arg("limit"); l != "" {
		if v, err := strconv.ParseInt(l, 10, 32); err == nil {
			limit = int32(v)
		}
	}
	result, err := c.rpc.PhotosGetUserPhotos(ctx, &tg.PhotosGetUserPhotosRequest{
		UserID: &tg.InputUser{UserID: uid},
		Offset: offset,
		Limit:  limit,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return convert.UserProfilePhotos(result), nil
}

// getUserProfileAudios implements the Bot API getUserProfileAudios method.
// Uses users.getSavedMusic at the MTProto level.
func (c *Client) getUserProfileAudios(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	offset := int32(0)
	if o := q.Arg("offset"); o != "" {
		if v, err := strconv.ParseInt(o, 10, 32); err == nil {
			offset = int32(v)
		}
	}
	limit := int32(100)
	if l := q.Arg("limit"); l != "" {
		if v, err := strconv.ParseInt(l, 10, 32); err == nil {
			limit = int32(v)
		}
	}
	result, err := c.rpc.UsersGetSavedMusic(ctx, &tg.UsersGetSavedMusicRequest{
		ID:     &tg.InputUser{UserID: uid},
		Offset: offset,
		Limit:  limit,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return convert.SavedMusic(result), nil
}

// setPassportDataErrors implements the Bot API setPassportDataErrors method.
// Uses users.setSecureValueErrors at the MTProto level.
func (c *Client) setPassportDataErrors(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	errorsJSON := q.Arg("errors")
	if errorsJSON == "" {
		return nil, NewError(400, "Bad Request: parameter \"errors\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	inputErrors, err := convert.SecureValueErrors(errorsJSON)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	_, err = c.rpc.UsersSetSecureValueErrors(ctx, &tg.UsersSetSecureValueErrorsRequest{
		ID:     &tg.InputUser{UserID: uid},
		Errors: inputErrors,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// getCustomEmojiStickers implements the Bot API getCustomEmojiStickers method.
func (c *Client) getCustomEmojiStickers(ctx context.Context, q *server.Query) (any, error) {
	idsJSON := q.Arg("custom_emoji_ids")
	if idsJSON == "" {
		return nil, NewError(400, "Bad Request: parameter \"custom_emoji_ids\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	var ids []string
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return nil, NewError(400, "Bad Request: invalid custom_emoji_ids JSON")
	}
	intIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		v, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return nil, NewError(400, "Bad Request: invalid custom emoji id: "+id)
		}
		intIDs = append(intIDs, v)
	}
	result, err := c.rpc.MessagesGetCustomEmojiDocuments(ctx, &tg.MessagesGetCustomEmojiDocumentsRequest{
		DocumentID: intIDs,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	// Resolve sticker set short names for custom emoji documents. The documents
	// reference their set by InputStickerSetID; TDLib resolves this internally.
	// We resolve via messages.getStickerSet and rewrite the attribute to
	// InputStickerSetShortName so the converter picks up the set_name.
	if vec, ok := result.(*tg.GenericVector); ok {
		resolved := make(map[int64]string) // setID → short_name
		for _, item := range vec.Items {
			doc, ok := item.(*tg.Document)
			if !ok {
				continue
			}
			for _, attr := range doc.Attributes {
				ce, ok := attr.(*tg.DocumentAttributeCustomEmoji)
				if !ok || ce.Stickerset == nil {
					continue
				}
				sid, ok := ce.Stickerset.(*tg.InputStickerSetID)
				if !ok {
					continue
				}
				if _, cached := resolved[sid.ID]; cached {
					continue
				}
				ss, err := c.rpc.MessagesGetStickerSet(ctx, &tg.MessagesGetStickerSetRequest{
					Stickerset: &tg.InputStickerSetID{ID: sid.ID, AccessHash: sid.AccessHash},
				})
				if err != nil {
					continue
				}
				if mss, ok := ss.(*tg.MessagesStickerSet); ok {
					if s, ok := mss.Set.(*tg.StickerSet); ok {
						resolved[sid.ID] = s.ShortName
					}
				}
			}
		}
		// Inject resolved names by rewriting the attribute.
		for _, item := range vec.Items {
			doc, ok := item.(*tg.Document)
			if !ok {
				continue
			}
			for _, attr := range doc.Attributes {
				ce, ok := attr.(*tg.DocumentAttributeCustomEmoji)
				if !ok || ce.Stickerset == nil {
					continue
				}
				if sid, ok := ce.Stickerset.(*tg.InputStickerSetID); ok {
					if name, found := resolved[sid.ID]; found {
						ce.Stickerset = &tg.InputStickerSetShortName{ShortName: name}
					}
				}
			}
		}
	}
	return convert.CustomEmojiStickers(result), nil
}

// setUserEmojiStatus implements the Bot API setUserEmojiStatus method.
func (c *Client) setUserEmojiStatus(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	emojiStatusID := q.Arg("emoji_status_custom_emoji_id")
	var emojiStatus *tg.EmojiStatus
	if emojiStatusID != "" {
		customID, err := strconv.ParseInt(emojiStatusID, 10, 64)
		if err != nil {
			return nil, NewError(400, "Bad Request: invalid emoji_status_custom_emoji_id")
		}
		emojiStatus = &tg.EmojiStatus{DocumentID: customID}
		if exp := q.Arg("emoji_status_expiration_date"); exp != "" {
			if until, err := strconv.ParseInt(exp, 10, 32); err == nil && until != 0 {
				emojiStatus.Until = int32(until)
			}
		}
	}
	// bots.updateUserEmojiStatus sets the TARGET user's emoji status (managed/business
	// bot). Was incorrectly AccountUpdateEmojiStatus (the bot's own status). USER_PERMISSION_DENIED
	// → "Not enough rights to change the user's emoji status" (handled in translateErrorMessage).
	_, err = c.rpc.BotsUpdateUserEmojiStatus(ctx, &tg.BotsUpdateUserEmojiStatusRequest{
		UserID:      c.resolveInputUser(ctx, uid),
		EmojiStatus: emojiStatus,
	})
	if err != nil {
		return nil, rpcErrorWith(err, "", "setuseremojistatus")
	}
	return true, nil
}

// getUserPersonalChatMessages implements the Bot API getUserPersonalChatMessages method.
// Uses messages.getPersonalChannelHistory at the MTProto level.
func (c *Client) getUserPersonalChatMessages(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	limit := int32(0)
	if l := q.Arg("limit"); l != "" {
		if v, err := strconv.ParseInt(l, 10, 32); err == nil {
			limit = int32(v)
		}
	}
	result, err := c.rpc.MessagesGetPersonalChannelHistory(ctx, &tg.MessagesGetPersonalChannelHistoryRequest{
		UserID: &tg.InputUser{UserID: uid},
		Limit:  limit,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return convert.Messages(result), nil
}

// getManagedBotToken implements the Bot API getManagedBotToken method.
// Uses bots.exportBotToken at the MTProto level.
func (c *Client) getManagedBotToken(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	result, err := c.rpc.BotsExportBotToken(ctx, &tg.BotsExportBotTokenRequest{
		Bot:    &tg.InputUser{UserID: uid},
		Revoke: q.ArgBool("revoke"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	// The reference returns the token as a bare JSON string (TdOnGetBotTokenCallback
	// → VirtuallyJsonableString → JsonString), i.e. {"ok":true,"result":"<token>"}.
	return result.Token, nil
}

// getManagedBotAccessSettings implements the Bot API getManagedBotAccessSettings method.
// Uses bots.getAccessSettings at the MTProto level.
func (c *Client) getManagedBotAccessSettings(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	result, err := c.rpc.BotsGetAccessSettings(ctx, &tg.BotsGetAccessSettingsRequest{
		Bot: &tg.InputUser{UserID: uid},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	addedUserIDs := make([]int64, 0, len(result.AddUsers))
	for _, u := range result.AddUsers {
		if user, ok := u.(*tg.User); ok {
			addedUserIDs = append(addedUserIDs, user.ID)
		}
	}
	return map[string]any{
		"is_access_restricted": result.Restricted,
		"added_user_ids":       addedUserIDs,
	}, nil
}

// setManagedBotAccessSettings implements the Bot API setManagedBotAccessSettings method.
// Uses bots.editAccessSettings at the MTProto level.
func (c *Client) setManagedBotAccessSettings(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	isRestricted := q.ArgBool("is_access_restricted")
	req := &tg.BotsEditAccessSettingsRequest{
		Restricted: isRestricted,
		Bot:        &tg.InputUser{UserID: uid},
	}
	if addedUserIDs := q.Arg("added_user_ids"); addedUserIDs != "" {
		var ids []int64
		if err := json.Unmarshal([]byte(addedUserIDs), &ids); err == nil {
			for _, id := range ids {
				req.AddUsers = append(req.AddUsers, &tg.InputUser{UserID: id})
			}
			req.Flags.Set(1)
		}
	}
	req.SetFlags()
	_, err = c.rpc.BotsEditAccessSettings(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// replaceManagedBotToken implements the Bot API replaceManagedBotToken method.
// NOTE: This method does not exist in TDLib or the TL schema.
func (c *Client) replaceManagedBotToken(ctx context.Context, q *server.Query) (any, error) {
	// Same as getManagedBotToken but with Revoke=true: revoke the old token and
	// issue a fresh one. The reference implements this as
	// td_api::getManagedBotToken(user_id, /*revoke=*/true), which TDLib maps to
	// bots.exportBotToken#bd0d99eb with the revoke flag set. TDLib is an MTProto
	// wrapper, so this is a plain raw-TL call — not a "TDLib-only" capability.
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	result, err := c.rpc.BotsExportBotToken(ctx, &tg.BotsExportBotTokenRequest{
		Bot:    &tg.InputUser{UserID: uid},
		Revoke: true,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return result.Token, nil
}

// parseBotCommandScope parses the Bot API scope parameter into a tg.BotCommandScopeClass.
func parseBotCommandScope(q *server.Query) (tg.BotCommandScopeClass, string) {
	langCode := q.Arg("language_code")
	scopeJSON := q.Arg("scope")
	if scopeJSON == "" {
		return &tg.BotCommandScopeDefault{}, langCode
	}
	var scope struct {
		Type   string `json:"type"`
		ChatID int64  `json:"chat_id"`
	}
	if err := json.Unmarshal([]byte(scopeJSON), &scope); err != nil {
		return &tg.BotCommandScopeDefault{}, langCode
	}
	switch scope.Type {
	case "all_private_chats":
		return &tg.BotCommandScopeUsers{}, langCode
	case "all_group_chats":
		return &tg.BotCommandScopeChats{}, langCode
	case "all_chat_administrators":
		return &tg.BotCommandScopeChatAdmins{}, langCode
	case "chat":
		return &tg.BotCommandScopePeer{Peer: peerFromChatID(scope.ChatID)}, langCode
	case "chat_administrators":
		return &tg.BotCommandScopePeerAdmins{Peer: peerFromChatID(scope.ChatID)}, langCode
	case "chat_member":
		return &tg.BotCommandScopePeerUser{Peer: peerFromChatID(scope.ChatID), UserID: &tg.InputUser{UserID: scope.ChatID}}, langCode
	default:
		return &tg.BotCommandScopeDefault{}, langCode
	}
}

func peerFromChatID(chatID int64) tg.InputPeerClass {
	if chatID > 0 {
		return &tg.InputPeerUser{UserID: chatID}
	}
	s := strconv.FormatInt(chatID, 10)
	if strings.HasPrefix(s, "-100") {
		cid, _ := strconv.ParseInt(s[4:], 10, 64)
		return &tg.InputPeerChannel{ChannelID: cid}
	}
	return &tg.InputPeerChat{ChatID: -chatID}
}

// parseBotCommands parses the Bot API commands JSON array.
func parseBotCommands(jsonStr string) ([]*tg.BotCommand, error) {
	var cmds []struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &cmds); err != nil {
		return nil, err
	}
	result := make([]*tg.BotCommand, 0, len(cmds))
	for _, c := range cmds {
		result = append(result, &tg.BotCommand{
			Command:     c.Command,
			Description: c.Description,
		})
	}
	return result, nil
}

// extractBotCommands extracts BotCommand list from a Vector<BotCommand> response
// (bots.getBotCommands returns a GenericVector of BotCommand objects).
func extractBotCommands(result tg.TLObject) []map[string]string {
	vec, ok := result.(*tg.GenericVector)
	if !ok {
		return []map[string]string{}
	}
	cmds := make([]map[string]string, 0, len(vec.Items))
	for _, item := range vec.Items {
		c, ok := item.(*tg.BotCommand)
		if !ok {
			continue
		}
		cmds = append(cmds, map[string]string{
			"command":     c.Command,
			"description": c.Description,
		})
	}
	return cmds
}
