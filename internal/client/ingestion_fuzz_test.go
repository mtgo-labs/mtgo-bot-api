package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/tqueue"
)

// FuzzPushUpdateInjection fuzzes the update_id injection + enqueue path:
// arbitrary JSON input is pushed through pushUpdateObj and any queued data must
// always be valid JSON with update_id present. (Adapted from the former
// injectUpdateID fuzz target — injection now happens at push time via
// PushWithData instead of a standalone byte-level function.)
func FuzzPushUpdateInjection(f *testing.F) {
	f.Add([]byte(`{"message":{"message_id":1}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"channel_post":{"message_id":99,"chat":{"id":1,"type":"channel"}}}`))
	f.Add([]byte(`{"edited_message":{"message_id":5}}`))
	f.Add([]byte(`{"my_chat_member":{"from":{"id":1},"chat":{"id":2}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return // not a JSON object — skip
		}

		tq := tqueue.New()
		c := NewClient(Params{TQueue: tq}, "111:secret")
		c.pushUpdateObj(obj) // must not panic

		events, err := tq.Get(context.Background(), c.queueID(), 0, false, 1<<30, 100)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		for _, e := range events {
			var check map[string]any
			if err := json.Unmarshal(e.Data, &check); err != nil {
				t.Errorf("queued data not valid JSON: %v\n%s", err, e.Data)
			}
			if _, ok := check["update_id"]; !ok {
				t.Errorf("update_id missing from queued data:\n%s", e.Data)
			}
		}
	})
}
