package client

import (
	"context"
	"errors"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func TestDispatchUnknownMethod(t *testing.T) {
	c := NewClient(Params{}, "123:abc")
	q := server.NewQuery()
	q.Method = "nonexistentmethod"

	status, body := c.Dispatch(context.Background(), q)
	if status != 404 {
		t.Fatalf("status = %d, want 404", status)
	}
	// The live official API returns the fixed string "Not Found" for unknown
	// methods (Client.cpp:13324 has "Not Found: method not found" but the
	// deployed API uses the shorter form).
	const wantDesc = `"description":"Not Found"`
	if !contains(body, `"ok":false`) || !contains(body, `"error_code":404`) {
		t.Fatalf("unexpected body: %s", body)
	}
	if !contains(body, wantDesc) {
		t.Fatalf("description mismatch: body=%s\nwant substring: %s", body, wantDesc)
	}
	if contains(body, "nonexistentmethod") {
		t.Fatalf("method name leaked into error description: %s", body)
	}
}

func TestDispatchRegisteredHandler(t *testing.T) {
	const name = "test_ping"
	Register(name, func(c *Client, ctx context.Context, q *server.Query) (any, error) {
		return map[string]string{"pong": "1"}, nil
	})
	defer delete(methods, name)

	c := NewClient(Params{}, "1:secret")
	q := server.NewQuery()
	q.Method = name

	status, body := c.Dispatch(context.Background(), q)
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if !contains(body, `"ok":true`) || !contains(body, `"pong":"1"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestDispatchAPIError(t *testing.T) {
	const name = "test_fail"
	Register(name, func(c *Client, ctx context.Context, q *server.Query) (any, error) {
		return nil, NewError(403, "Forbidden: nope")
	})
	defer delete(methods, name)

	c := NewClient(Params{}, "1:secret")
	q := server.NewQuery()
	q.Method = name

	status, body := c.Dispatch(context.Background(), q)
	if status != 403 {
		t.Fatalf("status = %d, want 403", status)
	}
	if !contains(body, `"error_code":403`) || !contains(body, "Forbidden: nope") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestDispatchGenericError(t *testing.T) {
	const name = "test_panic"
	Register(name, func(c *Client, ctx context.Context, q *server.Query) (any, error) {
		return nil, errBoom // generic non-*Error error → 500
	})
	defer delete(methods, name)

	c := NewClient(Params{}, "1:secret")
	q := server.NewQuery()
	q.Method = name

	status, body := c.Dispatch(context.Background(), q)
	if status != 500 {
		t.Fatalf("status = %d, want 500", status)
	}
	if contains(body, "boom details") {
		t.Fatalf("internal error leaked to client: %s", body)
	}
}

func TestTokenID(t *testing.T) {
	if got := tokenID("123456:ABC-DEF"); got != "123456" {
		t.Errorf("tokenID = %q, want 123456", got)
	}
	if got := tokenID("nocolon"); got != "nocolon" {
		t.Errorf("tokenID = %q, want nocolon", got)
	}
}

var errBoom = errors.New("boom details")

func contains(haystack []byte, needle string) bool {
	return string(haystack) != "" && index(string(haystack), needle) >= 0
}

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
