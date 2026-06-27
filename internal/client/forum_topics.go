package client

import (
	"context"
	"strconv"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// generalForumTopicID is the reserved topic ID for the "General" topic.
// Reference: telegram-bot-api/Client.h GENERAL_FORUM_TOPIC_ID = 1.
const generalForumTopicID int32 = 1

func init() {
	Register("createforumtopic", (*Client).createForumTopic)
	Register("editforumtopic", (*Client).editForumTopic)
	Register("closeforumtopic", (*Client).closeForumTopic)
	Register("reopenforumtopic", (*Client).reopenForumTopic)
	Register("deleteforumtopic", (*Client).deleteForumTopic)
	Register("unpinallforumtopicmessages", (*Client).unpinAllForumTopicMessages)
	Register("editgeneralforumtopic", (*Client).editGeneralForumTopic)
	Register("closegeneralforumtopic", (*Client).closeGeneralForumTopic)
	Register("reopengeneralforumtopic", (*Client).reopenGeneralForumTopic)
	Register("hidegeneralforumtopic", (*Client).hideGeneralForumTopic)
	Register("unhidegeneralforumtopic", (*Client).unhideGeneralForumTopic)
	Register("unpinallgeneralforumtopicmessages", (*Client).unpinAllGeneralForumTopicMessages)
}

// parseTopicID extracts the message_thread_id parameter as int32.
func parseTopicID(q *server.Query) (int32, error) {
	v := q.Arg("message_thread_id")
	if v == "" {
		return 0, NewError(400, "Bad Request: parameter \"message_thread_id\" is required")
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, NewError(400, "Bad Request: message_thread_id must be an integer")
	}
	return int32(n), nil
}

// createForumTopic creates a new forum topic.
// Reference: Client.cpp process_create_forum_topic_query.
// Params: chat_id, name, icon_color, icon_custom_emoji_id.
func (c *Client) createForumTopic(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	name := q.Arg("name")
	if name == "" {
		return nil, NewError(400, "Bad Request: parameter \"name\" is required")
	}
	req := &tg.MessagesCreateForumTopicRequest{
		Peer:     peer,
		Title:    name,
		RandomID: randomID(),
	}
	if v := q.Arg("icon_color"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 32); e == nil {
			req.IconColor = int32(n)
		}
	}
	if v := q.Arg("icon_custom_emoji_id"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			req.IconEmojiID = n
		}
	}
	res, err := c.rpc.MessagesCreateForumTopic(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	// The topic ID is the ID of the service message that created the topic.
	topicID := int32(0)
	if msg := extractMessageFromUpdates(res); msg != nil {
		topicID = msg.ID
	}
	return &apitypes.ForumTopicInfo{
		MessageThreadID:   topicID,
		Name:              name,
		IconColor:         req.IconColor,
		IconCustomEmojiID: strconv.FormatInt(req.IconEmojiID, 10),
	}, nil
}

// editForumTopic edits a topic's name or icon.
// Reference: Client.cpp process_edit_forum_topic_query.
// Params: chat_id, message_thread_id, name, icon_custom_emoji_id.
func (c *Client) editForumTopic(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	topicID, err := parseTopicID(q)
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesEditForumTopicRequest{
		Peer:    peer,
		TopicID: topicID,
		Title:   q.Arg("name"),
	}
	if v := q.Arg("icon_custom_emoji_id"); v != "" {
		if n, e := strconv.ParseInt(v, 10, 64); e == nil {
			req.IconEmojiID = n
		}
	}
	_, err = c.rpc.MessagesEditForumTopic(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// closeForumTopic closes a topic.
// Reference: Client.cpp process_close_forum_topic_query.
func (c *Client) closeForumTopic(ctx context.Context, q *server.Query) (any, error) {
	return c.toggleTopicClosed(ctx, q, true)
}

// reopenForumTopic reopens a topic.
// Reference: Client.cpp process_reopen_forum_topic_query.
func (c *Client) reopenForumTopic(ctx context.Context, q *server.Query) (any, error) {
	return c.toggleTopicClosed(ctx, q, false)
}

// toggleTopicClosed is the shared close/reopen path. For reopen (closed=false),
// the MTProto flag bit 2 must be explicitly set because the generated SetFlags
// only sets it when Closed==true; without the flag the server treats closed as
// unchanged rather than false.
func (c *Client) toggleTopicClosed(ctx context.Context, q *server.Query, closed bool) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	topicID, err := parseTopicID(q)
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesEditForumTopicRequest{
		Peer:    peer,
		TopicID: topicID,
		Closed:  closed,
	}
	if !closed {
		req.Flags.Set(2) // force the closed flag so the server sets closed=false
	}
	_, err = c.rpc.MessagesEditForumTopic(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// deleteForumTopic deletes a topic and its history.
// Reference: Client.cpp process_delete_forum_topic_query.
func (c *Client) deleteForumTopic(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	topicID, err := parseTopicID(q)
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.MessagesDeleteTopicHistory(ctx, &tg.MessagesDeleteTopicHistoryRequest{
		Peer:     peer,
		TopMsgID: topicID,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// unpinAllForumTopicMessages unpins all pinned messages in a topic.
// Reference: Client.cpp process_unpin_all_forum_topic_messages_query.
func (c *Client) unpinAllForumTopicMessages(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	topicID, err := parseTopicID(q)
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.MessagesUnpinAllMessages(ctx, &tg.MessagesUnpinAllMessagesRequest{
		Peer:     peer,
		TopMsgID: topicID,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// editGeneralForumTopic edits the General topic's name.
// Reference: Client.cpp process_edit_general_forum_topic_query.
func (c *Client) editGeneralForumTopic(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.MessagesEditForumTopic(ctx, &tg.MessagesEditForumTopicRequest{
		Peer:    peer,
		TopicID: generalForumTopicID,
		Title:   q.Arg("name"),
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// closeGeneralForumTopic closes the General topic.
func (c *Client) closeGeneralForumTopic(ctx context.Context, q *server.Query) (any, error) {
	return c.toggleGeneralTopicClosed(ctx, q, true)
}

// reopenGeneralForumTopic reopens the General topic.
func (c *Client) reopenGeneralForumTopic(ctx context.Context, q *server.Query) (any, error) {
	return c.toggleGeneralTopicClosed(ctx, q, false)
}

func (c *Client) toggleGeneralTopicClosed(ctx context.Context, q *server.Query, closed bool) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesEditForumTopicRequest{
		Peer:    peer,
		TopicID: generalForumTopicID,
		Closed:  closed,
	}
	if !closed {
		req.Flags.Set(2)
	}
	_, err = c.rpc.MessagesEditForumTopic(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// hideGeneralForumTopic hides the General topic.
func (c *Client) hideGeneralForumTopic(ctx context.Context, q *server.Query) (any, error) {
	return c.toggleGeneralTopicHidden(ctx, q, true)
}

// unhideGeneralForumTopic unhides the General topic.
func (c *Client) unhideGeneralForumTopic(ctx context.Context, q *server.Query) (any, error) {
	return c.toggleGeneralTopicHidden(ctx, q, false)
}

// toggleGeneralTopicHidden is the shared hide/unhide path. For unhide
// (hidden=false), the MTProto flag bit 3 must be explicitly set (same reason
// as the closed flag in toggleTopicClosed).
func (c *Client) toggleGeneralTopicHidden(ctx context.Context, q *server.Query, hidden bool) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	req := &tg.MessagesEditForumTopicRequest{
		Peer:    peer,
		TopicID: generalForumTopicID,
		Hidden:  hidden,
	}
	if !hidden {
		req.Flags.Set(3) // force the hidden flag so the server sets hidden=false
	}
	_, err = c.rpc.MessagesEditForumTopic(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}

// unpinAllGeneralForumTopicMessages unpins all messages in the General topic.
func (c *Client) unpinAllGeneralForumTopicMessages(ctx context.Context, q *server.Query) (any, error) {
	peer, err := resolvePeerForChat(ctx, c, q.Arg("chat_id"))
	if err != nil {
		return nil, err
	}
	_, err = c.rpc.MessagesUnpinAllMessages(ctx, &tg.MessagesUnpinAllMessagesRequest{
		Peer:     peer,
		TopMsgID: generalForumTopicID,
	})
	if err != nil {
		return nil, rpcError(err)
	}
	return true, nil
}
