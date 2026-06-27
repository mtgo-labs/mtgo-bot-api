package client

import (
	"context"
	"errors"
	"testing"

	"github.com/mtgo-labs/mtgo/tgerr"
)

// rpcErr builds a *tgerr.Error the way mtgo does (deriving Type/Argument from
// the message), so rpcError sees exactly what it would in production.
func rpcErr(code int, msg string) error {
	return tgerr.New(code, msg)
}

func TestRPCErrMessageNotModified(t *testing.T) {
	e := rpcError(rpcErr(400, "MESSAGE_NOT_MODIFIED"))
	want := "Bad Request: message is not modified: specified new message content and reply markup are exactly the same as a current content and reply markup of the message"
	assertErr(t, e, 400, want)
}

func TestRPCErrUserIsBlocked(t *testing.T) {
	// Reclassified 400 → 403, "Forbidden:" prefix preserved verbatim.
	e := rpcError(rpcErr(400, "USER_IS_BLOCKED"))
	assertErr(t, e, 403, "Forbidden: bot was blocked by the user")
}

func TestRPCErrInputUserDeactivated(t *testing.T) {
	e := rpcError(rpcErr(400, "INPUT_USER_DEACTIVATED"))
	assertErr(t, e, 403, "Forbidden: user is deactivated")
}

func TestRPCErrUserAdminInvalid(t *testing.T) {
	e := rpcError(rpcErr(400, "USER_ADMIN_INVALID"))
	assertErr(t, e, 400, "Bad Request: user is an administrator of the chat")
}

func TestRPCErrFileGenerationFailed(t *testing.T) {
	// Telegram returns the sentence form, not ALL_CAPS. Previously this case was
	// dead ("FILE_GENERATE_FAILED" never matched) and leaked raw text.
	e := rpcError(rpcErr(400, "File generation failed"))
	assertErr(t, e, 400, "Bad Request: can't upload file by URL")
}

func TestRPCErrFloodWait(t *testing.T) {
	e := rpcError(rpcErr(429, "FLOOD_WAIT_30"))
	assertErr(t, e, 429, "Too Many Requests: retry after 30")
	if e.Params == nil || e.Params.RetryAfter != 30 {
		t.Fatalf("RetryAfter = %v, want 30", retryAfterOf(e))
	}
}

func TestRPCErrFloodWaitNoArg(t *testing.T) {
	// FLOOD_WAIT_ with no parseable number: the reference falls back to a raw 500
	// (Client.cpp:78-79), not a 429 envelope.
	e := rpcError(rpcErr(429, "FLOOD_WAIT_"))
	assertErr(t, e, 500, "FLOOD_WAIT_")
}

func TestRPCErrRankDefaultIsCustomTitle(t *testing.T) {
	// Without method context, RANK_* rewrites to CUSTOM_TITLE_*.
	e := rpcError(rpcErr(400, "RANK_CHANGE_FORBIDDEN"))
	assertErr(t, e, 400, "Bad Request: CUSTOM_TITLE_CHANGE_FORBIDDEN")
}

func TestRPCErrRankWithSetChatMemberTagIsTag(t *testing.T) {
	// setChatMemberTag rewrites RANK_* to TAG_* (method-aware).
	e := rpcErrorWith(rpcErr(400, "RANK_CHANGE_FORBIDDEN"), "", "setchatmembertag")
	assertErr(t, e, 400, "Bad Request: TAG_CHANGE_FORBIDDEN")
}

func TestRPCErrChatNotFound(t *testing.T) {
	// Cold-cache sends to an invalid/uncached chat must synthesise "chat not
	// found" (Client.cpp:7151 default_message via the getChat pre-resolution),
	// not leak the raw MTProto error type.
	for _, msg := range []string{"PEER_ID_INVALID", "CHAT_ID_INVALID", "USER_ID_INVALID", "USERNAME_NOT_OCCUPIED"} {
		assertErr(t, rpcErrorWithChat(rpcErr(400, msg), "group"), 400, "Bad Request: chat not found")
	}
	// The existing friendly arms are unchanged.
	assertErr(t, rpcErrorWithChat(rpcErr(400, "CHANNEL_PRIVATE"), "channel"), 403, "Forbidden: bot is not a member of the channel chat")
	assertErr(t, rpcErrorWithChat(rpcErr(400, "CHAT_WRITE_FORBIDDEN"), "group"), 403, "Forbidden: bot is not a member of the group chat")
}

func TestRPCErrSendMessageTranslations(t *testing.T) {
	// TDLib send-message translations (process_send_message_fail_error). MTProto
	// returns these as ALL_CAPS 400s; the reference (TDLib→bot-api) renders them
	// as human "Bad Request: …" strings. Verify each maps byte-for-byte.
	cases := []struct {
		raw, want string
	}{
		{"MESSAGE_TOO_LONG", "Bad Request: message is too long"},
		{"MEDIA_CAPTION_TOO_LONG", "Bad Request: message caption is too long"},
		{"REPLY_TO_MONOFORUM_PEER_INVALID", "Bad Request: wrong direct messages topic identifier specified"},
		{"CHAT_FORWARDS_RESTRICTED", "Bad Request: message has protected content and can't be forwarded"},
		{"EXTENDED_MEDIA_INVALID", "Bad Request: invalid paid media file specified"},
		{"PHOTO_EXT_INVALID", "Bad Request: photo has unsupported extension. Use one of .jpg, .jpeg, .gif, .png, .tif or .bmp"},
	}
	for _, tc := range cases {
		e := rpcError(rpcErr(400, tc.raw))
		assertErr(t, e, 400, tc.want)
	}
}

func TestRPCErrUserPermissionDeniedEmojiStatus(t *testing.T) {
	// setUserEmojiStatus (bots.updateUserEmojiStatus) → 403 human string
	// (TDLib UserManager.cpp:1065). Method-scoped: other callers leak the raw type.
	e := rpcErrorWith(rpcErr(400, "USER_PERMISSION_DENIED"), "", "setuseremojistatus")
	assertErr(t, e, 403, "Forbidden: not enough rights to change the user's emoji status")
	e = rpcError(rpcErr(400, "USER_PERMISSION_DENIED"))
	assertErr(t, e, 400, "Bad Request: USER_PERMISSION_DENIED")
}

func TestRPCErrChatAccessFallback(t *testing.T) {
	// Cold-start chat-access fallback (post-RPC, when the pre-flight had no cache).
	assertErr(t, rpcErrorWithChat(rpcErr(400, "CHANNEL_PRIVATE"), "channel"), 403, "Forbidden: bot is not a member of the channel chat")
	assertErr(t, rpcErrorWithChat(rpcErr(400, "CHANNEL_PRIVATE"), "supergroup"), 403, "Forbidden: bot is not a member of the supergroup chat")
	assertErr(t, rpcErrorWithChat(rpcErr(400, "CHAT_WRITE_FORBIDDEN"), "group"), 403, "Forbidden: bot is not a member of the group chat")
	// Non-chat-access error falls through to rpcError unchanged.
	assertErr(t, rpcErrorWithChat(rpcErr(400, "MESSAGE_TOO_LONG"), "channel"), 400, "Bad Request: message is too long")
}

func TestRPCErrDefaultMessage(t *testing.T) {
	// rpcErrorDefault overrides the raw message with a method-specific
	// description when the code resolves to 400.
	e := rpcErrorDefault(rpcErr(400, "SOME_RANDOM_ERROR"), "chat not found")
	assertErr(t, e, 400, "Bad Request: chat not found")
}

func TestRPCErrDefaultMessageDoesNotOverrideNon400(t *testing.T) {
	// A 403 (server-style ALL_CAPS) is reclassified to 400, so the default
	// message applies; a genuine 500 keeps its code and ignores the default.
	e := rpcErrorDefault(rpcErr(500, "INTERNAL"), "should not apply")
	assertErr(t, e, 500, "Internal Server Error: INTERNAL")
}

func TestRPCErrReclassify403AllCaps(t *testing.T) {
	// A 403 carrying an ALL_CAPS message is really a bad request.
	e := rpcError(rpcErr(403, "CHAT_ADMIN_REQUIRED"))
	assertErr(t, e, 400, "Bad Request: CHAT_ADMIN_REQUIRED")
}

func TestRPCErrUnknownCodeCollapsesTo400(t *testing.T) {
	// Codes outside {400,401,403,500,429} collapse to 400 with a raw prefix,
	// matching the reference's default arm.
	e := rpcError(rpcErr(420, "SOME_ERROR"))
	assertErr(t, e, 400, "Bad Request: SOME_ERROR")
}

func TestRPCErrNonTGError(t *testing.T) {
	e := rpcError(errors.New("boom"))
	assertErr(t, e, 400, "Bad Request: boom")
}

func TestRPCErrEmptyMessageIsServerError(t *testing.T) {
	// isServerError must return true for an EMPTY message (Client.cpp:90-98:
	// the loop never executes, is_server_error stays true). So an empty 403 is
	// reclassified to 400, and buildDescription("Bad Request","") → "Bad Request".
	e := rpcError(rpcErr(403, ""))
	assertErr(t, e, 400, "Bad Request")
}

func TestRPCErrNonServerErrorStays403(t *testing.T) {
	// A mixed-case 403 message is NOT a server error → stays 403. Use a message
	// that does NOT start with "Forbidden" so buildDescription prepends the prefix
	// (a prefix-aligned message would short-circuit and hide the isServerError path).
	e := rpcError(rpcErr(403, "some denial"))
	assertErr(t, e, 403, "Forbidden: some denial")
}

func TestRPCErrNil(t *testing.T) {
	if e := rpcError(nil); e != nil {
		t.Fatalf("expected nil, got %+v", e)
	}
}

func TestRPCErrContextCanceled(t *testing.T) {
	e := rpcError(context.Canceled)
	assertErr(t, e, 503, "Service Unavailable")
}

func TestRPCErrContextDeadlineExceeded(t *testing.T) {
	e := rpcError(context.DeadlineExceeded)
	assertErr(t, e, 503, "Service Unavailable")
}

// assertErr checks the Code and Description of a mapped error.
func assertErr(t *testing.T, e *Error, wantCode int, wantDesc string) {
	t.Helper()
	if e == nil {
		t.Fatalf("expected error, got nil")
	}
	if e.Code != wantCode {
		t.Errorf("Code = %d, want %d", e.Code, wantCode)
	}
	if e.Description != wantDesc {
		t.Errorf("Description = %q\nwant %q", e.Description, wantDesc)
	}
}

func retryAfterOf(e *Error) any {
	if e == nil || e.Params == nil {
		return nil
	}
	return e.Params.RetryAfter
}
