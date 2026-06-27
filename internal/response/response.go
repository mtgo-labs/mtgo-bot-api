// Package response implements the Bot API JSON response envelope
// {ok, result, error_code, description, parameters}, mirroring the
// JsonQueryOk/JsonQueryError builders in telegram-bot-api/Query.h.
package response

import (
	"bytes"
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// Parameters is the optional "parameters" object in error responses
// (Bot API type ResponseParameters).
type Parameters struct {
	MigrateToChatID int64 `json:"migrate_to_chat_id,omitempty"`
	RetryAfter      int   `json:"retry_after,omitempty"`
}

// successEnvelope is {ok:true, result, [description]}. Result is ALWAYS
// present on success (no omitempty), matching the official server.
type successEnvelope struct {
	Ok          bool   `json:"ok"`
	Result      any    `json:"result"`
	Description string `json:"description,omitempty"`
}

// errorEnvelope is {ok:false, error_code, description, [parameters]}.
type errorEnvelope struct {
	Ok          bool        `json:"ok"`
	ErrorCode   int         `json:"error_code"`
	Description string      `json:"description"`
	Parameters  *Parameters `json:"parameters,omitempty"`
}

// OK builds a success envelope: {"ok":true,"result":<result>[,"description":"..."]}.
func OK(result any, description string) []byte {
	b, err := json.Marshal(successEnvelope{Ok: true, Result: result, Description: description})
	if err != nil {
		// Should never happen for a JSON-serialisable result; fall back to a
		// minimal valid envelope so we never emit invalid JSON.
		return []byte(`{"ok":true,"result":null}`)
	}
	return escapeNonASCII(b)
}

// Fail builds an error envelope: {"ok":false,"error_code":<code>,"description":"..."[,"parameters":{...}]}.
func Fail(code int, description string, params *Parameters) []byte {
	b, err := json.Marshal(errorEnvelope{Ok: false, ErrorCode: code, Description: description, Parameters: params})
	if err != nil {
		return []byte(`{"ok":false,"error_code":500,"description":"Internal Server Error"}`)
	}
	return escapeNonASCII(b)
}

// Decode unmarshals an envelope (useful for tests).
func Decode(b []byte) (ok bool, result json.RawMessage, errorCode int, description string, err error) {
	var raw struct {
		Ok          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
	}
	if err = json.Unmarshal(b, &raw); err != nil {
		return false, nil, 0, "", err
	}
	return raw.Ok, raw.Result, raw.ErrorCode, raw.Description, nil
}

// escapeNonASCII replaces non-ASCII bytes within JSON string values with their
// \uXXXX representation, matching the official Bot API's C++ JSON serializer
// (td::json_encode with default escaping). It preserves structural characters
// and existing escape sequences (including Go's built-in \u003c for '<', etc.).
func escapeNonASCII(data []byte) []byte {
	var buf bytes.Buffer
	buf.Grow(len(data))
	inString := false
	for i := 0; i < len(data); {
		b := data[i]
		if !inString {
			if b == '"' {
				inString = true
			}
			buf.WriteByte(b)
			i++
			continue
		}
		// Inside a JSON string value.
		switch {
		case b == '\\':
			// Copy the escape sequence verbatim (backslash + next byte).
			buf.WriteByte(b)
			i++
			if i < len(data) {
				buf.WriteByte(data[i])
				i++
			}
		case b == '"':
			inString = false
			buf.WriteByte(b)
			i++
		case b < 0x80:
			buf.WriteByte(b)
			i++
		default:
			// Non-ASCII: decode the UTF-8 rune and emit \uXXXX.
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 {
				fmt.Fprintf(&buf, `\ufffd`)
				i++
			} else if r <= 0xFFFF {
				fmt.Fprintf(&buf, `\u%04x`, r)
				i += size
			} else {
				// Supplementary plane: emit a UTF-16 surrogate pair.
				r -= 0x10000
				fmt.Fprintf(&buf, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
				i += size
			}
		}
	}
	return buf.Bytes()
}
