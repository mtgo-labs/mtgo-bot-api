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
	Register("sendcontact", (*Client).sendContact)
	Register("sendvenue", (*Client).sendVenue)
	Register("sendlocation", (*Client).sendLocation)
	Register("senddice", (*Client).sendDice)
	Register("sendpoll", (*Client).sendPoll)
	Register("stoppoll", (*Client).stopPoll)
	Register("sendpaidmedia", (*Client).sendPaidMedia)
	Register("setmessagereaction", (*Client).setMessageReaction)
}

func (c *Client) sendContact(ctx context.Context, q *server.Query) (any, error) {
	phoneNumber := q.Arg("phone_number")
	if phoneNumber == "" {
		return nil, NewError(400, `Bad Request: parameter "phone_number" is required`)
	}
	firstName := q.Arg("first_name")
	if firstName == "" {
		return nil, NewError(400, `Bad Request: parameter "first_name" is required`)
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	media := &tg.InputMediaContact{
		PhoneNumber: phoneNumber,
		FirstName:   firstName,
		LastName:    q.Arg("last_name"),
		Vcard:       q.Arg("vcard"),
	}
	return c.sendMediaMessage(ctx, peer, media, q)
}

// sendVenue implements the Bot API sendVenue method.
func (c *Client) sendVenue(ctx context.Context, q *server.Query) (any, error) {
	// Coordinates are validated per-field, mirroring the reference get_location
	// helper (Client.cpp:12003/12007). The reference process_send_venue_query
	// (Client.cpp:13777-13780) does NOT validate title/address — they may be
	// empty — so we must not reject them here.
	latStr := q.Arg("latitude")
	if latStr == "" {
		return nil, NewError(400, "Bad Request: latitude is empty")
	}
	lonStr := q.Arg("longitude")
	if lonStr == "" {
		return nil, NewError(400, "Bad Request: longitude is empty")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	title := q.Arg("title")
	address := q.Arg("address")
	// Venue provider/id/type: google_place → provider "gplaces", foursquare →
	// "foursquare", with foursquare taking precedence. Mirrors the reference
	// process_send_venue_query (Client.cpp:13782-13795) which collapses the four
	// Bot API params onto inputMediaVenue's provider/venue_id/venue_type.
	var provider, venueID, venueType string
	if gpID := q.Arg("google_place_id"); gpID != "" || q.Arg("google_place_type") != "" {
		provider, venueID, venueType = "gplaces", gpID, q.Arg("google_place_type")
	}
	if fsID := q.Arg("foursquare_id"); fsID != "" || q.Arg("foursquare_type") != "" {
		provider, venueID, venueType = "foursquare", fsID, q.Arg("foursquare_type")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	lat, _ := strconv.ParseFloat(latStr, 64)
	lon, _ := strconv.ParseFloat(lonStr, 64)
	media := &tg.InputMediaVenue{
		GeoPoint:  &tg.InputGeoPoint{Lat: lat, Long: lon},
		Title:     title,
		Address:   address,
		Provider:  provider,
		VenueID:   venueID,
		VenueType: venueType,
	}
	return c.sendMediaMessage(ctx, peer, media, q)
}

// sendLocation implements the Bot API sendLocation method.
func (c *Client) sendLocation(ctx context.Context, q *server.Query) (any, error) {
	// Coordinate validation mirrors the reference get_location helper
	// (Client.cpp:12003/12007), which checks each field separately.
	latStr := q.Arg("latitude")
	if latStr == "" {
		return nil, NewError(400, "Bad Request: latitude is empty")
	}
	lonStr := q.Arg("longitude")
	if lonStr == "" {
		return nil, NewError(400, "Bad Request: longitude is empty")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
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
	media := tg.InputMediaClass(&tg.InputMediaGeoPoint{GeoPoint: geo})
	// Live location: when live_period is set, send inputMediaGeoLive (heading +
	// proximity_alert_radius apply only on the live path). Reference: Client.cpp:13759-13766.
	if lp, err := q.ArgInt64("live_period"); err == nil && lp > 0 {
		live := &tg.InputMediaGeoLive{GeoPoint: geo, Period: int32(lp)}
		if h, err := q.ArgInt64("heading"); err == nil && h != 0 {
			live.Heading = int32(h)
		}
		if par, err := q.ArgInt64("proximity_alert_radius"); err == nil && par != 0 {
			live.ProximityNotificationRadius = int32(par)
		}
		live.SetFlags()
		media = live
	}
	return c.sendMediaMessage(ctx, peer, media, q)
}

// sendDice implements the Bot API sendDice method.
func (c *Client) sendDice(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	emoticon := q.Arg("emoji")
	if emoticon == "" {
		emoticon = "🎲"
	}
	media := &tg.InputMediaDice{Emoticon: emoticon}
	return c.sendMediaMessage(ctx, peer, media, q)
}

// sendPoll implements the Bot API sendPoll method.
func (c *Client) sendPoll(ctx context.Context, q *server.Query) (any, error) {
	// Options validated before question — matches official error ordering
	// ("can't parse options JSON object" fires first when params are missing).
	optionTexts, err := parsePollOptions(q.Arg("options"))
	if err != nil {
		return nil, err
	}

	// Question (with question_parse_mode / question_entities). Mirrors the
	// reference's get_formatted_text("question", "question_parse_mode", …).
	questionText, questionEntities, err := convert.FormattedText(q.Arg("question"), q.Arg("question_parse_mode"), q.Arg("question_entities"))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if questionText == "" {
		return nil, NewError(400, "Bad Request: parameter \"question\" is required")
	}

	answers := make([]tg.PollAnswerClass, 0, len(optionTexts))
	for _, opt := range optionTexts {
		// When CREATING a poll (inputMediaPoll), Poll.answers must use
		// inputPollAnswer#199fed96 (the INPUT variant, no option bytes) — the
		// server assigns option identifiers. Using pollAnswer#4b7d786a (the
		// OUTPUT variant, with option:bytes) is rejected with POLL_OPTION_INVALID.
		a := &tg.InputPollAnswer{Text: &tg.TextWithEntities{Text: opt}}
		a.SetFlags()
		answers = append(answers, a)
	}

	pollType := q.Arg("type")
	if pollType != "" && pollType != "regular" && pollType != "quiz" {
		return nil, NewError(400, "Bad Request: Unsupported poll type specified")
	}
	isQuiz := pollType == "quiz"

	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	poll := &tg.Poll{
		Question: &tg.TextWithEntities{Text: questionText, Entities: questionEntities},
		Answers:  answers,
	}
	// is_anonymous defaults to true (anonymous); public_voters means NOT anonymous,
	// so only set it when is_anonymous is explicitly false.
	if q.HasArg("is_anonymous") && !q.ArgBool("is_anonymous") {
		poll.PublicVoters = true
	}
	if q.ArgBool("allows_multiple_answers") {
		poll.MultipleChoice = true
	}
	// allow_adding_options → poll open_answers (flag 6); regular polls only
	// (Client.cpp:13862 inputPollTypeRegular(allow_adding_options) → TDLib
	// has_open_answers → MessageContent.cpp:5016).
	if !isQuiz && q.ArgBool("allow_adding_options") {
		poll.OpenAnswers = true
	}
	if isQuiz {
		poll.Quiz = true
	}
	// allows_revoting defaults to true for regular, false for quiz; overridden by arg.
	revoting := !isQuiz
	if q.HasArg("allows_revoting") {
		revoting = q.ArgBool("allows_revoting")
	}
	if !revoting {
		poll.RevotingDisabled = true
	}
	if q.ArgBool("is_closed") {
		poll.Closed = true
	}
	if q.ArgBool("shuffle_options") {
		poll.ShuffleAnswers = true
	}
	if q.ArgBool("hide_results_until_closes") {
		poll.HideResultsUntilClose = true
	}
	if q.ArgBool("members_only") {
		poll.SubscribersOnly = true
	}
	if op := q.Arg("open_period"); op != "" {
		if v, e := strconv.ParseInt(op, 10, 32); e == nil {
			poll.ClosePeriod = int32(v)
		}
	}
	if cd := q.Arg("close_date"); cd != "" {
		if v, e := strconv.ParseInt(cd, 10, 32); e == nil {
			poll.CloseDate = int32(v)
		}
	}
	if cc := q.Arg("country_codes"); cc != "" {
		var codes []string
		if e := json.Unmarshal([]byte(cc), &codes); e == nil {
			poll.CountriesIso2 = codes
		}
	}
	poll.SetFlags()

	media := &tg.InputMediaPoll{Poll: poll}

	// Quiz: correct option(s) + explanation. description / description_parse_mode
	// have no field on the Poll/InputMediaPoll structs in this tg layer (blocked).
	if isQuiz {
		if q.HasArg("correct_option_ids") {
			var ids []int32
			if e := json.Unmarshal([]byte(q.Arg("correct_option_ids")), &ids); e == nil {
				media.CorrectAnswers = ids
			}
		} else if q.HasArg("correct_option_id") {
			if v, e := strconv.ParseInt(q.Arg("correct_option_id"), 10, 32); e == nil {
				media.CorrectAnswers = []int32{int32(v)}
			}
		}
		if expl := q.Arg("explanation"); expl != "" {
			text, entities, e := convert.FormattedText(expl, q.Arg("explanation_parse_mode"), q.Arg("explanation_entities"))
			if e != nil {
				return nil, NewError(400, "Bad Request: "+e.Error())
			}
			media.Solution = text
			media.SolutionEntities = entities
		}
		// explanation_media → TL inputMediaPoll solution_media (flag 2).
		// Traced via TDLib: inputPollTypeQuiz.explanation_media → get_input_poll_media → solution_media.
		if explMedia := q.Arg("explanation_media"); explMedia != "" {
			m, _, e := c.parseMediaDescriptor(ctx, q, explMedia)
			if e != nil {
				return nil, NewError(400, "Bad Request: "+e.Error())
			}
			media.SolutionMedia = m
		}
	}
	media.SetFlags()

	return c.sendMediaMessage(ctx, peer, media, q)
}

// parsePollOptions decodes the Bot API "options" param: an array of strings or an
// array of {"text": …} objects. Mirrors get_input_poll_options' JSON shape.
func parsePollOptions(raw string) ([]string, error) {
	var optionTexts []string
	if err := json.Unmarshal([]byte(raw), &optionTexts); err != nil {
		var optionObjs []struct {
			Text string `json:"text"`
		}
		if err2 := json.Unmarshal([]byte(raw), &optionObjs); err2 != nil {
			return nil, NewError(400, "Bad Request: can't parse options JSON object")
		}
		for _, o := range optionObjs {
			optionTexts = append(optionTexts, o.Text)
		}
	}
	return optionTexts, nil
}

// stopPoll implements the Bot API stopPoll method.
// Uses messages.editMessage with InputMediaPoll containing a closed Poll.
// First fetches the message to extract the poll ID, then edits with Closed flag.
func (c *Client) stopPoll(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	messageID := q.Arg("message_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	msgID, err := strconv.ParseInt(messageID, 10, 32)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid message_id")
	}

	// Fetch the message to get the poll ID.
	msgs, err := c.rpc.MessagesGetMessages(ctx, &tg.MessagesGetMessagesRequest{
		ID: []tg.InputMessageClass{&tg.InputMessageID{ID: int32(msgID)}},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	// Extract poll ID from the message.
	var pollID int64
	var found bool
	switch result := msgs.(type) {
	case *tg.MessagesMessages:
		for _, m := range result.Messages {
			if msg, ok := m.(*tg.Message); ok {
				if poll, ok := msg.Media.(*tg.MessageMediaPoll); ok && poll.Poll != nil {
					pollID = poll.Poll.ID
					found = true
				}
			}
		}
	case *tg.MessagesMessagesSlice:
		for _, m := range result.Messages {
			if msg, ok := m.(*tg.Message); ok {
				if poll, ok := msg.Media.(*tg.MessageMediaPoll); ok && poll.Poll != nil {
					pollID = poll.Poll.ID
					found = true
				}
			}
		}
	case *tg.MessagesChannelMessages:
		for _, m := range result.Messages {
			if msg, ok := m.(*tg.Message); ok {
				if poll, ok := msg.Media.(*tg.MessageMediaPoll); ok && poll.Poll != nil {
					pollID = poll.Poll.ID
					found = true
				}
			}
		}
	}
	if !found {
		return nil, NewError(400, "Bad Request: message does not contain a poll")
	}

	// Construct InputMediaPoll with a closed Poll.
	closedPoll := &tg.Poll{
		ID:       pollID,
		Closed:   true,
		Question: &tg.TextWithEntities{Text: ""}, // required field
	}
	closedPoll.SetFlags()
	media := &tg.InputMediaPoll{Poll: closedPoll}

	req := &tg.MessagesEditMessageRequest{
		Peer:  peer,
		ID:    int32(msgID),
		Media: media,
	}
	req.Flags.Set(1) // media flag
	req.SetFlags()
	var result tg.UpdatesClass
	if connID := businessConnID(q); connID != "" {
		obj, err := c.invokeBusiness(ctx, connID, req)
		if err != nil {
			return nil, rpcError(err)
		}
		upd, ok := obj.(tg.UpdatesClass)
		if !ok {
			return nil, NewError(500, "Internal Server Error: unexpected stop-poll result")
		}
		result = upd
	} else {
		var err error
		result, err = c.rpc.MessagesEditMessage(ctx, req)
		if err != nil {
			return nil, rpcError(err)
		}
	}
	// Extract the updated poll from the response.
	if u, ok := result.(*tg.Updates); ok {
		for _, upd := range u.Updates {
			if edit, ok := upd.(*tg.UpdateEditMessage); ok {
				if msg, ok := edit.Message.(*tg.Message); ok {
					if poll, ok := msg.Media.(*tg.MessageMediaPoll); ok && poll.Poll != nil {
						return map[string]any{
							"id":                poll.Poll.ID,
							"is_closed":         poll.Poll.Closed,
							"question":          poll.Poll.Question.Text,
							"total_voter_count": 0,
						}, nil
					}
				}
			}
		}
	}
	return true, nil
}

// sendPaidMedia implements the Bot API sendPaidMedia method.
// Uses messages.sendMedia with InputMediaPaidMedia.
func (c *Client) sendPaidMedia(ctx context.Context, q *server.Query) (any, error) {
	// Official validates media content before chat_id.
	if q.Arg("media") == "" {
		return nil, NewError(400, `Bad Request: parameter "media" is required`)
	}
	starCount, err := strconv.ParseInt(q.Arg("star_count"), 10, 64)
	if err != nil || starCount <= 0 {
		return nil, NewError(400, "Bad Request: star_count must be positive")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	// Build extended media from the media JSON array (attach:// upload, URL, or file_id).
	mediaList, err := c.resolvePaidMediaArray(ctx, q, q.Arg("media"))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	if len(mediaList) == 0 {
		return nil, NewError(400, "Bad Request: parameter \"media\" is required")
	}
	paid := &tg.InputMediaPaidMedia{
		StarsAmount:   starCount,
		ExtendedMedia: mediaList,
	}
	if payload := q.Arg("payload"); payload != "" {
		paid.Payload = payload
	}
	paid.SetFlags()
	req := &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    paid,
		RandomID: randomID(),
	}
	req.InvertMedia = q.ArgBool("show_caption_above_media")
	req.SetFlags()
	result, err := c.rpc.MessagesSendMedia(ctx, req)
	if err != nil {
		return nil, c.sendRPCError(ctx, err, q)
	}
	msg := extractMessageFromUpdates(result)
	if msg == nil {
		return true, nil
	}
	c.cachePeersFromMessage(ctx, msg)
	return c.botMessage(ctx, msg, extractChats(result)), nil
}

// setMessageReaction implements the Bot API setMessageReaction method.
func (c *Client) setMessageReaction(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	messageID := q.Arg("message_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	msgID, err := strconv.ParseInt(messageID, 10, 32)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid message_id")
	}
	// Build reaction list from JSON.
	reactionsJSON := q.Arg("reaction")
	var reactionClasses []tg.ReactionClass
	if reactionsJSON != "" && reactionsJSON != "[]" {
		var reactions []struct {
			Type          string `json:"type"`
			Emoji         string `json:"emoji,omitempty"`
			CustomEmojiID string `json:"custom_emoji_id,omitempty"`
		}
		if err := json.Unmarshal([]byte(reactionsJSON), &reactions); err == nil {
			for _, r := range reactions {
				switch r.Type {
				case "emoji":
					reactionClasses = append(reactionClasses, &tg.ReactionEmoji{Emoticon: r.Emoji})
				case "custom_emoji":
					id, _ := strconv.ParseInt(r.CustomEmojiID, 10, 64)
					reactionClasses = append(reactionClasses, &tg.ReactionCustomEmoji{DocumentID: id})
				}
			}
		}
	}
	_, err = c.rpc.MessagesSendReaction(ctx, &tg.MessagesSendReactionRequest{
		Peer:     peer,
		MsgID:    int32(msgID),
		Reaction: reactionClasses,
		Big:      q.ArgBool("is_big"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// sendMediaMessage is a helper that sends a message with InputMedia via messages.sendMedia.
func (c *Client) sendMediaMessage(ctx context.Context, peer tg.InputPeerClass, media tg.InputMediaClass, q *server.Query) (any, error) {
	if err := c.checkWriteAccess(ctx, q); err != nil {
		return nil, err
	}
	req := &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		RandomID: randomID(),
	}
	if rt := buildReplyTo(q); rt != nil {
		req.ReplyTo = rt
		req.Flags.Set(0)
	}
	// Caption. sendInvoice carries it as paid_media_caption; other media-send
	// callers (sendContact/Venue/Location/Dice/Poll) don't pass a caption here.
	caption := q.Arg("caption")
	parseMode := q.Arg("caption_parse_mode")
	entitiesJSON := q.Arg("caption_entities")
	if caption == "" {
		caption = q.Arg("paid_media_caption")
		parseMode = q.Arg("paid_media_caption_parse_mode")
		entitiesJSON = q.Arg("paid_media_caption_entities")
	}
	if caption != "" {
		parsed, ents, err := convert.FormattedText(caption, parseMode, entitiesJSON)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		req.Message = parsed
		if len(ents) > 0 {
			req.Entities = ents
		}
	}
	if q.ArgBool("disable_notification") {
		req.Silent = true
		req.Flags.Set(5)
	}
	if q.ArgBool("protect_content") {
		req.Noforwards = true
		req.Flags.Set(14)
	}
	if q.ArgBool("show_caption_above_media") {
		req.InvertMedia = true
		req.Flags.Set(16)
	}
	if eid, err := q.ArgInt64("message_effect_id"); err == nil && eid != 0 {
		req.Effect = eid
	}
	req.SetFlags()
	result, err := c.rpc.MessagesSendMedia(ctx, req)
	if err != nil {
		return nil, c.sendRPCError(ctx, err, q)
	}
	msg := extractMessageFromUpdates(result)
	if msg == nil {
		return nil, NewError(500, "Internal Server Error: failed to extract message")
	}
	// Enrich partial messages from UpdateShortSentMessage.
	c.enrichPartialMessage(msg, peer, "")
	c.cachePeersFromMessage(ctx, msg)
	return c.botMessage(ctx, msg, extractChats(result)), nil
}
