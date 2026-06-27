package client

import (
	"context"
	"errors"

	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

// Error is a Bot API error carrying the HTTP/error_code, description, and
// optional parameters. Handlers return *Error to drive the response envelope;
// the dispatch layer (Dispatch) translates it via response.Fail.
type Error struct {
	Code        int    // HTTP status AND Telegram error_code
	Description string // "Bad Request: …" / "Forbidden: …"
	Params      *response.Parameters
}

func (e *Error) Error() string { return e.Description }

// NewError builds a Bot API error.
func NewError(code int, description string) *Error {
	return &Error{Code: code, Description: description}
}

type Success struct {
	Result      any
	Description string
}

func NewSuccess(result any, description string) Success {
	return Success{Result: result, Description: description}
}

// HandlerFunc is the signature of a Bot API method handler bound to a Client,
// mirroring the pointer-to-member-function registration in Client.cpp
// (methods_.emplace("getme", &Client::process_get_me_query)). It returns the
// result object (any JSON-serialisable value) and/or a *Error. A non-*Error
// error is treated as an internal server error (HTTP 500).
type HandlerFunc func(c *Client, ctx context.Context, q *server.Query) (result any, err error)

// methods maps lowercased method names to client-bound handlers. Mirrors the
// methods_.emplace(...) table in telegram-bot-api/Client.cpp (~line 217).
// Populated incrementally as methods are implemented.
var methods = map[string]HandlerFunc{}

// Register adds a method handler (used by per-method files via init()).
func Register(name string, h HandlerFunc) {
	methods[name] = h
}

// Dispatch resolves a method name and invokes its handler, returning the HTTP
// status code and the JSON envelope body to write. Mirrors Client::send().
// Handlers that need the live connection call c.ensureConnected themselves, so
// unknown-method resolution never touches the network.
func (c *Client) Dispatch(ctx context.Context, q *server.Query) (status int, body []byte) {
	// fail_query_closing (Client.cpp:13315-13317): when the bot is closing or
	// logging out, every method returns the closing envelope. Checked before
	// method resolution, exactly as in on_cmd, so even unknown methods surface
	// the closing error rather than 404.
	if code, desc, retryAfter, closing := c.closingError(); closing {
		var params *response.Parameters
		if retryAfter > 0 {
			params = &response.Parameters{RetryAfter: retryAfter}
		}
		return code, response.Fail(code, desc, params)
	}
	h, ok := methods[q.Method]
	if !ok {
		// Matches the live official API: returns "Not Found" (Client.cpp:13324
		// has "Not Found: method not found" but the deployed API returns "Not Found").
		return 404, response.Fail(404, "Not Found", nil)
	}
	// Upload flood-limit gate (Client.cpp:13327-13350). Applied after method
	// resolution, exactly as in on_cmd, so unknown methods 404 before any
	// throttling state is touched.
	if status, body, proceed := c.applyUploadFloodLimit(ctx, q); !proceed {
		return status, body
	}
	// Resolve @username peer args (chat_id/from_chat_id/sender_chat_id) live via
	// contacts.resolveUsername and rewrite them to numeric chat_ids, so methods
	// work with @username targets the bot has never interacted with — mirroring
	// the official server's TDLib resolution. Per-client (multi-bot safe).
	c.warmUsernamePeers(ctx, q)
	result, err := h(c, ctx, q)
	if err == nil {
		if success, ok := result.(Success); ok {
			return 200, response.OK(success.Result, success.Description)
		}
		return 200, response.OK(result, "")
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr.Code, response.Fail(apiErr.Code, apiErr.Description, apiErr.Params)
	}
	// Unknown error: never leak internals to the client.
	return 500, response.Fail(500, "Internal Server Error", nil)
}

// HasMethod reports whether a method is registered (useful for tests).
func HasMethod(name string) bool { _, ok := methods[name]; return ok }

// RegisteredMethods returns the sorted list of registered method names.
func RegisteredMethods() []string {
	out := make([]string, 0, len(methods))
	for k := range methods {
		out = append(out, k)
	}
	return out
}
