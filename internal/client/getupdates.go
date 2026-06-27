package client

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

func init() {
	Register("getupdates", (*Client).getUpdates)
}

const (
	getUpdatesDefaultLimit = 100
	getUpdatesMaxLimit     = 100
	getUpdatesPollInterval = 100 * time.Millisecond
	getUpdatesMaxJSONBytes = 1 << 22
	// longPollMaxTimeout caps the getUpdates timeout (Client.h:1634
	// LONG_POLL_MAX_TIMEOUT). The reference clamps timeout to [0, 50].
	longPollMaxTimeout = 50
)

type longPollConflict struct {
	err   *Error
	after time.Duration
}

// getUpdates implements the Bot API getUpdates method: long-poll the per-bot
// update queue, returning events strictly after the supplied offset (which also
// confirms/forgets all earlier events). Mirrors Client.cpp
// process_get_updates_query.
//
// Parameters: offset (first id to return; confirms earlier), limit (1-100,
// default 100), timeout (long-poll seconds), allowed_updates (update types to receive).
func (c *Client) getUpdates(ctx context.Context, q *server.Query) (any, error) {
	// Webhook mode takes precedence — getUpdates is forbidden while a webhook
	// is active (Bot API returns 409 Conflict). Mirrors Client.cpp behavior.
	if c.hasActiveWebhook() {
		return nil, NewError(409, "Conflict: can't use getUpdates method while webhook is active; use deleteWebhook to delete the webhook first")
	}
	if c.params.TQueue == nil {
		return []json.RawMessage{}, nil
	}
	qid := c.queueID()

	offset := int64(0)
	if q.HasArg("offset") {
		if v, err := q.ArgInt64("offset"); err == nil {
			offset = v
		}
	}
	limit := getUpdatesDefaultLimit
	if q.HasArg("limit") {
		if v, err := q.ArgInt64("limit"); err == nil && v > 0 {
			limit = int(v)
		}
	}
	if limit > getUpdatesMaxLimit {
		limit = getUpdatesMaxLimit
	}
	timeoutSec := 0
	if q.HasArg("timeout") {
		if v, err := q.ArgInt64("timeout"); err == nil {
			timeoutSec = int(v)
		}
	}
	// Clamp to [0, LONG_POLL_MAX_TIMEOUT] (Client.cpp:16443).
	if timeoutSec < 0 {
		timeoutSec = 0
	}
	if timeoutSec > longPollMaxTimeout {
		timeoutSec = longPollMaxTimeout
	}
	nowTime := time.Now()
	c.longPollMu.Lock()
	if offset == c.previousGetUpdatesOffset && timeoutSec < 3 && nowTime.Before(c.previousGetUpdatesStart.Add(3*time.Second)) {
		timeoutSec = 3
	}
	if offset == c.previousGetUpdatesOffset && nowTime.Before(c.previousGetUpdatesStart.Add(500*time.Millisecond)) {
		limit = 1
	}
	c.previousGetUpdatesOffset = offset
	c.previousGetUpdatesStart = nowTime
	c.longPollMu.Unlock()
	// allowed_updates: parse + store per-bot (persists across calls).
	if q.HasArg("allowed_updates") {
		c.applyAllowedUpdatesRaw(q.Arg("allowed_updates"))
	}
	// allowed_updates is applied at push time (pushUpdateObj), mirroring the
	// reference add_update_impl (Client.cpp:17706); getUpdates returns all
	// enqueued events without re-filtering.

	var fromID tqueue.EventID
	forgetPrevious := false
	if offset < 0 {
		if c.params.TQueue != nil {
			c.params.TQueue.Clear(ctx, qid, int(-offset))
		}
		offset = 0
	}
	if offset > 0 {
		fromID = tqueue.EventID(offset) - 1
		forgetPrevious = true
	}

	// Long-poll coordination (A3): only one long poll per bot. A new getUpdates
	// with a timeout aborts any in-flight long poll with the reference 409
	// conflict (Client.cpp abort_long_poll, do_get_updates).
	pollCtx := ctx
	var lpCancel context.CancelFunc
	var conflictCh chan longPollConflict
	if timeoutSec > 0 {
		c.longPollMu.Lock()
		if c.longPollCancel != nil {
			c.abortLongPollLocked(false)
		}
		pollCtx, lpCancel = context.WithCancel(ctx)
		c.longPollCancel = lpCancel
		conflictCh = make(chan longPollConflict, 1)
		c.longPollConflict = conflictCh
		c.longPollMu.Unlock()
	}
	defer func() {
		if lpCancel == nil {
			return
		}
		lpCancel()
		c.longPollMu.Lock()
		if c.longPollConflict == conflictCh {
			c.longPollCancel = nil
			c.longPollConflict = nil
		}
		c.longPollMu.Unlock()
	}()

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	pollTimer := time.NewTimer(0)
	if !pollTimer.Stop() {
		<-pollTimer.C
	}
	defer pollTimer.Stop()
	for {
		now := int32(time.Now().Unix())
		events, err := c.params.TQueue.Get(ctx, qid, fromID, forgetPrevious, now, limit)
		if err != nil {
			return nil, rpcError(err)
		}
		// forgetPrevious applies only to the first fetch (the confirmation of
		// the offset); subsequent poll iterations must not re-forget.
		forgetPrevious = false

		if len(events) > 0 {
			c.markGetUpdatesFinished()
			return eventsToRaw(events), nil
		}
		// No events: long-poll until timeout or request cancellation.
		if timeoutSec <= 0 || time.Now().After(deadline) {
			c.markGetUpdatesFinished()
			return []json.RawMessage{}, nil
		}
		pollTimer.Reset(getUpdatesPollInterval)
		select {
		case conflict := <-conflictCh:
			if conflict.after > 0 {
				timer := time.NewTimer(conflict.after)
				select {
				case <-timer.C:
				case <-ctx.Done():
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					c.markGetUpdatesFinished()
					return []json.RawMessage{}, nil
				}
			}
			return nil, conflict.err
		case <-pollCtx.Done():
			c.markGetUpdatesFinished()
			return []json.RawMessage{}, nil
		case <-pollTimer.C:
		}
	}
}

func (c *Client) markGetUpdatesFinished() {
	c.longPollMu.Lock()
	c.previousGetUpdatesFinish = time.Now()
	c.longPollMu.Unlock()
}

func (c *Client) abortLongPollLocked(fromSetWebhook bool) {
	if c.longPollCancel == nil {
		return
	}
	message := "Conflict: terminated by other getUpdates request; make sure that only one bot instance is running"
	if fromSetWebhook {
		message = "Conflict: terminated by setWebhook request"
	}
	now := time.Now()
	after := time.Duration(0)
	if now.Before(c.nextGetUpdatesConflictTime) {
		after = 3 * time.Second
	} else {
		c.nextGetUpdatesConflictTime = now.Add(3 * time.Second)
	}
	err := NewError(409, message)
	sent := false
	if c.longPollConflict != nil {
		select {
		case c.longPollConflict <- longPollConflict{err: err, after: after}:
			sent = true
		default:
		}
	}
	if !sent {
		c.longPollCancel()
	}
}

// eventsToRaw converts queue Events into a JSON array of Bot API Update
// objects. Client-ingested events are stored with update_id already included at
// push time, so this path does not reparse or rewrite JSON.
func eventsToRaw(events []tqueue.Event) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(events))
	leftLen := getUpdatesMaxJSONBytes
	for i := range events {
		leftLen -= 50 + len(events[i].Data)
		if leftLen <= 0 {
			break
		}
		out = append(out, json.RawMessage(events[i].Data))
	}
	return out
}

var updateTypeOrder = []string{
	"message",
	"edited_message",
	"channel_post",
	"edited_channel_post",
	"inline_query",
	"chosen_inline_result",
	"callback_query",
	"custom_event",
	"custom_query",
	"shipping_query",
	"pre_checkout_query",
	"poll",
	"poll_answer",
	"my_chat_member",
	"chat_member",
	"chat_join_request",
	"chat_boost",
	"removed_chat_boost",
	"message_reaction",
	"message_reaction_count",
	"business_connection",
	"business_message",
	"edited_business_message",
	"deleted_business_messages",
	"purchased_paid_media",
	"managed_bot",
	"guest_message",
}

var updateTypeBits = func() map[string]uint32 {
	m := make(map[string]uint32, len(updateTypeOrder))
	for i, name := range updateTypeOrder {
		m[name] = 1 << uint(i)
	}
	return m
}()

var defaultAllowedUpdateMask = (uint32(1) << uint(len(updateTypeOrder))) - 1 -
	(1 << 14) - // chat_member
	(1 << 18) - // message_reaction
	(1 << 19) // message_reaction_count

// updateAllowed reports whether an update type passes the bot's allowed_updates
// filter at push time (mirrors add_update_impl, Client.cpp:17706). nil allowed →
// default exclusions; "" type (undeterminable) → keep.
func (c *Client) updateAllowed(typ string) bool {
	if typ == "" {
		return true
	}
	c.mu.Lock()
	allowed := c.allowedUpdates
	c.mu.Unlock()
	if allowed != nil {
		return allowed[typ]
	}
	bit, ok := updateTypeBits[typ]
	if !ok {
		return true
	}
	return defaultAllowedUpdateMask&bit != 0
}

func (c *Client) currentAllowedUpdatesJSON() string {
	c.mu.Lock()
	allowed := c.allowedUpdates
	c.mu.Unlock()
	if allowed == nil {
		return ""
	}
	var mask uint32
	for name := range allowed {
		if bit, ok := updateTypeBits[name]; ok {
			mask |= bit
		}
	}
	if mask == 0 || mask == defaultAllowedUpdateMask {
		return ""
	}
	normalized, _ := json.Marshal(allowedUpdatesNames(mask, false))
	return string(normalized)
}

func (c *Client) applyAllowedUpdatesRaw(raw string) bool {
	set, _, ok := parseAllowedUpdates(raw)
	if !ok {
		return false
	}
	c.mu.Lock()
	c.allowedUpdates = set
	c.mu.Unlock()
	return true
}

func (c *Client) applyAllowedUpdatesNormalized(raw string) {
	if raw == "" {
		c.mu.Lock()
		c.allowedUpdates = nil
		c.mu.Unlock()
		return
	}
	set, _, ok := parseAllowedUpdates(raw)
	if !ok {
		return
	}
	c.mu.Lock()
	c.allowedUpdates = set
	c.mu.Unlock()
}

func parseAllowedUpdates(raw string) (map[string]bool, string, bool) {
	mask, ok := allowedUpdatesMask(raw)
	if !ok {
		return nil, "", false
	}
	if mask == defaultAllowedUpdateMask {
		return nil, "", true
	}
	names := allowedUpdatesNames(mask, false)
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	normalized, _ := json.Marshal(names)
	return set, string(normalized), true
}

func allowedUpdatesMask(raw string) (uint32, bool) {
	if raw == "" {
		return 0, false
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return 0, false
	}
	var mask uint32
	for _, name := range names {
		if bit, ok := updateTypeBits[strings.ToLower(name)]; ok {
			mask |= bit
		}
	}
	if mask == 0 {
		return defaultAllowedUpdateMask, true
	}
	return mask, true
}

func allowedUpdatesNames(mask uint32, includeInternal bool) []string {
	if mask == defaultAllowedUpdateMask {
		return nil
	}
	out := make([]string, 0, len(updateTypeOrder))
	for i, name := range updateTypeOrder {
		if !includeInternal && (name == "custom_event" || name == "custom_query") {
			continue
		}
		if mask&(1<<uint(i)) != 0 {
			out = append(out, name)
		}
	}
	return out
}
