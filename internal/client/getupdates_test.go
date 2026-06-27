package client

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

// Regression (A6): allowed_updates is now applied at push time via updateAllowed
// (mirrors add_update_impl, Client.cpp:17706) — nil allowed → default exclusions;
// non-nil → explicit allowlist; "" (undeterminable) → keep.
func TestUpdateAllowed(t *testing.T) {
	// Default (nil allowed): official DEFAULT_ALLOWED_UPDATE_TYPES excludes
	// chat_member, message_reaction, and message_reaction_count only.
	c := &Client{}
	for typ, want := range map[string]bool{
		"message": true, "edited_message": true, "callback_query": true, "poll": true,
		"chat_member": false, "message_reaction": false, "message_reaction_count": false,
		"chat_boost": true, "removed_chat_boost": true,
		"": true, // undeterminable → keep
	} {
		if got := c.updateAllowed(typ); got != want {
			t.Errorf("default updateAllowed(%q) = %v, want %v", typ, got, want)
		}
	}

	// Explicit allowlist: only listed types pass.
	c.allowedUpdates = map[string]bool{"message": true, "poll": true}
	for typ, want := range map[string]bool{
		"message": true, "poll": true, "callback_query": false, "chat_member": false,
	} {
		if got := c.updateAllowed(typ); got != want {
			t.Errorf("allowlist updateAllowed(%q) = %v, want %v", typ, got, want)
		}
	}
}

func TestAllowedUpdatesParsingMatchesOfficialMaskRules(t *testing.T) {
	set, normalized, ok := parseAllowedUpdates(`["MESSAGE","poll","unknown","message"]`)
	if !ok {
		t.Fatal("parseAllowedUpdates returned ok=false")
	}
	if !set["message"] || !set["poll"] || set["callback_query"] {
		t.Fatalf("set = %+v", set)
	}
	if normalized != `["message","poll"]` {
		t.Fatalf("normalized = %s", normalized)
	}

	set, normalized, ok = parseAllowedUpdates(`[]`)
	if !ok {
		t.Fatal("empty array should be valid and reset to default")
	}
	if set != nil || normalized != "" {
		t.Fatalf("empty array parsed to set=%+v normalized=%q, want default", set, normalized)
	}

	if _, _, ok := parseAllowedUpdates(`{"not":"array"}`); ok {
		t.Fatal("object should be invalid and leave current state unchanged")
	}
	if _, _, ok := parseAllowedUpdates(`not-json`); ok {
		t.Fatal("invalid JSON should leave current state unchanged")
	}
}

func TestApplyAllowedUpdatesRawInvalidDoesNotChangeState(t *testing.T) {
	c := &Client{allowedUpdates: map[string]bool{"message": true}}
	if c.applyAllowedUpdatesRaw(`not-json`) {
		t.Fatal("invalid allowed_updates unexpectedly applied")
	}
	if !c.allowedUpdates["message"] || len(c.allowedUpdates) != 1 {
		t.Fatalf("allowedUpdates changed: %+v", c.allowedUpdates)
	}
}

func TestGetUpdatesNegativeOffsetKeepsNewestEvents(t *testing.T) {
	tq := tqueue.New()
	c := NewClient(Params{TQueue: tq}, "123:tok")
	qid := c.queueID()
	_, _ = tq.Push(context.Background(), qid, []byte(`{"update_id":1,"message":{"message_id":1}}`), 0, 0, 0)
	_, _ = tq.Push(context.Background(), qid, []byte(`{"update_id":2,"message":{"message_id":2}}`), 0, 0, 0)
	_, _ = tq.Push(context.Background(), qid, []byte(`{"update_id":3,"message":{"message_id":3}}`), 0, 0, 0)

	result, err := c.getUpdates(context.Background(), queryWithArgs(map[string]string{
		"offset": "-1",
		"limit":  "100",
	}))
	if err != nil {
		t.Fatal(err)
	}
	raw := result.([]json.RawMessage)
	if len(raw) != 1 {
		t.Fatalf("len(updates) = %d, want 1: %s", len(raw), raw)
	}
	var got struct {
		UpdateID int64 `json:"update_id"`
		Message  struct {
			MessageID int `json:"message_id"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw[0], &got); err != nil {
		t.Fatal(err)
	}
	if got.UpdateID != 3 || got.Message.MessageID != 3 {
		t.Fatalf("update = %+v, want newest event id/message 3", got)
	}
	if tq.Size(qid) != 1 {
		t.Fatalf("queue size = %d, want 1", tq.Size(qid))
	}
}

func TestGetUpdatesRepeatedOffsetShapesLimit(t *testing.T) {
	tq := tqueue.New()
	c := NewClient(Params{TQueue: tq}, "123:tok")
	qid := c.queueID()
	_, _ = tq.Push(context.Background(), qid, []byte(`{"message":{"message_id":1}}`), 0, 0, 0)
	_, _ = tq.Push(context.Background(), qid, []byte(`{"message":{"message_id":2}}`), 0, 0, 0)

	q := queryWithArgs(map[string]string{"offset": "0", "limit": "100"})
	if _, err := c.getUpdates(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	result, err := c.getUpdates(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	raw := result.([]json.RawMessage)
	if len(raw) != 1 {
		t.Fatalf("len(updates) = %d, want 1 after repeated same offset within 0.5s", len(raw))
	}
}

func TestGetUpdatesConcurrentLongPollReturnsConflict(t *testing.T) {
	tq := tqueue.New()
	c := NewClient(Params{TQueue: tq}, "123:tok")
	firstErr := make(chan error, 1)
	go func() {
		_, err := c.getUpdates(context.Background(), queryWithArgs(map[string]string{"timeout": "5"}))
		firstErr <- err
	}()

	deadline := time.After(2 * time.Second)
	for {
		c.longPollMu.Lock()
		installed := c.longPollCancel != nil
		c.longPollMu.Unlock()
		if installed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("first long poll was not installed")
		case <-time.After(10 * time.Millisecond):
		}
	}

	_, err := c.getUpdates(context.Background(), queryWithArgs(map[string]string{"timeout": "5"}))
	if err != nil {
		t.Fatalf("second long poll returned %v, want nil empty result", err)
	}
	select {
	case err := <-firstErr:
		apiErr, ok := err.(*Error)
		if !ok {
			t.Fatalf("first long poll err = %T %v, want *Error", err, err)
		}
		if apiErr.Code != 409 ||
			apiErr.Description != "Conflict: terminated by other getUpdates request; make sure that only one bot instance is running" {
			t.Fatalf("first long poll error = %+v", apiErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first long poll did not return conflict")
	}
}

func TestGetUpdatesConflictDelayWhenRecentlyConflicted(t *testing.T) {
	c := NewClient(Params{}, "123:tok")
	conflicts := make(chan longPollConflict, 1)
	c.longPollMu.Lock()
	c.longPollCancel = func() {}
	c.longPollConflict = conflicts
	c.nextGetUpdatesConflictTime = time.Now().Add(3 * time.Second)
	c.abortLongPollLocked(false)
	c.longPollMu.Unlock()

	select {
	case conflict := <-conflicts:
		if conflict.err == nil || conflict.err.Code != 409 {
			t.Fatalf("conflict err = %+v, want 409", conflict.err)
		}
		if conflict.after != 3*time.Second {
			t.Fatalf("conflict delay = %v, want 3s", conflict.after)
		}
	default:
		t.Fatal("conflict was not sent")
	}
}

func TestEventsToRawHonorsOfficialByteBudget(t *testing.T) {
	payload := largeUpdatePayload(getUpdatesMaxJSONBytes / 2)
	got := eventsToRaw([]tqueue.Event{
		{ID: 1, Data: payload},
		{ID: 2, Data: payload},
	})
	if len(got) != 1 {
		t.Fatalf("len(eventsToRaw) = %d, want 1", len(got))
	}
	var update struct {
		UpdateID int64 `json:"update_id"`
	}
	if err := json.Unmarshal(got[0], &update); err != nil {
		t.Fatal(err)
	}
	if update.UpdateID != 1 {
		t.Fatalf("update_id = %d, want 1", update.UpdateID)
	}
}

func queryWithArgs(args map[string]string) *server.Query {
	q := server.NewQuery()
	q.Method = "getupdates"
	q.Args = args
	return q
}

func largeUpdatePayload(size int) []byte {
	prefix := `{"update_id":1,"message":{"text":"`
	suffix := `"}}`
	return []byte(prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix)
}
