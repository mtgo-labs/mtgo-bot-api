package client

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/convert"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func init() {
	Register("poststory", (*Client).postStory)
	Register("editstory", (*Client).editStory)
	Register("deletestory", (*Client).deleteStory)
	Register("repoststory", (*Client).repostStory)
	Register("approvesuggestedpost", (*Client).approveSuggestedPost)
	Register("declinesuggestedpost", (*Client).declineSuggestedPost)
	Register("sendmessagedraft", (*Client).sendMessageDraft)
	Register("sendrichmessagedraft", (*Client).sendRichMessageDraft)
	Register("sendrichmessage", (*Client).sendRichMessage)
	Register("sendchecklist", (*Client).sendChecklist)
	Register("editmessagechecklist", (*Client).editMessageChecklist)
	Register("deletemessagereaction", (*Client).deleteMessageReaction)
	Register("deleteallmessagereactions", (*Client).deleteAllMessageReactions)
	Register("sendlivephoto", (*Client).sendLivePhoto)
}

// postStory implements the Bot API postStory method.
//
// Stories carry their media as a JSON `content` object referencing a multipart-
// uploaded file (attach://…); we upload it and invoke stories.sendStory with the
// resolved peer. The result is the lightweight Bot API Story {chat, id} — the
// reference's JsonStory emits only those two fields, so apitypes.Story matches.
//
// The reference (Client.cpp process_post_story_query) resolves the peer from the
// business connection's user_chat_id. We honor business_connection_id when present
// (via resolveStoryPeer) and fall back to chat_id otherwise.
func (c *Client) postStory(ctx context.Context, q *server.Query) (any, error) {
	if q.Arg("content") == "" {
		return nil, NewError(400, "Bad Request: story content isn't specified")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := c.resolveStoryPeer(ctx, q)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	media, err := c.storyContentMedia(ctx, q)
	if err != nil {
		return nil, err
	}
	caption, entities, err := convert.FormattedText(q.Arg("caption"), q.Arg("parse_mode"), q.Arg("caption_entities"))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	areas, err := convert.StoryAreas(q.Arg("areas"))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	req := &tg.StoriesSendStoryRequest{
		Peer:         peer,
		Media:        media,
		MediaAreas:   areas,
		Caption:      caption,
		Entities:     entities,
		PrivacyRules: []tg.InputPrivacyRuleClass{&tg.InputPrivacyValueAllowAll{}},
		RandomID:     randomID(),
	}
	if ap := q.Arg("active_period"); ap != "" {
		if v, perr := strconv.ParseInt(ap, 10, 32); perr == nil {
			req.Period = int32(v)
		}
	}
	if q.ArgBool("protect_content") {
		req.Noforwards = true
	}
	// post_to_chat_page → the MTProto `pinned` flag (TL stories.sendStory flag 2).
	// Traced via TDLib: postStory is_posted_to_chat_page → StoryManager story.is_pinned_
	// → stories_sendStory pinned. (toggleStoryIsPostedToChatPage == toggle_story_is_pinned.)
	if q.ArgBool("post_to_chat_page") {
		req.Pinned = true
	}
	req.SetFlags()
	result, err := c.rpc.StoriesSendStory(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return storyResult(result, peer)
}

// editStory implements the Bot API editStory method via stories.editStory.
// See postStory for the content/peer model and the business-connection note.
func (c *Client) editStory(ctx context.Context, q *server.Query) (any, error) {
	storyID := q.Arg("story_id")
	if q.Arg("content") == "" {
		return nil, NewError(400, "Bad Request: story content isn't specified")
	}
	if storyID == "" {
		return nil, NewError(400, "Bad Request: parameter \"story_id\" is required")
	}
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := c.resolveStoryPeer(ctx, q)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	sid, err := strconv.ParseInt(storyID, 10, 32)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid story_id")
	}
	media, err := c.storyContentMedia(ctx, q)
	if err != nil {
		return nil, err
	}
	caption, entities, err := convert.FormattedText(q.Arg("caption"), q.Arg("parse_mode"), q.Arg("caption_entities"))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	areas, err := convert.StoryAreas(q.Arg("areas"))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	req := &tg.StoriesEditStoryRequest{
		Peer:       peer,
		ID:         int32(sid),
		Media:      media,
		MediaAreas: areas,
		Caption:    caption,
		Entities:   entities,
	}
	req.SetFlags()
	result, err := c.rpc.StoriesEditStory(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return storyResult(result, peer)
}

// resolveStoryPeer resolves the target peer for a story method. When
// business_connection_id is present, the peer is the business user (matching the
// reference's check_business_connection → user_chat_id) — fetched via
// account.getBotBusinessConnection then resolved with the cached access hash or the
// hash=0 fallback for known users. Otherwise it falls back to chat_id.
func (c *Client) resolveStoryPeer(ctx context.Context, q *server.Query) (tg.InputPeerClass, error) {
	if connID := businessConnID(q); connID != "" {
		res, err := c.rpc.AccountGetBotBusinessConnection(ctx, &tg.AccountGetBotBusinessConnectionRequest{
			ConnectionID: connID,
		})
		if err != nil {
			return nil, rpcError(err)
		}
		if u, ok := res.(*tg.Updates); ok {
			for _, upd := range u.Updates {
				if bc, ok := upd.(*tg.UpdateBotBusinessConnect); ok && bc.Connection != nil {
					return convert.ResolvePeer(ctx, strconv.FormatInt(bc.Connection.UserID, 10), c.store)
				}
			}
		}
		return nil, NewError(400, "Bad Request: business connection not found")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	return convert.ResolvePeer(ctx, chatID, c.store)
}

// storyContentMedia parses the Bot API `content` JSON param (a StoryContent object:
// {"type":"photo","photo":"attach://<file>"} or {"type":"video",…}) and uploads the
// referenced multipart file, returning the InputMedia for stories.sendStory/editStory.
// Mirrors the reference get_input_story_content (Client.cpp:12939).
func (c *Client) storyContentMedia(ctx context.Context, q *server.Query) (tg.InputMediaClass, error) {
	raw := q.Arg("content")
	if raw == "" {
		return nil, NewError(400, "Bad Request: story content isn't specified")
	}
	var content struct {
		Type        string  `json:"type"`
		Photo       string  `json:"photo"`
		Video       string  `json:"video"`
		Duration    float64 `json:"duration"`
		IsAnimation bool    `json:"is_animation"`
	}
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return nil, NewError(400, "Bad Request: can't parse story content JSON object")
	}
	switch content.Type {
	case "photo":
		inputFile, err := c.storyUpload(ctx, q, content.Photo, "photo")
		if err != nil {
			return nil, err
		}
		media := &tg.InputMediaUploadedPhoto{File: inputFile}
		media.SetFlags()
		return media, nil
	case "video":
		inputFile, err := c.storyUpload(ctx, q, content.Video, "video")
		if err != nil {
			return nil, err
		}
		doc := &tg.InputMediaUploadedDocument{
			File:     inputFile,
			MimeType: "video/mp4",
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeVideo{Duration: content.Duration},
			},
		}
		if content.IsAnimation {
			doc.NosoundVideo = true
		}
		doc.SetFlags()
		return doc, nil
	default:
		return nil, NewError(400, "Bad Request: invalid story content type specified")
	}
}

// storyUpload resolves an "attach://<name>" reference to its multipart file and
// uploads it, returning the InputFile. Stories require an uploaded file (not a
// file_id), matching the reference get_input_file(..., /*attach=*/true).
func (c *Client) storyUpload(ctx context.Context, q *server.Query, ref, kind string) (tg.InputFileClass, error) {
	const prefix = "attach://"
	if !strings.HasPrefix(ref, prefix) {
		return nil, NewError(400, "Bad Request: Story "+kind+" must be uploaded as a file")
	}
	file, ok := q.File(ref[len(prefix):])
	if !ok {
		return nil, NewError(400, "Bad Request: Story "+kind+" must be uploaded as a file")
	}
	f, err := os.Open(file.TempPath)
	if err != nil {
		return nil, NewError(400, "Bad Request: failed to read "+kind)
	}
	defer func() { _ = f.Close() }()
	fileID, _ := generateFileID()
	return c.uploadFile(ctx, fileID, file.FileName, file.Size, f)
}

// storyResult extracts the new/edited story's {chat, id} from the stories.sendStory/
// editStory Updates response (an updateStory carrying a *tg.StoryItem). reqPeer is the
// request peer, used as the chat fallback. Mirrors the reference JsonStory {chat, id}.
func storyResult(result tg.UpdatesClass, reqPeer tg.InputPeerClass) (*apitypes.Story, error) {
	id, chatPeer := extractStoryFromUpdates(result)
	if id == 0 {
		return nil, NewError(500, "Internal Server Error: failed to extract story from response")
	}
	return &apitypes.Story{Chat: storyChat(chatPeer, reqPeer), ID: int64(id)}, nil
}

// extractStoryFromUpdates pulls the story id + peer from an updateStory in the
// Updates response returned by stories.sendStory/editStory.
func extractStoryFromUpdates(result tg.UpdatesClass) (int32, tg.PeerClass) {
	if result == nil {
		return 0, nil
	}
	if u, ok := result.(*tg.Updates); ok {
		for _, upd := range u.Updates {
			if us, ok := upd.(*tg.UpdateStory); ok {
				if item, ok := us.Story.(*tg.StoryItem); ok {
					return item.ID, us.Peer
				}
			}
		}
	}
	return 0, nil
}

// storyChat derives the Bot API Chat {id, type} for the result Story, preferring
// the update's peer and falling back to the request peer.
func storyChat(chatPeer tg.PeerClass, reqPeer tg.InputPeerClass) apitypes.Chat {
	switch p := chatPeer.(type) {
	case *tg.PeerUser:
		return apitypes.Chat{ID: p.UserID, Type: apitypes.ChatTypePrivate}
	case *tg.PeerChannel:
		return apitypes.Chat{ID: -1000000000000 - p.ChannelID, Type: apitypes.ChatTypeSupergroup}
	case *tg.PeerChat:
		return apitypes.Chat{ID: -p.ChatID, Type: apitypes.ChatTypeGroup}
	}
	switch p := reqPeer.(type) {
	case *tg.InputPeerUser:
		return apitypes.Chat{ID: p.UserID, Type: apitypes.ChatTypePrivate}
	case *tg.InputPeerChannel:
		return apitypes.Chat{ID: -1000000000000 - p.ChannelID, Type: apitypes.ChatTypeSupergroup}
	case *tg.InputPeerChat:
		return apitypes.Chat{ID: -p.ChatID, Type: apitypes.ChatTypeGroup}
	}
	return apitypes.Chat{}
}

// deleteStory implements the Bot API deleteStory method.
// Uses stories.deleteStories at the MTProto level.
func (c *Client) deleteStory(ctx context.Context, q *server.Query) (any, error) {
	connID, err := requireBusinessConn(ctx, c, q)
	if err != nil {
		return nil, err
	}
	chatID := q.Arg("chat_id")
	storyID := q.Arg("story_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	sid, err := strconv.ParseInt(storyID, 10, 32)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid story_id")
	}
	req := &tg.StoriesDeleteStoriesRequest{
		Peer: peer,
		ID:   []int32{int32(sid)},
	}
	if _, err := c.invokeBusiness(ctx, connID, req); err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// repostStory implements the Bot API repostStory method.
// Uses stories.sendStory with fwd_from_id and fwd_from_story flags.
// No media upload needed — content/areas/caption are null for reposts.
func (c *Client) repostStory(ctx context.Context, q *server.Query) (any, error) {
	fromChatID := q.Arg("from_chat_id")
	if fromChatID == "" {
		return nil, NewError(400, `Bad Request: parameter "from_chat_id" is required`)
	}
	storyIDStr := q.Arg("story_id")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	// Target peer: repost to the business user's story when business_connection_id
	// is present, else chat_id. Mirrors the reference process_repost_story_query
	// (Client.cpp:14616-14632: chat_id = business_connection->user_chat_id_).
	peer, err := c.resolveStoryPeer(ctx, q)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	fromPeer, err := convert.ResolvePeer(ctx, fromChatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	storyID, err := strconv.ParseInt(storyIDStr, 10, 32)
	if err != nil {
		return nil, NewError(400, "Bad Request: invalid story_id")
	}
	req := &tg.StoriesSendStoryRequest{
		Peer:         peer,
		FwdFromID:    fromPeer,
		FwdFromStory: int32(storyID),
		RandomID:     randomID(),
	}
	if q.ArgBool("protect_content") {
		req.Noforwards = true
	}
	if q.ArgBool("post_to_chat_page") {
		req.Pinned = true
	}
	if ap := q.Arg("active_period"); ap != "" {
		if v, err := strconv.ParseInt(ap, 10, 32); err == nil && v > 0 {
			req.Period = int32(v)
		}
	}
	req.SetFlags()
	_, err = c.rpc.StoriesSendStory(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// approveSuggestedPost implements the Bot API approveSuggestedPost method.
// Uses messages.toggleSuggestedPostApproval at the MTProto level.
func (c *Client) approveSuggestedPost(ctx context.Context, q *server.Query) (any, error) {
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
	req := &tg.MessagesToggleSuggestedPostApprovalRequest{
		Peer:  peer,
		MsgID: int32(msgID),
	}
	if sendDate := q.Arg("send_date"); sendDate != "" {
		if v, err := strconv.ParseInt(sendDate, 10, 32); err == nil {
			req.ScheduleDate = int32(v)
			req.Flags.Set(0)
		}
	}
	req.SetFlags()
	_, err = c.rpc.MessagesToggleSuggestedPostApproval(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// declineSuggestedPost implements the Bot API declineSuggestedPost method.
// Uses messages.toggleSuggestedPostApproval with Reject flag.
func (c *Client) declineSuggestedPost(ctx context.Context, q *server.Query) (any, error) {
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
	req := &tg.MessagesToggleSuggestedPostApprovalRequest{
		Reject: true,
		Peer:   peer,
		MsgID:  int32(msgID),
	}
	if comment := q.Arg("comment"); comment != "" {
		req.RejectComment = comment
		req.Flags.Set(2)
	}
	req.SetFlags()
	_, err = c.rpc.MessagesToggleSuggestedPostApproval(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// sendMessageDraft implements the Bot API sendMessageDraft method.
// Uses messages.setTyping with sendMessageTextDraftAction (NOT messages.saveDraft
// — this is a "user is typing a draft" broadcast, like sendChatAction). draft_id is
// forwarded verbatim as the action's random_id. Mirrors Client.cpp
// process_send_message_draft_query (Client.cpp:14158-14172) + TDLib sendTextMessageDraft.
func (c *Client) sendMessageDraft(ctx context.Context, q *server.Query) (any, error) {
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	rawText := q.Arg("text")
	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	text, entities, err := convert.FormattedText(rawText, q.Arg("parse_mode"), q.Arg("entities"))
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}
	draftID, _ := q.ArgInt64("draft_id")
	_, err = c.rpc.MessagesSetTyping(ctx, &tg.MessagesSetTypingRequest{
		Peer: peer,
		Action: &tg.SendMessageTextDraftAction{
			RandomID: draftID,
			Text: &tg.TextWithEntities{
				Text:     text,
				Entities: entities,
			},
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// parseInputRichMessage parses the Bot API `rich_message` JSON ({markdown|html,
// is_rtl, skip_entity_detection}) into a TL InputRichMessage. Mirrors the reference
// get_input_rich_message (Client.cpp). Used by the rich send/edit paths.
func parseInputRichMessage(richJSON string) (tg.InputRichMessageClass, error) {
	var rm struct {
		Markdown            string `json:"markdown"`
		HTML                string `json:"html"`
		IsRtl               bool   `json:"is_rtl"`
		SkipEntityDetection bool   `json:"skip_entity_detection"`
	}
	if err := json.Unmarshal([]byte(richJSON), &rm); err != nil {
		return nil, NewError(400, "Bad Request: can't parse rich message JSON")
	}
	var richMsg tg.InputRichMessageClass
	if rm.Markdown != "" {
		richMsg = &tg.InputRichMessageMarkdown{Rtl: rm.IsRtl, Markdown: rm.Markdown}
	} else if rm.HTML != "" {
		richMsg = &tg.InputRichMessageHTML{Rtl: rm.IsRtl, HTML: rm.HTML}
	} else {
		return nil, NewError(400, "Bad Request: rich_message must contain markdown or html")
	}
	if sf, ok := richMsg.(interface{ SetFlags() }); ok {
		sf.SetFlags()
	}
	return richMsg, nil
}

// sendRichMessageDraft implements the Bot API sendRichMessageDraft method.
// Uses messages.setTyping with inputSendMessageRichMessageDraftAction (NOT messages.saveDraft).
// Reference: TDLib DialogAction::RichTextDraft → inputSendMessageRichMessageDraftAction.
func (c *Client) sendRichMessageDraft(ctx context.Context, q *server.Query) (any, error) {
	richJSON := q.Arg("rich_message")
	if richJSON == "" {
		return nil, NewError(400, "Bad Request: rich message must be non-empty")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}
	var rm struct {
		Markdown            string `json:"markdown"`
		HTML                string `json:"html"`
		IsRtl               bool   `json:"is_rtl"`
		SkipEntityDetection bool   `json:"skip_entity_detection"`
	}
	if err := json.Unmarshal([]byte(richJSON), &rm); err != nil {
		return nil, NewError(400, "Bad Request: can't parse rich message JSON")
	}
	var richMsg tg.InputRichMessageClass
	if rm.Markdown != "" {
		richMsg = &tg.InputRichMessageMarkdown{
			Rtl:      rm.IsRtl,
			Markdown: rm.Markdown,
		}
	} else if rm.HTML != "" {
		richMsg = &tg.InputRichMessageHTML{
			Rtl:  rm.IsRtl,
			HTML: rm.HTML,
		}
	} else {
		return nil, NewError(400, "Bad Request: rich_message must contain markdown or html")
	}
	if sf, ok := richMsg.(interface{ SetFlags() }); ok {
		sf.SetFlags()
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	// draft_id is passed through as the typing action's random_id. The official
	// server does NOT generate one when omitted — it forwards draft_id verbatim
	// (0 when absent → RANDOM_ID_INVALID). Mirrors Client.cpp
	// process_send_rich_message_draft_query + TDLib sendRichMessageDraft.
	draftID, _ := q.ArgInt64("draft_id")

	_, err = c.rpc.MessagesSetTyping(ctx, &tg.MessagesSetTypingRequest{
		Peer: peer,
		Action: &tg.InputSendMessageRichMessageDraftAction{
			RandomID:    draftID,
			RichMessage: richMsg,
		},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// sendRichMessage implements the Bot API sendRichMessage method.
// Reference: Client.cpp:13560. Sends a rich message via MessagesSendMessage with RichMessage field.
// Params: chat_id, rich_message (JSON: markdown/html + is_rtl + skip_entity_detection).
func (c *Client) sendRichMessage(ctx context.Context, q *server.Query) (any, error) {
	richJSON := q.Arg("rich_message")
	if richJSON == "" {
		return nil, NewError(400, "Bad Request: rich message must be non-empty")
	}
	chatID := q.Arg("chat_id")
	if chatID == "" {
		return nil, NewError(400, "Bad Request: chat_id is empty")
	}

	// Parse the rich_message JSON.
	var rm struct {
		Markdown            string `json:"markdown"`
		HTML                string `json:"html"`
		IsRtl               bool   `json:"is_rtl"`
		SkipEntityDetection bool   `json:"skip_entity_detection"`
	}
	if err := json.Unmarshal([]byte(richJSON), &rm); err != nil {
		return nil, NewError(400, "Bad Request: can't parse rich message JSON")
	}

	// Build the InputRichMessage from markdown or html source.
	var richMsg tg.InputRichMessageClass
	if rm.Markdown != "" {
		richMsg = &tg.InputRichMessageMarkdown{
			Rtl:      rm.IsRtl,
			Markdown: rm.Markdown,
		}
	} else if rm.HTML != "" {
		richMsg = &tg.InputRichMessageHTML{
			Rtl:  rm.IsRtl,
			HTML: rm.HTML,
		}
	} else {
		return nil, NewError(400, "Bad Request: rich_message must contain markdown or html")
	}
	// Always call SetFlags to set the TL flags for optional fields.
	if sf, ok := richMsg.(interface{ SetFlags() }); ok {
		sf.SetFlags()
	}

	if err := c.ensureConnected(ctx); err != nil {
		return nil, &Error{Code: 502, Description: "Bad Gateway: failed to connect"}
	}
	peer, err := convert.ResolvePeer(ctx, chatID, c.store)
	if err != nil {
		return nil, NewError(400, "Bad Request: "+err.Error())
	}

	req := &tg.MessagesSendMessageRequest{
		Peer:        peer,
		Message:     "",
		RandomID:    randomID(),
		RichMessage: richMsg,
	}
	req.SetFlags()

	result, err := c.rpc.MessagesSendMessage(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	msg := extractMessageFromUpdates(result)
	if msg == nil {
		return true, nil
	}
	// Enrich partial messages from UpdateShortSentMessage.
	c.enrichPartialMessage(msg, peer, "")
	c.cachePeersFromMessage(ctx, msg)

	// Convert to Bot API format (enrichment is handled by botMessage).
	out := c.botMessage(ctx, msg, extractChats(result))

	return out, nil
}

// sendChecklist implements the Bot API sendChecklist method.
// Reference: Client.cpp:11899. Uses messages.sendMedia with InputMediaTodo.
// Params: chat_id, checklist (JSON: title, tasks[], others_can_add_tasks, others_can_mark_tasks_as_done).
func (c *Client) sendChecklist(ctx context.Context, q *server.Query) (any, error) {
	todo, err := parseChecklistJSON(q.Arg("checklist"))
	if err != nil {
		return nil, err
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
	req := &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    &tg.InputMediaTodo{Todo: todo},
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

// editMessageChecklist implements the Bot API editMessageChecklist method.
// Reference: Client.cpp:12347. Uses messages.editMessage with InputMediaTodo.
func (c *Client) editMessageChecklist(ctx context.Context, q *server.Query) (any, error) {
	todo, err := parseChecklistJSON(q.Arg("checklist"))
	if err != nil {
		return nil, err
	}
	peer, id, err := c.resolveTargetMessage(ctx, q)
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesEditMessageRequest{
		Peer:  peer,
		ID:    id,
		Media: &tg.InputMediaTodo{Todo: todo},
	}
	req.SetFlags()
	return c.invokeEdit(ctx, req, businessConnID(q))
}

// parseChecklistJSON parses the Bot API checklist JSON into tg.TodoList.
func parseChecklistJSON(raw string) (*tg.TodoList, error) {
	if raw == "" {
		return nil, NewError(400, "Bad Request: parameter \"checklist\" is required")
	}
	var cl struct {
		Title string `json:"title"`
		Tasks []struct {
			ID   int32  `json:"id"`
			Text string `json:"text"`
		} `json:"tasks"`
		OthersCanAddTasks        bool `json:"others_can_add_tasks"`
		OthersCanMarkTasksAsDone bool `json:"others_can_mark_tasks_as_done"`
	}
	if err := json.Unmarshal([]byte(raw), &cl); err != nil {
		return nil, NewError(400, "Bad Request: can't parse checklist JSON")
	}
	if cl.Title == "" {
		return nil, NewError(400, "Bad Request: parameter \"checklist title\" is required")
	}
	items := make([]*tg.TodoItem, 0, len(cl.Tasks))
	for _, t := range cl.Tasks {
		items = append(items, &tg.TodoItem{
			ID:    t.ID,
			Title: &tg.TextWithEntities{Text: t.Text},
		})
	}
	todo := &tg.TodoList{
		Title:             &tg.TextWithEntities{Text: cl.Title},
		List:              items,
		OthersCanAppend:   cl.OthersCanAddTasks,
		OthersCanComplete: cl.OthersCanMarkTasksAsDone,
	}
	todo.SetFlags()
	return todo, nil
}

// deleteMessageReaction (deletemessagereaction) deletes a specific sender's reactions
// from a single message → messages.deleteParticipantReaction (per-message, per-sender).
// Mirrors Client.cpp process_delete_message_reaction_query → TDLib
// deleteMessageReactionsFromSender → DeleteParticipantReactionQuery. The sender is
// user_id OR actor_chat_id; deleteAllMessageReactions below is the no-message variant.
func (c *Client) deleteMessageReaction(ctx context.Context, q *server.Query) (any, error) {
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
	// Resolve the participant whose reactions to delete (user_id or actor_chat_id).
	var participant tg.InputPeerClass
	if userIDStr := q.Arg("user_id"); userIDStr != "" {
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return nil, NewError(400, "Bad Request: invalid user_id")
		}
		participant = &tg.InputPeerUser{UserID: userID}
	} else if actorChatID := q.Arg("actor_chat_id"); actorChatID != "" {
		participant, err = convert.ResolvePeer(ctx, actorChatID, c.store)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
	} else {
		return nil, NewError(400, "Bad Request: parameter \"user_id\" or \"actor_chat_id\" is required")
	}
	_, err = c.rpc.MessagesDeleteParticipantReaction(ctx, &tg.MessagesDeleteParticipantReactionRequest{
		Peer:        peer,
		MsgID:       int32(msgID),
		Participant: participant,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// deleteAllMessageReactions implements the Bot API deleteAllMessageReactions method.
// Uses messages.deleteParticipantReaction (MTProto: 0xe3b7f82c).
func (c *Client) deleteAllMessageReactions(ctx context.Context, q *server.Query) (any, error) {
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
	// Resolve the participant (user_id or actor_chat_id).
	var participant tg.InputPeerClass
	if userIDStr := q.Arg("user_id"); userIDStr != "" {
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return nil, NewError(400, "Bad Request: invalid user_id")
		}
		participant = &tg.InputPeerUser{UserID: userID}
	} else if actorChatID := q.Arg("actor_chat_id"); actorChatID != "" {
		actorPeer, err := convert.ResolvePeer(ctx, actorChatID, c.store)
		if err != nil {
			return nil, NewError(400, "Bad Request: "+err.Error())
		}
		participant = actorPeer
	} else {
		return nil, NewError(400, "Bad Request: parameter \"user_id\" or \"actor_chat_id\" is required")
	}
	_, err = c.rpc.MessagesDeleteParticipantReactions(ctx, &tg.MessagesDeleteParticipantReactionsRequest{
		Peer:        peer,
		Participant: participant,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// sendLivePhoto implements the Bot API sendLivePhoto method.
// Uses InputMediaUploadedPhoto with live_photo flag and video InputDocument.
// TL: inputMediaUploadedPhoto#7d8375da flags:# live_photo:flags.3?true file:InputFile video:flags.3?InputDocument
func (c *Client) sendLivePhoto(ctx context.Context, q *server.Query) (any, error) {
	photoFile, ok := q.File("photo")
	if !ok {
		return nil, NewError(400, "Bad Request: there is no live photo in the request")
	}
	_ = photoFile
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
	// Build the live photo media — InputMediaUploadedPhoto with LivePhoto=true.
	media := &tg.InputMediaUploadedPhoto{
		File:      inputFile,
		LivePhoto: true,
		Spoiler:   q.ArgBool("has_spoiler"),
	}
	media.SetFlags()
	req := &tg.MessagesSendMediaRequest{
		Peer:     peer,
		Media:    media,
		RandomID: randomID(),
	}
	req.InvertMedia = q.ArgBool("show_caption_above_media")
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
