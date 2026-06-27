package convert

import (
	"math"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// Message converts a raw *tg.Message (layer 225+) into the Bot API
// types.Message. Handles core fields (text, chat, from, reply, entities),
// media (photo/video/audio/animation/document/voice/video_note/sticker),
// rich_message, and service messages.
//
// Field gotchas:
//   - tg.Message.ID is int32, Bot API message_id is int64
//   - tg.Message.Message (the text body) → types.Message.Text
//   - tg.Message.PeerID → Chat (resolved via peerToChat)
//   - tg.Message.FromID → From or SenderChat (PeerClass → User or Chat)
//   - tg.Message.Entities → MessageEntity (different field layout)
//   - tg.Message.Media → various media fields (MessageMediaClass)
//   - tg.Message.FwdFrom → forward_origin (restructured in Bot API 7.x)
//   - tg.Message.ReplyTo → reply_to_message (content resolved from the per-bot
//     msgCache in the client layer — cacheMessageAndReply in updates.go — for
//     messages the bot has previously seen)
//   - tg.Message.ReplyMarkup → reply_markup (InlineKeyboardMarkup)
func Message(m *tg.Message) *apitypes.Message {
	if m == nil {
		return nil
	}

	out := &apitypes.Message{
		MessageID:           int64(m.ID),
		Date:                int64(m.Date),
		EditDate:            int64(m.EditDate),
		HasProtectedContent: m.Noforwards,
		IsFromOffline:       m.Offline,
		AuthorSignature:     m.PostAuthor,
		SenderBoostCount:    int(m.FromBoostsApplied),
		SenderTag:           m.FromRank,
		IsPaidPost:          m.PaidSuggestedPostStars || m.PaidSuggestedPostTon,
	}
	// is_automatic_forward: the reference (Client.cpp:4701) emits this only for
	// messages auto-forwarded from a linked channel to its discussion group.
	// mtgo's Message exposes no signal for this; the previous m.FromScheduled
	// proxy was wrong (scheduled ≠ automatic forward) and emitted spurious
	// true for every scheduled message. Left unset until linked-channel
	// detection is implemented (follow-up).

	// Effect ID.
	if m.Effect != 0 {
		out.EffectID = formatInt64(m.Effect)
	}

	// Paid message stars.
	if m.PaidMessageStars > 0 {
		out.PaidStarCount = m.PaidMessageStars
	}

	// Chat: resolved from PeerID.
	out.Chat = peerToChat(m.PeerID)

	// From: resolved from FromID (PeerClass).
	// If FromID is a user → populate From; if it's a chat → populate SenderChat.
	// For outgoing messages (m.Out), the sender is the bot itself.
	if m.FromID != nil {
		switch from := m.FromID.(type) {
		case *tg.PeerUser:
			out.From = &apitypes.User{ID: from.UserID, IsBot: m.Out}
		case *tg.PeerChat:
			out.SenderChat = &apitypes.Chat{ID: from.ChatID}
		case *tg.PeerChannel:
			out.SenderChat = &apitypes.Chat{ID: from.ChannelID}
		}
	}

	// Entities.
	if len(m.Entities) > 0 {
		out.Entities = make([]apitypes.MessageEntity, 0, len(m.Entities))
		for _, e := range m.Entities {
			if ent := messageEntity(e); ent != nil {
				out.Entities = append(out.Entities, *ent)
			}
		}
	}

	// Media: extract text/media content from MessageMediaClass.
	if m.Media != nil {
		convertMedia(m.Media, out)
	}

	// m.Message is the caption for caption-bearing media (photo/video/document/
	// audio/animation/voice) and the body text otherwise. Route it accordingly.
	if hasCaptionMedia(out) {
		out.Caption = m.Message
	} else {
		out.Text = m.Message
	}

	// Rich message content. Full PageBlock → RichBlock rendering via
	// convert.RichMessage (mirrors the official JsonRichMessage output:
	// {blocks:[...], is_rtl?}).
	if m.RichMessage != nil {
		out.RichMessage = RichMessage(m.RichMessage)
	}

	// Forward info: tg.Message.FwdFrom → forward_origin + legacy fields.
	if m.FwdFrom != nil {
		out.ForwardOrigin = convertFwdFrom(m.FwdFrom)
		out.ForwardDate = int64(m.FwdFrom.Date)
		// Legacy forward fields (still emitted by reference alongside forward_origin).
		if m.FwdFrom.FromID != nil {
			switch from := m.FwdFrom.FromID.(type) {
			case *tg.PeerUser:
				out.ForwardFrom = &apitypes.User{ID: from.UserID}
			case *tg.PeerChat:
				out.ForwardFromChat = &apitypes.Chat{ID: from.ChatID, Type: apitypes.ChatTypeGroup}
			case *tg.PeerChannel:
				out.ForwardFromChat = &apitypes.Chat{ID: -1000000000000 - from.ChannelID, Type: apitypes.ChatTypeChannel}
			}
		}
		if m.FwdFrom.ChannelPost != 0 {
			out.ForwardFromMessageID = int64(m.FwdFrom.ChannelPost)
		}
		if m.FwdFrom.PostAuthor != "" {
			out.ForwardSignature = m.FwdFrom.PostAuthor
		}
		if m.FwdFrom.FromName != "" {
			out.ForwardSenderName = m.FwdFrom.FromName
		}
	}

	// Reply markup (inline keyboard).
	if m.ReplyMarkup != nil {
		out.ReplyMarkup = convertReplyMarkup(m.ReplyMarkup)
	}

	// Reply header: tg.Message.ReplyTo carries the replied-to message id and the
	// checklist-task / poll-option context. reply_to_message content is resolved
	// from the per-bot msgCache by the client layer (cacheMessageAndReply in
	// updates.go) for messages the bot has previously seen; here we only wire the
	// directly-available scalar fields.
	if m.ReplyTo != nil {
		if rh, ok := m.ReplyTo.(*tg.MessageReplyHeader); ok {
			if rh.TodoItemID != 0 {
				out.ReplyToChecklistTaskID = int(rh.TodoItemID)
			}
			// rh.PollOption is the option bytes; reply_to_poll_option_id is the
			// option INDEX, which needs the poll's option list to resolve -> deferred.
		}
	}

	// Via bot.
	if m.ViaBotID != 0 {
		out.ViaBot = &apitypes.User{ID: m.ViaBotID}
	}

	// Grouped ID (media group).
	if m.GroupedID != 0 {
		out.MediaGroupID = formatInt64(m.GroupedID)
	}

	// TTL period.
	// (Not directly in Message struct yet; skip for MVP.)

	return out
}

// peerToChat converts a tg.PeerClass into a Bot API Chat.
func peerToChat(p tg.PeerClass) apitypes.Chat {
	if p == nil {
		return apitypes.Chat{}
	}
	switch peer := p.(type) {
	case *tg.PeerUser:
		return apitypes.Chat{ID: peer.UserID, Type: apitypes.ChatTypePrivate}
	case *tg.PeerChat:
		return apitypes.Chat{ID: peer.ChatID, Type: apitypes.ChatTypeGroup}
	case *tg.PeerChannel:
		return apitypes.Chat{ID: -1000000000000 - peer.ChannelID, Type: apitypes.ChatTypeSupergroup}
	default:
		return apitypes.Chat{}
	}
}

// messageEntity converts a tg.MessageEntityClass to a Bot API MessageEntity.
func messageEntity(e tg.MessageEntityClass) *apitypes.MessageEntity {
	if e == nil {
		return nil
	}
	switch ent := e.(type) {
	case *tg.MessageEntityMention:
		return &apitypes.MessageEntity{Type: "mention", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityHashtag:
		return &apitypes.MessageEntity{Type: "hashtag", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityBotCommand:
		return &apitypes.MessageEntity{Type: "bot_command", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityURL:
		return &apitypes.MessageEntity{Type: "url", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityEmail:
		return &apitypes.MessageEntity{Type: "email", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityBold:
		return &apitypes.MessageEntity{Type: "bold", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityItalic:
		return &apitypes.MessageEntity{Type: "italic", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityCode:
		return &apitypes.MessageEntity{Type: "code", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityPre:
		return &apitypes.MessageEntity{Type: "pre", Offset: int(ent.Offset), Length: int(ent.Length), Language: ent.Language}
	case *tg.MessageEntityTextURL:
		return &apitypes.MessageEntity{Type: "text_link", Offset: int(ent.Offset), Length: int(ent.Length), URL: ent.URL}
	case *tg.MessageEntityMentionName:
		return &apitypes.MessageEntity{Type: "text_mention", Offset: int(ent.Offset), Length: int(ent.Length), User: &apitypes.User{ID: ent.UserID}}
	case *tg.MessageEntityUnderline:
		return &apitypes.MessageEntity{Type: "underline", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityStrike:
		return &apitypes.MessageEntity{Type: "strikethrough", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityBlockquote:
		// MTProto messageEntityBlockquote "collapsed" flag → Bot API "expandable_blockquote".
		etype := "blockquote"
		if ent.Collapsed {
			etype = "expandable_blockquote"
		}
		return &apitypes.MessageEntity{Type: etype, Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityCashtag:
		return &apitypes.MessageEntity{Type: "cashtag", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityPhone:
		return &apitypes.MessageEntity{Type: "phone_number", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntitySpoiler:
		return &apitypes.MessageEntity{Type: "spoiler", Offset: int(ent.Offset), Length: int(ent.Length)}
	case *tg.MessageEntityCustomEmoji:
		return &apitypes.MessageEntity{Type: "custom_emoji", Offset: int(ent.Offset), Length: int(ent.Length), CustomEmojiID: formatInt64(ent.DocumentID)}
	default:
		// MessageEntityBankCard and MessageEntityMediaTimestamp are intentionally
		// dropped to match the reference (Client.cpp:636-637 suppresses them).
		return nil
	}
}

// hasCaptionMedia reports whether the message carries a media type whose
// text is rendered as a caption (photo/video/document/audio/animation/voice).
func hasCaptionMedia(out *apitypes.Message) bool {
	return len(out.Photo) > 0 ||
		out.Document != nil ||
		out.Video != nil ||
		out.Audio != nil ||
		out.Animation != nil ||
		out.Voice != nil
}

// convertMedia extracts media content from tg.MessageMediaClass into the
// appropriate Message fields.
func convertMedia(media tg.MessageMediaClass, out *apitypes.Message) {
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		if m.Photo != nil {
			if photo, ok := m.Photo.(*tg.Photo); ok {
				out.Photo = Photo(photo)
			}
		}
	case *tg.MessageMediaDocument:
		if m.Document != nil {
			if doc, ok := m.Document.(*tg.Document); ok {
				DocumentMedia(doc, out)
			}
		}
	case *tg.MessageMediaGeo:
		if m.Geo != nil {
			if geo, ok := m.Geo.(*tg.GeoPoint); ok {
				loc := geoLocation(geo)
				out.Location = &loc
			}
		}
	case *tg.MessageMediaGeoLive:
		if m.Geo != nil {
			if geo, ok := m.Geo.(*tg.GeoPoint); ok {
				loc := geoLocation(geo)
				loc.LivePeriod = int(m.Period)
				if m.Heading != 0 {
					loc.Heading = int(m.Heading)
				}
				if m.ProximityNotificationRadius != 0 {
					loc.ProximityAlertRadius = int(m.ProximityNotificationRadius)
				}
				out.Location = &loc
			}
		}
	case *tg.MessageMediaContact:
		out.Contact = &apitypes.Contact{
			PhoneNumber: m.PhoneNumber,
			FirstName:   m.FirstName,
			LastName:    m.LastName,
			UserID:      m.UserID,
		}
	case *tg.MessageMediaVenue:
		out.Venue = &apitypes.Venue{
			Title:   m.Title,
			Address: m.Address,
		}
		if m.Geo != nil {
			if geo, ok := m.Geo.(*tg.GeoPoint); ok {
				// A venue message carries the venue's location both as the
				// top-level "location" and inside "venue" (Client.cpp).
				loc := geoLocation(geo)
				out.Location = &loc
				out.Venue.Location = loc
			}
		}
	case *tg.MessageMediaDice:
		out.Dice = &apitypes.Dice{
			Emoji: m.Emoticon,
			Value: int(m.Value),
		}
	case *tg.MessageMediaPoll:
		if m.Poll != nil {
			out.Poll = convertPoll(m.Poll, m.Results)
		}
	case *tg.MessageMediaWebPage:
		// A web-page preview maps to the message's link_preview_options
		// (Client.cpp emits url + force_small/large_media when present).
		if wp, ok := m.Webpage.(*tg.WebPage); ok && wp.URL != "" {
			out.LinkPreviewOptions = &apitypes.LinkPreviewOptions{
				URL:              wp.URL,
				PreferSmallMedia: m.ForceSmallMedia,
				PreferLargeMedia: m.ForceLargeMedia,
			}
		}
	case *tg.MessageMediaGame:
		if m.Game != nil {
			out.Game = convertGame(m.Game)
		}
	case *tg.MessageMediaPaidMedia:
		out.PaidMedia = convertPaidMedia(m.ExtendedMedia, m.StarsAmount)
	}
}

// convertPoll converts a tg.Poll (+ its results) to types.Poll. The results
// carry the quiz explanation (Solution/SolutionEntities) which the poll itself
// does not. correct_option_ids are not recoverable (mtgo's tg.Poll flattens
// pollTypeQuiz to a bool, dropping the IDs); explanation_media defers a media
// conversion.
// geoLocation converts a tg.GeoPoint to a Bot API Location, rounding the
// coordinates to 6 decimal places (~0.1m). The reference (TDLib) stores and
// serializes coordinates at this precision; emitting the raw float64 leaks
// representation noise (e.g. 40.71278880130426 instead of 40.712789).
func geoLocation(geo *tg.GeoPoint) apitypes.Location {
	return apitypes.Location{
		Latitude:           apitypes.ForceFloat(roundCoord(geo.Lat)),
		Longitude:          apitypes.ForceFloat(roundCoord(geo.Long)),
		HorizontalAccuracy: apitypes.ForceFloat(geo.AccuracyRadius),
	}
}

// GeoPointLocation converts an MTProto GeoPoint into a Bot API Location, or nil
// for nil/empty (no location shared). Used for the inline-query `location` field.
func GeoPointLocation(geo tg.GeoPointClass) *apitypes.Location {
	if geo == nil {
		return nil
	}
	g, ok := geo.(*tg.GeoPoint)
	if !ok {
		return nil // GeoPointEmpty carries no coordinates
	}
	loc := geoLocation(g)
	return &loc
}

// InlineQueryChatType maps an MTProto InlineQueryPeerType to the Bot API
// inline-query chat_type string. Mirrors TDLib's conversion
// (InlineQueriesManager.cpp:2455 — SameBotPm→chatTypePrivate(sender),
// BotPm/Pm→chatTypePrivate(0), Chat→BasicGroup, Megagroup→Supergroup(false),
// Broadcast→Supergroup(true)) composed with bot-api's JsonInlineQuery switch
// (Client.cpp:5376: chatTypePrivate(user_id==sender)→"sender", else "private";
// BasicGroup→"group"; Supergroup(!is_channel)→"supergroup"; (is_channel)→"channel").
// Returns "" (omitted) when there is no peer type.
func InlineQueryChatType(pt tg.InlineQueryPeerTypeClass) string {
	switch pt.(type) {
	case *tg.InlineQueryPeerTypeSameBotPm:
		return "sender"
	case *tg.InlineQueryPeerTypeBotPm, *tg.InlineQueryPeerTypePm:
		return "private"
	case *tg.InlineQueryPeerTypeChat:
		return "group"
	case *tg.InlineQueryPeerTypeMegagroup:
		return "supergroup"
	case *tg.InlineQueryPeerTypeBroadcast:
		return "channel"
	}
	return ""
}

// roundCoord rounds a coordinate to 6 decimal places.
func roundCoord(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func convertPoll(p *tg.Poll, results *tg.PollResults) *apitypes.Poll {
	if p == nil {
		return nil
	}
	poll := &apitypes.Poll{
		ID:                    formatInt64(p.ID),
		IsClosed:              p.Closed,
		IsAnonymous:           !p.PublicVoters,
		AllowsMultipleAnswers: p.MultipleChoice,
		// TL revoting_disabled is the inverse of the Bot API allows_revoting.
		AllowsRevoting: !p.RevotingDisabled,
		// TL subscribers_only maps to the Bot API members_only flag.
		MembersOnly:  p.SubscribersOnly,
		CountryCodes: p.CountriesIso2,
		OpenPeriod:   int(p.ClosePeriod),
		CloseDate:    int(p.CloseDate),
	}
	if p.Quiz {
		poll.Type = "quiz"
	} else {
		poll.Type = "regular"
	}
	if p.Question != nil {
		poll.Question = p.Question.Text
		poll.QuestionEntities = convertEntities(p.Question.Entities)
	}
	// Per-option voter counts keyed by the option bytes (= persistent_id), plus
	// the total voter count, both read from poll results (Client.cpp JsonPoll:2745
	// reads total_voter_count; JsonPollOption:2717 reads voter_count per option).
	// correct_option_id(s) are intentionally not populated: the correct answers
	// are absent from a received poll (only the author knows them); the reference
	// fills them only when TDLib actually has the data.
	voters := map[string]int{}
	if results != nil {
		poll.TotalVoterCount = int(results.TotalVoters)
		for _, v := range results.Results {
			if v != nil {
				voters[string(v.Option)] = int(v.Voters)
			}
		}
		poll.Explanation = results.Solution
		poll.ExplanationEntities = convertEntities(results.SolutionEntities)
	}
	// Convert answers. PollAnswer.Text is *TextWithEntities; PollAnswer.Option is []byte.
	poll.Options = make([]apitypes.PollOption, 0, len(p.Answers))
	for _, a := range p.Answers {
		ans, ok := a.(*tg.PollAnswer)
		if !ok {
			continue
		}
		opt := apitypes.PollOption{PersistentID: string(ans.Option), VoterCount: voters[string(ans.Option)]}
		if ans.Text != nil {
			opt.Text = ans.Text.Text
			opt.TextEntities = convertEntities(ans.Text.Entities)
		}
		poll.Options = append(poll.Options, opt)
	}
	return poll
}

// convertEntities maps a slice of MTProto MessageEntityClass to Bot API
// MessageEntity, dropping suppressed types (mirrors the reference entity list).
func convertEntities(in []tg.MessageEntityClass) []apitypes.MessageEntity {
	out := make([]apitypes.MessageEntity, 0, len(in))
	for _, e := range in {
		if ent := messageEntity(e); ent != nil {
			out = append(out, *ent)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// convertGame converts a tg.Game to types.Game.
func convertGame(g *tg.Game) *apitypes.Game {
	if g == nil {
		return nil
	}
	game := &apitypes.Game{
		Title:       g.Title,
		Description: g.Description,
	}
	if g.Photo != nil {
		if photo, ok := g.Photo.(*tg.Photo); ok {
			game.Photo = Photo(photo)
		}
	}
	if g.Document != nil {
		if doc, ok := g.Document.(*tg.Document); ok {
			game.Animation = Animation(doc)
		}
	}
	return game
}

// convertFwdFrom converts a tg.MessageFwdHeader to a Bot API MessageOrigin.
func convertFwdFrom(fwd *tg.MessageFwdHeader) *apitypes.MessageOrigin {
	if fwd == nil {
		return nil
	}
	origin := &apitypes.MessageOrigin{
		Date:            int64(fwd.Date),
		AuthorSignature: fwd.PostAuthor,
	}
	if fwd.FromID != nil {
		switch from := fwd.FromID.(type) {
		case *tg.PeerUser:
			origin.Type = "user"
			origin.SenderUser = &apitypes.User{ID: from.UserID}
		case *tg.PeerChannel:
			// Reference "channel" variant emits "chat" (the channel), not
			// sender_chat, in Bot API channel-id form.
			origin.Type = "channel"
			origin.Chat = &apitypes.Chat{ID: -1000000000000 - from.ChannelID, Type: apitypes.ChatTypeChannel}
			if fwd.ChannelPost != 0 {
				origin.MessageID = int64(fwd.ChannelPost)
			}
		case *tg.PeerChat:
			// "chat" variant emits "sender_chat" in basic-group id form.
			origin.Type = "chat"
			origin.SenderChat = &apitypes.Chat{ID: -from.ChatID, Type: apitypes.ChatTypeGroup}
		}
	} else if fwd.FromName != "" {
		origin.Type = "hidden_user"
		origin.SenderUserName = fwd.FromName
	}
	return origin
}

// convertReplyMarkup converts a tg.ReplyMarkupClass to InlineKeyboardMarkup.
func convertReplyMarkup(rm tg.ReplyMarkupClass) *apitypes.InlineKeyboardMarkup {
	if rm == nil {
		return nil
	}
	switch markup := rm.(type) {
	case *tg.ReplyInlineMarkup:
		rows := make([][]apitypes.InlineKeyboardButton, 0, len(markup.Rows))
		for _, row := range markup.Rows {
			buttons := make([]apitypes.InlineKeyboardButton, 0, len(row.Buttons))
			for _, btn := range row.Buttons {
				b := apitypes.InlineKeyboardButton{}
				switch btn := btn.(type) {
				case *tg.KeyboardButton:
					b.Text = btn.Text
				case *tg.KeyboardButtonURL:
					b.Text = btn.Text
					b.URL = btn.URL
				case *tg.KeyboardButtonCallback:
					b.Text = btn.Text
					b.CallbackData = string(btn.Data)
				case *tg.KeyboardButtonSwitchInline:
					b.Text = btn.Text
					b.SwitchInlineQuery = btn.Query
				case *tg.KeyboardButtonBuy:
					b.Text = btn.Text
					b.Pay = true
				case *tg.KeyboardButtonWebView:
					b.Text = btn.Text
					b.WebApp = &apitypes.WebAppInfo{URL: btn.URL}
				default:
					continue
				}
				buttons = append(buttons, b)
			}
			rows = append(rows, buttons)
		}
		return &apitypes.InlineKeyboardMarkup{InlineKeyboard: rows}
	default:
		return nil
	}
}

// formatInt64 converts an int64 to a decimal string.
func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}

// MessageService converts a service message (*tg.MessageService) into a Bot API
// types.Message, mapping the MessageAction to the Bot API service-message fields
// (new_chat_members, left_chat_member, new_chat_title, etc.). Mirrors the
// reference Client.cpp JsonMessageAction. users resolves action user IDs to full
// User objects (the action only carries IDs); entries missing from the map fall
// back to ID-only User stubs.
func MessageService(svc *tg.MessageService, users map[int64]*tg.User) *apitypes.Message {
	if svc == nil {
		return nil
	}
	out := &apitypes.Message{
		MessageID: int64(svc.ID),
		Date:      int64(svc.Date),
	}
	out.Chat = peerToChat(svc.PeerID)
	if svc.FromID != nil {
		switch from := svc.FromID.(type) {
		case *tg.PeerUser:
			out.From = serviceUser(from.UserID, users)
			if svc.Out {
				out.From.IsBot = true
			}
		case *tg.PeerChat:
			out.SenderChat = &apitypes.Chat{ID: from.ChatID}
		case *tg.PeerChannel:
			out.SenderChat = &apitypes.Chat{ID: from.ChannelID}
		}
	}
	if svc.Action != nil {
		mapMessageAction(out, svc.Action, users)
	}
	// Pin: the pinned message is referenced by the service message's reply_to. The
	// reference (Client.cpp:5001) returns the full pinned message if it has it
	// cached, else an inaccessible-message stub {message_id, date:0, chat}. This
	// converter emits the inaccessible stub carrying the pinned message id; the
	// client layer (resolvePinnedFromCache in updates.go) replaces it with the
	// full message when the bot has previously seen it (per-bot msgCache).
	if _, ok := svc.Action.(*tg.MessageActionPinMessage); ok {
		if rid := replyToMsgID(svc.ReplyTo); rid > 0 {
			out.PinnedMessage = &apitypes.Message{MessageID: int64(rid), Date: 0, Chat: out.Chat}
		}
	}
	return out
}

// replyToMsgID extracts the replied-to message id from a MessageReplyHeader.
func replyToMsgID(replyTo tg.MessageReplyHeaderClass) int32 {
	if h, ok := replyTo.(*tg.MessageReplyHeader); ok {
		return h.ReplyToMsgID
	}
	return 0
}

// mapMessageAction maps a tg.MessageActionClass onto the Bot API Message service
// fields. Only the common, self-contained actions are handled; the rest (gifts,
// payments, secure values, suggested posts, etc.) have no Bot API service field
// and are intentionally ignored, matching the reference's JsonMessageAction.
func mapMessageAction(out *apitypes.Message, action tg.MessageActionClass, users map[int64]*tg.User) {
	switch a := action.(type) {
	case *tg.MessageActionChatAddUser:
		for _, id := range a.Users {
			out.NewChatMembers = append(out.NewChatMembers, *serviceUser(id, users))
		}
	case *tg.MessageActionChatJoinedByLink, *tg.MessageActionChatJoinedByRequest:
		// The joining user is the message sender.
		if out.From != nil {
			out.NewChatMembers = []apitypes.User{*out.From}
		}
	case *tg.MessageActionChatDeleteUser:
		out.LeftChatMember = serviceUser(a.UserID, users)
	case *tg.MessageActionChatEditTitle:
		out.NewChatTitle = a.Title
	case *tg.MessageActionChatEditPhoto:
		if photo, ok := a.Photo.(*tg.Photo); ok {
			out.NewChatPhoto = Photo(photo)
		}
	case *tg.MessageActionChatDeletePhoto:
		out.DeleteChatPhoto = true
	case *tg.MessageActionChatCreate:
		out.GroupChatCreated = true
	case *tg.MessageActionChannelCreate:
		out.ChannelChatCreated = true
	case *tg.MessageActionChatMigrateTo:
		// Basic group → supergroup: the message in the old chat points to the new
		// supergroup. Bot API channel id = -1e13 - channelId (mirrors
		// get_supergroup_chat_id / peerToChat's PeerChannel arm).
		out.MigrateToChatID = -1000000000000 - a.ChannelID
	case *tg.MessageActionChannelMigrateFrom:
		// First message in the new supergroup, pointing back to the old basic
		// group. Bot API basic-group id = -chatId (mirrors get_basic_group_chat_id).
		out.MigrateFromChatID = -a.ChatID
	case *tg.MessageActionSetMessagesTTL:
		out.MessageAutoDeleteTimerChanged = &apitypes.MessageAutoDeleteTimerChanged{
			MessageAutoDeleteTime: int(a.Period),
		}
	case *tg.MessageActionGroupCall:
		// Bot API emits both video_chat_started and the voice_chat_started alias.
		started := &apitypes.VideoChatStarted{}
		out.VoiceChatStarted = started
		out.VideoChatStarted = started
	case *tg.MessageActionGroupCallScheduled:
		scheduled := &apitypes.VideoChatScheduled{StartDate: int64(a.ScheduleDate)}
		out.VoiceChatScheduled = scheduled
		out.VideoChatScheduled = scheduled
	case *tg.MessageActionInviteToGroupCall:
		// Users invited to a video chat → both video/voice alias fields.
		invited := make([]apitypes.User, 0, len(a.Users))
		for _, id := range a.Users {
			invited = append(invited, *serviceUser(id, users))
		}
		v := &apitypes.VideoChatParticipantsInvited{Users: invited}
		out.VoiceChatParticipantsInvited = v
		out.VideoChatParticipantsInvited = v
	}
}

// serviceUser resolves a user ID to a Bot API User via the update's user map,
// falling back to an ID-only stub when the full user is unavailable.
func serviceUser(id int64, users map[int64]*tg.User) *apitypes.User {
	if u, ok := users[id]; ok && u != nil {
		return User(u)
	}
	return &apitypes.User{ID: id}
}
