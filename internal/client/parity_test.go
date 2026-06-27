package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/response"
	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

// parity_test.go validates that Bot API response envelopes are byte-for-byte
// compatible with the official telegram-bot-api server's JSON output.
//
// Reference format (Query.h — JsonQueryOk / JsonQueryError):
//   success: {"ok":true,"result":<value>[,"description":"..."]}
//   error:   {"ok":false,"error_code":<code>,"description":"..."[,"parameters":{...}]}
//
// The official server ALWAYS includes "result" on success (never omitted),
// uses snake_case field names, and serialises booleans as true/false (not 1/0).

// ---------------------------------------------------------------------------
// Golden success envelope tests — exact byte output
// ---------------------------------------------------------------------------

func TestParitySuccessGoldenBytes(t *testing.T) {
	tests := []struct {
		name     string
		result   any
		desc     string
		expected string
	}{
		{"bool_true", true, "", `{"ok":true,"result":true}`},
		{"bool_false", false, "", `{"ok":true,"result":false}`},
		{"string", "hello", "", `{"ok":true,"result":"hello"}`},
		{"empty_string", "", "", `{"ok":true,"result":""}`},
		{"int", 42, "", `{"ok":true,"result":42}`},
		{"nil_result", nil, "", `{"ok":true,"result":null}`},
		{"with_description", "ok", "custom desc", `{"ok":true,"result":"ok","description":"custom desc"}`},
		{
			"nested_object",
			map[string]any{"id": 123, "name": "test"},
			"",
			`{"ok":true,"result":{"id":123,"name":"test"}}`,
		},
		{"array_result", []int{1, 2, 3}, "", `{"ok":true,"result":[1,2,3]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(response.OK(tt.result, tt.desc))
			if got != tt.expected {
				t.Errorf("OK(%v, %q)\n  got:  %s\n  want: %s", tt.result, tt.desc, got, tt.expected)
			}
		})
	}
}

func TestParityResultAlwaysPresent(t *testing.T) {
	// Official parity: "result" key is ALWAYS present on success, even when nil.
	// Our successEnvelope uses `json:"result"` (no omitempty).
	body := response.OK(nil, "")
	var env map[string]json.RawMessage
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := env["result"]; !ok {
		t.Fatal("result key must always be present on success")
	}
}

// ---------------------------------------------------------------------------
// Golden error envelope tests — exact byte output
// ---------------------------------------------------------------------------

func TestParityErrorGoldenBytes(t *testing.T) {
	tests := []struct {
		name        string
		code        int
		description string
		params      *response.Parameters
		expected    string
	}{
		{
			"bad_request", 400, "Bad Request: chat_id is empty", nil,
			`{"ok":false,"error_code":400,"description":"Bad Request: chat_id is empty"}`,
		},
		{
			"unauthorized", 401, "Unauthorized", nil,
			`{"ok":false,"error_code":401,"description":"Unauthorized"}`,
		},
		{
			"forbidden", 403, "Forbidden: bot was blocked by the user", nil,
			`{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`,
		},
		{
		"not_found", 404, "Not Found", nil,
		`{"ok":false,"error_code":404,"description":"Not Found"}`,
		},
		{
			"conflict", 409, "Conflict: terminated by other getUpdates request", nil,
			`{"ok":false,"error_code":409,"description":"Conflict: terminated by other getUpdates request"}`,
		},
		{
			"too_many_requests_retry", 429, "Too Many Requests: retry after 5",
			&response.Parameters{RetryAfter: 5},
			`{"ok":false,"error_code":429,"description":"Too Many Requests: retry after 5","parameters":{"retry_after":5}}`,
		},
		{
			"too_many_requests_migrate", 429, "Too Many Requests",
			&response.Parameters{MigrateToChatID: 123456789},
			`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"migrate_to_chat_id":123456789}}`,
		},
		{
			"internal_error", 500, "Internal Server Error", nil,
			`{"ok":false,"error_code":500,"description":"Internal Server Error"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(response.Fail(tt.code, tt.description, tt.params))
			if got != tt.expected {
				t.Errorf("Fail(%d, %q)\n  got:  %s\n  want: %s", tt.code, tt.description, got, tt.expected)
			}
		})
	}
}

func TestParityErrorNoExtraFields(t *testing.T) {
	// Error envelopes must NOT include "result" (official server omits it).
	body := response.Fail(400, "test", nil)
	var env map[string]json.RawMessage
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := env["result"]; ok {
		t.Error("error envelope must not contain 'result' key")
	}
	if _, ok := env["parameters"]; ok {
		t.Error("error envelope without params must not contain 'parameters' key")
	}
}

// ---------------------------------------------------------------------------
// Field naming parity — snake_case matching Bot API spec
// ---------------------------------------------------------------------------

func TestParitySnakeCaseFieldNames(t *testing.T) {
	// Success envelope must use "ok" and "result" (not Ok/Result/ok_).
	body := response.OK(true, "")
	var env map[string]json.RawMessage
	_ = json.Unmarshal(body, &env)
	requiredSuccessKeys := []string{"ok", "result"}
	for _, k := range requiredSuccessKeys {
		if _, ok := env[k]; !ok {
			t.Errorf("success envelope missing key %q", k)
		}
	}

	// Error envelope must use "ok", "error_code", "description".
	errBody := response.Fail(400, "test", nil)
	_ = json.Unmarshal(errBody, &env)
	requiredErrorKeys := []string{"ok", "error_code", "description"}
	for _, k := range requiredErrorKeys {
		if _, ok := env[k]; !ok {
			t.Errorf("error envelope missing key %q", k)
		}
	}
}

// ---------------------------------------------------------------------------
// Decode round-trip
// ---------------------------------------------------------------------------

func TestParityDecodeSuccess(t *testing.T) {
	original := response.OK(map[string]int{"answer": 42}, "")
	ok, result, errorCode, _, err := response.Decode(original)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !ok {
		t.Error("ok should be true")
	}
	if errorCode != 0 {
		t.Errorf("errorCode = %d, want 0", errorCode)
	}
	if !contains(result, `"answer":42`) {
		t.Errorf("result missing expected content: %s", result)
	}
}

func TestParityDecodeError(t *testing.T) {
	original := response.Fail(429, "rate limited", &response.Parameters{RetryAfter: 10})
	ok, _, errorCode, desc, err := response.Decode(original)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ok {
		t.Error("ok should be false for error")
	}
	if errorCode != 429 {
		t.Errorf("errorCode = %d, want 429", errorCode)
	}
	if desc != "rate limited" {
		t.Errorf("description = %q, want %q", desc, "rate limited")
	}
}

// ---------------------------------------------------------------------------
// Dispatch → envelope integration (end-to-end parity)
// ---------------------------------------------------------------------------

func TestParityDispatchSuccessEnvelope(t *testing.T) {
	const name = "parity_test_success"
	Register(name, func(c *Client, ctx context.Context, q *server.Query) (any, error) {
		return map[string]any{"message_id": 123, "date": 1700000000}, nil
	})
	defer delete(methods, name)

	c := NewClient(Params{}, "1:test")
	q := server.NewQuery()
	q.Method = name

	status, body := c.Dispatch(context.Background(), q)
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	// Must be a valid success envelope with the result embedded.
	ok, result, errorCode, _, err := response.Decode(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !ok || errorCode != 0 {
		t.Fatalf("expected success envelope, got ok=%v error_code=%d", ok, errorCode)
	}
	if !contains(result, `"message_id":123`) {
		t.Errorf("result missing message_id: %s", result)
	}
}

func TestParityDispatchErrorEnvelopeWithParams(t *testing.T) {
	const name = "parity_test_flood"
	Register(name, func(c *Client, ctx context.Context, q *server.Query) (any, error) {
		return nil, &Error{
			Code:        429,
			Description: "Too Many Requests: retry after 3",
			Params:      &response.Parameters{RetryAfter: 3},
		}
	})
	defer delete(methods, name)

	c := NewClient(Params{}, "1:test")
	q := server.NewQuery()
	q.Method = name

	status, body := c.Dispatch(context.Background(), q)
	if status != 429 {
		t.Fatalf("status = %d, want 429", status)
	}
	if !contains(body, `"ok":false`) {
		t.Errorf("missing ok:false: %s", body)
	}
	if !contains(body, `"error_code":429`) {
		t.Errorf("missing error_code:429: %s", body)
	}
	if !contains(body, `"retry_after":3`) {
		t.Errorf("missing retry_after:3: %s", body)
	}
}

func TestParityDispatchGenericErrorIs500(t *testing.T) {
	const name = "parity_test_internal"
	Register(name, func(c *Client, ctx context.Context, q *server.Query) (any, error) {
		return nil, errBoom // reuse from dispatch_test.go
	})
	defer delete(methods, name)

	c := NewClient(Params{}, "1:test")
	q := server.NewQuery()
	q.Method = name

	status, body := c.Dispatch(context.Background(), q)
	if status != 500 {
		t.Fatalf("status = %d, want 500", status)
	}
	// Internal error details must NOT leak.
	if contains(body, "boom") {
		t.Errorf("internal error leaked: %s", body)
	}
	if !contains(body, `"error_code":500`) {
		t.Errorf("missing error_code:500: %s", body)
	}
}

// ---------------------------------------------------------------------------
// Boolean serialization parity (must be true/false, not 1/0)
// ---------------------------------------------------------------------------

func TestParityBooleanSerialization(t *testing.T) {
	body := response.OK(true, "")
	if !contains(body, `"ok":true`) {
		t.Errorf("ok must serialize as true (not 1): %s", body)
	}

	errBody := response.Fail(400, "test", nil)
	if !contains(errBody, `"ok":false`) {
		t.Errorf("ok must serialize as false (not 0): %s", errBody)
	}
}
