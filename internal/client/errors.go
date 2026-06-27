package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mtgo-labs/mtgo/tgerr"

	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
)

// connError maps a connection error (from ensureConnected) to a Bot API 502
// envelope. Used by handlers that require a live Telegram connection.
func connError(err error) *Error {
	return &Error{Code: 502, Description: "Bad Gateway: failed to connect to Telegram: " + err.Error()}
}

// rpcErrorWithChat adds chat-type-aware translations for the cold-start
// chat-access errors that the checkWriteAccess pre-flight can't catch (the chat
// was never cached, so its membership status is unknown). When the RPC surfaces
// CHANNEL_PRIVATE (channel/supergroup the bot can't access) or CHAT_WRITE_FORBIDDEN
// (basic group), translate to the friendly "bot is not a member of the <type> chat"
// the official server returns. chatType is the suffix hint: "channel" | "supergroup"
// | "group". Best-effort — without cached state it can't distinguish kicked/deleted/
// upgraded, and defaults "channel" when supergroup is indistinguishable.
func rpcErrorWithChat(err error, chatType string) *Error {
	if rpcErr, ok := tgerr.As(err); ok {
		switch rpcErr.Message {
		case "CHANNEL_PRIVATE":
			suffix := chatType
			if suffix != "supergroup" {
				suffix = "channel"
			}
			return NewError(403, "Forbidden: bot is not a member of the "+suffix+" chat")
		case "CHAT_WRITE_FORBIDDEN":
			return NewError(403, "Forbidden: bot is not a member of the group chat")
		case "PEER_ID_INVALID", "CHAT_ID_INVALID", "USER_ID_INVALID", "USERNAME_INVALID", "USERNAME_NOT_OCCUPIED":
			// The official synthesises "chat not found" via the getChat
			// pre-resolution (Requests.cpp:347 "Chat is not accessible" +
			// Client.cpp:7151 default_message), so the raw MTProto type never
			// surfaces. Reproduce that for cold-cache sends.
			return NewError(400, "Bad Request: chat not found")
		}
	}
	return rpcError(err)
}

// rpcError maps a tgerr.Error (from the mtgo RPC layer) to a Bot API *Error
// with the correct HTTP status code and human-readable description.
//
// This mirrors Client::fail_query_with_error in telegram-bot-api/Client.cpp
// (lines 71–205). It is the default entry point used by the 170+ handlers that
// have no method-specific error wording.
func rpcError(err error) *Error {
	return rpcErrorWith(err, "", "")
}

// rpcErrorDefault is rpcError with a method-specific default description. When
// the error resolves to HTTP 400, defaultMsg overrides the raw MTProto message
// before translation — exactly mirroring fail_query_with_error's default_message
// parameter (Client.cpp lines 104–107). This is how handlers attach the
// user-facing "Bad Request: …" strings the official server emits.
func rpcErrorDefault(err error, defaultMsg string) *Error {
	return rpcErrorWith(err, defaultMsg, "")
}

// rpcErrorWith is the full form, accepting an optional default message and the
// Bot API method name. The method is used for the few method-aware translations
// (currently RANK_* → TAG_* for setChatMemberTag vs CUSTOM_TITLE_* elsewhere;
// Client.cpp lines 152–161).
func rpcErrorWith(err error, defaultMsg, method string) *Error {
	if err == nil {
		return nil
	}

	// Context cancellation/deadline → 503, not 400. The official server never
	// exposes request-cancellation as a client error; returning 400 "context
	// canceled" would leak Go internals and misclassify a transient failure.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return NewError(503, "Service Unavailable")
	}

	rpcErr, ok := tgerr.As(err)
	if !ok {
		return NewError(400, "Bad Request: "+err.Error())
	}

	code := rpcErr.Code
	msg := rpcErr.Message

	// 429 Too Many Requests — extract retry_after from FLOOD_WAIT_N
	// (Client.cpp lines 73–80). When retry_after is present, emit the 429
	// retry_after envelope. When it cannot be parsed, the reference falls back
	// to fail_query(500, error_message) — a raw 500 with no prefix building.
	if code == 429 {
		if rpcErr.Argument > 0 {
			return &Error{
				Code:        429,
				Description: fmt.Sprintf("Too Many Requests: retry after %d", rpcErr.Argument),
				Params:      &response.Parameters{RetryAfter: rpcErr.Argument},
			}
		}
		// Client.cpp:78-79 — malformed/unparseable 429 → 500 raw message.
		return NewError(500, msg)
	}

	// Normalise codes outside the valid HTTP range to 400 (Client.cpp lines
	// 83–88). A 403 carrying an ALL_CAPS server-style message is really a bad
	// request and is reclassified to 400 (lines 89–102).
	if code < 400 || code == 404 {
		code = 400
	} else if code == 403 {
		if isServerError(msg) {
			code = 400
		}
	}

	// Translate known 400 messages (Client.cpp lines 104–162). defaultMsg, when
	// set, overrides the raw message first — this lets a handler supply the
	// method-specific description the reference passes as default_message.
	if code == 400 {
		if defaultMsg != "" {
			msg = defaultMsg
		}
		msg, code = translateErrorMessage(msg, code, method)
	}

	// Select the prefix by code (Client.cpp lines 163–185). Codes outside the
	// known set collapse to 400 with a raw "Bad Request: <message>", matching
	// the reference's default arm.
	switch code {
	case 400, 401, 403, 500:
		return NewError(code, buildDescription(codePrefix(code), msg))
	default:
		return NewError(400, "Bad Request: "+msg)
	}
}

// isServerError reports whether the error message looks like a raw server
// error (ALL_CAPS with underscores and digits), which should be reclassified
// from 403 to 400. Mirrors Client.cpp:90-98 exactly — including the empty-
// message case, where the reference's loop never executes and leaves
// is_server_error=true (so an empty 403 message is also reclassified to 400).
func isServerError(msg string) bool {
	isServerError := true
	for _, c := range msg {
		if c == '_' || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		isServerError = false
		break
	}
	return isServerError
}

// translateErrorMessage maps known Telegram error message strings to
// human-readable Bot API descriptions, adjusting the error code where the
// reference does. It mirrors the if/else chain in Client.cpp lines 108–161 and
// matches on the message (the raw MTProto string), as the reference does.
func translateErrorMessage(msg string, code int, method string) (string, int) {
	switch msg {
	case "MESSAGE_NOT_MODIFIED":
		return "message is not modified: specified new message content and reply markup are exactly the same as a current content and reply markup of the message", code
	case "WC_CONVERT_URL_INVALID", "EXTERNAL_URL_INVALID":
		return "Wrong HTTP URL specified", code
	case "WEBPAGE_CURL_FAILED":
		return "Failed to get HTTP URL content", code
	case "WEBPAGE_MEDIA_EMPTY":
		return "Wrong type of the web page content", code
	case "MEDIA_GROUPED_INVALID":
		return "Can't use the media of the specified type in the album", code
	case "REPLY_MARKUP_TOO_LONG":
		return "reply markup is too long", code
	case "INPUT_USER_DEACTIVATED":
		return "Forbidden: user is deactivated", 403
	case "USER_IS_BLOCKED":
		return "bot was blocked by the user", 403
	case "USER_ADMIN_INVALID":
		return "user is an administrator of the chat", code
	case "File generation failed", "FILE_GENERATE_FAILED":
		// Telegram returns the sentence form "File generation failed" (mixed
		// case), not an ALL_CAPS type — see Client.cpp line 131.
		return "can't upload file by URL", code
	case "CHAT_ABOUT_NOT_MODIFIED":
		return "chat description is not modified", code
	case "PACK_SHORT_NAME_INVALID":
		return "invalid sticker set name is specified", code
	case "PACK_SHORT_NAME_OCCUPIED":
		return "sticker set name is already occupied", code
	case "STICKER_EMOJI_INVALID":
		return "invalid sticker emojis", code
	case "QUERY_ID_INVALID":
		return "query is too old and response timeout expired or query ID is invalid", code
	case "MESSAGE_DELETE_FORBIDDEN":
		return "message can't be deleted", code
	// TDLib send-message translations (MessagesManager.cpp
	// process_send_message_fail_error, case 400). These arrive from MTProto as
	// ALL_CAPS 400 errors; TDLib converts them to human strings before bot-api
	// wraps them, so without these entries we leak the raw type. buildDescription
	// lowercases the first letter, yielding the reference's "Bad Request: …".
	case "MESSAGE_TOO_LONG":
		return "Message is too long", code
	case "MEDIA_CAPTION_TOO_LONG":
		return "Message caption is too long", code
	case "REPLY_TO_MONOFORUM_PEER_INVALID":
		return "Wrong direct messages topic identifier specified", code
	case "CHAT_FORWARDS_RESTRICTED":
		return "Message has protected content and can't be forwarded", code
	case "EXTENDED_MEDIA_INVALID":
		return "Invalid paid media file specified", code
	case "PHOTO_EXT_INVALID":
		return "Photo has unsupported extension. Use one of .jpg, .jpeg, .gif, .png, .tif or .bmp", code
	case "USER_PERMISSION_DENIED":
		// setUserEmojiStatus (bots.updateUserEmojiStatus) → TDLib UserManager.cpp:1065.
		// Method-scoped: other callers of this generic MTProto error must leak the raw type.
		if method == "setuseremojistatus" {
			return "Not enough rights to change the user's emoji status", 403
		}
	case "RANK_CHANGE_FORBIDDEN", "RANK_EMOJI_NOT_ALLOWED", "RANK_INVALID":
		// Rewrite RANK_* to TAG_* / CUSTOM_TITLE_* based on the calling method
		// (Client.cpp lines 152–161): setChatMemberTag → "TAG"+suffix, every
		// other caller → "CUSTOM_TITLE"+suffix. msg[4:] drops the "RANK".
		prefix := "CUSTOM_TITLE"
		if method == "setchatmembertag" {
			prefix = "TAG"
		}
		if len(msg) > 4 {
			return prefix + msg[4:], code
		}
		return prefix, code
	case "USER_ID_INVALID":
		return "invalid user_id specified", code
	case "CONNECTION_ID_INVALID":
		return "business connection not found", code
	}
	return msg, code
}

// codePrefix returns the standard Bot API error prefix for the given HTTP
// error code, matching Client.cpp lines 164–184.
func codePrefix(code int) string {
	switch code {
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 500:
		return "Internal Server Error"
	default:
		return "Bad Request"
	}
}

// buildDescription constructs the final "Prefix: message" string, applying
// the casing rule from Client.cpp lines 187–203: if the message already starts
// with the prefix, use it as-is; otherwise lowercase the first letter of the
// message and prepend the prefix (unless the message is an ALL_CAPS type
// string, which is preserved verbatim).
func buildDescription(prefix, msg string) string {
	if msg == "" {
		return prefix
	}
	if strings.HasPrefix(msg, prefix) {
		return msg
	}
	// If the second character is _ or uppercase, the message is an ALL_CAPS
	// type string — keep it as-is.
	if len(msg) >= 2 && (msg[1] == '_' || (msg[1] >= 'A' && msg[1] <= 'Z')) {
		return prefix + ": " + msg
	}
	// Lowercase the first character.
	lowered := strings.ToLower(msg[:1]) + msg[1:]
	return prefix + ": " + lowered
}
