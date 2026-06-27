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
	Register("getchat", (*Client).getChat)
	Register("getchatadministrators", (*Client).getChatAdministrators)
	Register("getchatmember", (*Client).getChatMember)
	Register("getchatmembercount", (*Client).getChatMemberCount)
	Register("leavechat", (*Client).leaveChat)
}

// getChat implements the Bot API getChat method.
func (c *Client) getChat(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, err
	}

	switch chatType {
	case "user":
		return c.getChatPrivate(ctx, id)
	case "chat":
		return c.getChatGroup(ctx, id)
	case "channel":
		return c.getChatChannel(ctx, id)
	default:
		return nil, NewError(400, "Bad Request: invalid chat_id")
	}
}

func (c *Client) getChatGroup(ctx context.Context, chatID int64) (any, error) {
	fullChat, err := c.rpc.MessagesGetFullChat(ctx, &tg.MessagesGetFullChatRequest{ChatID: chatID})
	if err != nil {
		return nil, rpcErrorDefault(err, "chat not found")
	}
	mcf, ok := fullChat.(*tg.MessagesChatFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected response type")
	}
	cf, ok := mcf.FullChat.(*tg.ChatFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected chat full type")
	}
	var chat *tg.Chat
	for _, ch := range mcf.Chats {
		if cc, ok := ch.(*tg.Chat); ok && cc.ID == chatID {
			chat = cc
			break
		}
	}
	// Warm the check_chat_access cache (cold-start) from the fetched chat.
	if chat != nil {
		var migratedTo string
		if ic, ok := chat.MigratedTo.(*tg.InputChannel); ok && ic.ChannelID != 0 {
			migratedTo = "-100" + strconv.FormatInt(ic.ChannelID, 10)
		}
		c.saveChatFlags(chatID, false, chat.Deactivated, migratedTo)
		if chat.Left {
			c.saveBotMemberStatus(chatID, "left")
		}
	}
	return convert.ChatFullInfoFromChatFull(cf, chat), nil
}

// getChatPrivate implements getChat for private chats (positive chat_id = user
// ID). Calls users.getFullUser and maps the result to ChatFullInfo.
func (c *Client) getChatPrivate(ctx context.Context, userID int64) (any, error) {
	inputUser := c.resolveInputUser(ctx, userID)
	resp, err := c.rpc.UsersGetFullUser(ctx, &tg.UsersGetFullUserRequest{ID: inputUser})
	if err != nil {
		return nil, rpcErrorDefault(err, "chat not found")
	}
	uuf, ok := resp.(*tg.UsersUserFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected response type")
	}
	uf, ok := uuf.FullUser.(*tg.UserFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected user full type")
	}
	var user *tg.User
	for _, uc := range uuf.Users {
		if u, ok := uc.(*tg.User); ok && u.ID == userID {
			user = u
			break
		}
	}
	if user == nil {
		return nil, NewError(400, "Bad Request: user not found")
	}
	return convert.ChatFullInfoFromUserFull(uf, user), nil
}

func (c *Client) getChatChannel(ctx context.Context, id int64) (any, error) {
	channelID := extractChannelID(id)

	inputCh, err := c.resolveInputChannel(ctx, channelID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	fullChannel, err := c.rpc.ChannelsGetFullChannel(ctx, &tg.ChannelsGetFullChannelRequest{Channel: inputCh})
	if err != nil {
		return nil, rpcErrorDefault(err, "chat not found")
	}
	mcf, ok := fullChannel.(*tg.MessagesChatFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected response type")
	}
	cf, ok := mcf.FullChat.(*tg.ChannelFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected channel full type")
	}
	var ch *tg.Channel
	for _, cc := range mcf.Chats {
		if c2, ok := cc.(*tg.Channel); ok && c2.ID == channelID {
			ch = c2
			break
		}
	}
	// Warm the check_chat_access cache (cold-start): megagroup flag + left state.
	if ch != nil {
		c.saveChatFlags(channelID, ch.Megagroup, false, "")
		if ch.Left {
			c.saveBotMemberStatus(channelID, "left")
		}
	}
	return convert.ChatFullInfoFromChannelFull(cf, ch), nil
}

// getChatAdministrators implements the Bot API getChatAdministrators method.
func (c *Client) getChatAdministrators(ctx context.Context, q *server.Query) (any, error) {
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
	case "user":
		return nil, NewError(400, "Bad Request: no administrators in a private chat")
	case "chat":
		return c.getChatAdminsGroup(ctx, id)
	case "channel":
		return c.getChatAdminsChannel(ctx, id)
	default:
		return nil, NewError(400, "Bad Request: invalid chat_id")
	}
}

func (c *Client) getChatAdminsGroup(ctx context.Context, chatID int64) (any, error) {
	fullChat, err := c.rpc.MessagesGetFullChat(ctx, &tg.MessagesGetFullChatRequest{ChatID: chatID})
	if err != nil {
		return nil, rpcErrorDefault(err, "chat not found")
	}
	mcf, ok := fullChat.(*tg.MessagesChatFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected response type")
	}
	cf, ok := mcf.FullChat.(*tg.ChatFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected chat full type")
	}
	participants, ok := cf.Participants.(*tg.ChatParticipants)
	if !ok {
		return []any{}, nil
	}
	users := usersFromUserClasses(mcf.Users)
	var result []any
	for _, p := range participants.Participants {
		m := convert.ChatMemberFromChatParticipant(p, users)
		if m != nil && (m.Status == "creator" || m.Status == "administrator") {
			m.SetChatType(apitypes.ChatTypeGroup)
			result = append(result, m)
		}
	}
	if result == nil {
		result = []any{}
	}
	return result, nil
}

func (c *Client) getChatAdminsChannel(ctx context.Context, id int64) (any, error) {
	channelID := extractChannelID(id)
	inputCh, err := c.resolveInputChannel(ctx, channelID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	result, err := c.rpc.ChannelsGetParticipants(ctx, &tg.ChannelsGetParticipantsRequest{
		Channel: inputCh,
		Filter:  &tg.ChannelParticipantsAdmins{},
		Offset:  0,
		Limit:   200,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	part, ok := result.(*tg.ChannelsChannelParticipants)
	if !ok {
		return []any{}, nil
	}
	users := usersFromUserClasses(part.Users)
	var members []any
	for _, p := range part.Participants {
		m := convert.ChatMemberFromParticipant(p, users)
		if m != nil {
			members = append(members, m)
		}
	}
	if members == nil {
		members = []any{}
	}
	return members, nil
}

// getChatMember implements the Bot API getChatMember method.
func (c *Client) getChatMember(ctx context.Context, q *server.Query) (any, error) {
	userID := q.Arg("user_id")
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil || uid <= 0 {
		return nil, NewError(400, "Bad Request: invalid user_id specified")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}

	id, chatType, err := parseChatID(chatID)
	if err != nil {
		return nil, err
	}

	switch chatType {
	case "user":
		return c.getChatMemberPrivate(ctx, uid)
	case "chat":
		return c.getChatMemberGroup(ctx, id, uid)
	case "channel":
		return c.getChatMemberChannel(ctx, id, uid)
	default:
		return nil, NewError(400, "Bad Request: invalid chat_id")
	}
}

// getChatMemberPrivate handles getChatMember for private (1:1) chats.
// The reference Bot API returns the user with status "member".
func (c *Client) getChatMemberPrivate(ctx context.Context, userID int64) (any, error) {
	botUID, _ := strconv.ParseInt(c.botID, 10, 64)
	var inputUser tg.InputUserClass
	if userID == botUID {
		inputUser = &tg.InputUserSelf{}
	} else {
		inputUser = c.resolveInputUser(ctx, userID)
	}
	full, err := c.rpc.UsersGetFullUser(ctx, &tg.UsersGetFullUserRequest{ID: inputUser})
	if err != nil {
		return nil, rpcError(err)
	}
	uf, ok := full.(*tg.UsersUserFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected response type")
	}
	for _, u := range uf.Users {
		if user, ok := u.(*tg.User); ok && user.ID == userID {
			return &apitypes.ChatMember{Status: "member", User: convert.User(user)}, nil
		}
	}
	// Fallback: first user in the response.
	for _, u := range uf.Users {
		if user, ok := u.(*tg.User); ok {
			return &apitypes.ChatMember{Status: "member", User: convert.User(user)}, nil
		}
	}
	return nil, NewError(400, "Bad Request: user not found")
}

func (c *Client) getChatMemberGroup(ctx context.Context, chatID, userID int64) (any, error) {
	fullChat, err := c.rpc.MessagesGetFullChat(ctx, &tg.MessagesGetFullChatRequest{ChatID: chatID})
	if err != nil {
		return nil, rpcErrorDefault(err, "chat not found")
	}
	mcf, ok := fullChat.(*tg.MessagesChatFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected response type")
	}
	cf, ok := mcf.FullChat.(*tg.ChatFull)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected chat full type")
	}
	participants, ok := cf.Participants.(*tg.ChatParticipants)
	if !ok {
		return nil, NewError(400, "Bad Request: participant list not available")
	}
	users := usersFromUserClasses(mcf.Users)
	for _, p := range participants.Participants {
		m := convert.ChatMemberFromChatParticipant(p, users)
		if m != nil && m.User != nil && m.User.ID == userID {
			m.SetChatType(apitypes.ChatTypeGroup)
			return m, nil
		}
	}
	return nil, NewError(400, "Bad Request: user is not a member of this chat")
}

func (c *Client) getChatMemberChannel(ctx context.Context, id, userID int64) (any, error) {
	channelID := extractChannelID(id)
	inputCh, err := c.resolveInputChannel(ctx, channelID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	peer := &tg.InputPeerUser{UserID: userID}
	result, err := c.rpc.ChannelsGetParticipant(ctx, &tg.ChannelsGetParticipantRequest{
		Channel:     inputCh,
		Participant: peer,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	part, ok := result.(*tg.ChannelsChannelParticipant)
	if !ok {
		return nil, NewError(500, "Internal Server Error: unexpected response type")
	}
	users := usersFromUserClasses(part.Users)
	return convert.ChatMemberFromParticipant(part.Participant, users), nil
}

// getChatMemberCount implements the Bot API getChatMemberCount method.
func (c *Client) getChatMemberCount(ctx context.Context, q *server.Query) (any, error) {
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
	case "user":
		return 1, nil
	case "chat":
		return c.getMemberCountGroup(ctx, id)
	case "channel":
		return c.getMemberCountChannel(ctx, id)
	default:
		return nil, NewError(400, "Bad Request: invalid chat_id")
	}
}

func (c *Client) getMemberCountGroup(ctx context.Context, chatID int64) (any, error) {
	fullChat, err := c.rpc.MessagesGetFullChat(ctx, &tg.MessagesGetFullChatRequest{ChatID: chatID})
	if err != nil {
		return nil, rpcErrorDefault(err, "chat not found")
	}
	mcf, ok := fullChat.(*tg.MessagesChatFull)
	if !ok {
		return 0, nil
	}
	cf, ok := mcf.FullChat.(*tg.ChatFull)
	if !ok {
		return 0, nil
	}
	if participants, ok := cf.Participants.(*tg.ChatParticipants); ok {
		return len(participants.Participants), nil
	}
	return 0, nil
}

func (c *Client) getMemberCountChannel(ctx context.Context, id int64) (any, error) {
	channelID := extractChannelID(id)
	inputCh, err := c.resolveInputChannel(ctx, channelID)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	fullChannel, err := c.rpc.ChannelsGetFullChannel(ctx, &tg.ChannelsGetFullChannelRequest{Channel: inputCh})
	if err != nil {
		return nil, rpcErrorDefault(err, "chat not found")
	}
	mcf, ok := fullChannel.(*tg.MessagesChatFull)
	if !ok {
		return 0, nil
	}
	cf, ok := mcf.FullChat.(*tg.ChannelFull)
	if !ok {
		return 0, nil
	}
	if cf.ParticipantsCount != 0 {
		return int(cf.ParticipantsCount), nil
	}
	return 0, nil
}

// leaveChat implements the Bot API leaveChat method.
func (c *Client) leaveChat(ctx context.Context, q *server.Query) (any, error) {
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
	case "user":
		return nil, NewError(400, "Bad Request: can't leave a private chat")
	case "chat":
		_, err = c.rpc.MessagesDeleteChatUser(ctx, &tg.MessagesDeleteChatUserRequest{
			ChatID: id,
			UserID: &tg.InputUserSelf{},
		})
	case "channel":
		channelID := extractChannelID(id)
		inputCh, err2 := c.resolveInputChannel(ctx, channelID)
		if err2 != nil {
			return nil, NewError(400, "Bad Request: "+err2.Error())
		}
		_, err = c.rpc.ChannelsLeaveChannel(ctx, &tg.ChannelsLeaveChannelRequest{Channel: inputCh})
	default:
		return nil, NewError(400, "Bad Request: invalid chat_id")
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// parseChatID parses a chat_id string and returns the numeric ID plus the chat type.
func parseChatID(chatIDStr string) (int64, string, error) {
	if chatIDStr == "" {
		return 0, "", &Error{Code: 400, Description: "Bad Request: chat_id is empty"}
	}
	if strings.HasPrefix(chatIDStr, "@") {
		return 0, "", &Error{Code: 400, Description: "Bad Request: username chat_id not supported for this method"}
	}
	id, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return 0, "", &Error{Code: 400, Description: "Bad Request: chat not found"}
	}
	if id == 0 {
		return 0, "", &Error{Code: 400, Description: "Bad Request: chat not found"}
	}
	if id < 0 {
		s := strconv.FormatInt(id, 10)
		if strings.HasPrefix(s, "-100") {
			return id, "channel", nil
		}
		return id, "chat", nil
	}
	return id, "user", nil
}

// extractChannelID extracts the raw channel ID from a -100xxxxx chat_id.
func extractChannelID(id int64) int64 {
	s := strconv.FormatInt(id, 10)
	if strings.HasPrefix(s, "-100") {
		cid, _ := strconv.ParseInt(s[4:], 10, 64)
		return cid
	}
	return id
}

// resolveInputChannel builds an InputChannel from the channel ID, using the
// peer cache for access_hash when available.
func (c *Client) resolveInputChannel(ctx context.Context, channelID int64) (tg.InputChannelClass, error) {
	if c.store != nil {
		if p, err := c.store.GetPeer(ctx, channelID); err == nil {
			return &tg.InputChannel{ChannelID: channelID, AccessHash: p.AccessHash}, nil
		}
	}
	return &tg.InputChannel{ChannelID: channelID}, nil
}

// resolveInputUser builds an InputUser from the user ID, using the peer cache
// for access_hash when available.
func (c *Client) resolveInputUser(ctx context.Context, userID int64) tg.InputUserClass {
	if c.store != nil {
		if p, err := c.store.GetPeer(ctx, userID); err == nil {
			return &tg.InputUser{UserID: userID, AccessHash: p.AccessHash}
		}
	}
	return &tg.InputUser{UserID: userID}
}

// usersFromUserClasses collects users from a []UserClass slice.
func usersFromUserClasses(users []tg.UserClass) map[int64]*tg.User {
	m := make(map[int64]*tg.User)
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			m[user.ID] = user
		}
	}
	return m
}
