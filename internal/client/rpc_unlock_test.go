package client

import (
	"context"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
	"github.com/mtgo-labs/mtgo/tgerr"
)

// TestSendMessageRecordsRequest proves the rpcInvoker unlock: we can verify
// the exact MTProto request a handler builds, without a live connection.
func TestSendMessageRecordsRequest(t *testing.T) {
	r := &recorder{}
	c := newTestClient(r)

	_, _ = c.sendMessage(context.Background(), newQ("sendmessage", map[string]string{
		"chat_id": "999",
		"text":    "hello",
	}))

	req := wantSendMessage(t, r)
	if req.Message != "hello" {
		t.Errorf("Message = %q, want %q", req.Message, "hello")
	}
}

// TestSendMessageCannedResponse proves the recorder can return a canned TL
// response, enabling tests for response conversion (MTProto → Bot API JSON).
func TestSendMessageCannedResponse(t *testing.T) {
	r := &recorder{
		response: &tg.Updates{
			Updates: []tg.UpdateClass{
				&tg.UpdateNewMessage{
					Message: &tg.Message{
						ID:      42,
						Message: "hello world",
						PeerID:  &tg.PeerUser{UserID: 999},
						FromID:  &tg.PeerUser{UserID: 123},
					},
				},
			},
		},
	}
	c := newTestClient(r)

	result, err := c.sendMessage(context.Background(), newQ("sendmessage", map[string]string{
		"chat_id": "999",
		"text":    "hello world",
	}))
	if err != nil {
		t.Fatalf("sendMessage: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil — canned response not propagated")
	}
}

// TestSendMessageRPCError verifies RPC errors are correctly mapped via rpcError.
func TestSendMessageRPCError(t *testing.T) {
	r := &recorder{
		err: tgerr.New(400, "MESSAGE_NOT_MODIFIED"),
	}
	c := newTestClient(r)

	_, err := c.sendMessage(context.Background(), newQ("sendmessage", map[string]string{
		"chat_id": "999",
		"text":    "test",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err type = %T, want *Error", err)
	}
	if apiErr.Code != 400 {
		t.Errorf("Code = %d, want 400", apiErr.Code)
	}
}
