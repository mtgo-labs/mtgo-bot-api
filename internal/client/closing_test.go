package client

import (
	"context"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func TestClosingError_States(t *testing.T) {
	cases := []struct {
		name           string
		setClosing     bool
		setLoggingOut  bool
		setLoggedOut   bool
		wantOk         bool
		wantCode       int
		wantDescSubstr string
	}{
		{name: "live", wantOk: false},
		{name: "closing", setClosing: true, wantOk: true, wantCode: 500, wantDescSubstr: "Internal Server Error: restart"},
		{name: "logging out in flight", setLoggingOut: true, wantOk: true, wantCode: 401, wantDescSubstr: "Unauthorized"},
		{name: "logged out (queue cleared)", setLoggingOut: true, setLoggedOut: true, wantOk: true, wantCode: 400, wantDescSubstr: "Logged out"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient(Params{}, "1:tok")
			if tc.setClosing {
				c.closing.Store(true)
			}
			if tc.setLoggingOut {
				c.loggingOut.Store(true)
			}
			if tc.setLoggedOut {
				c.loggedOut.Store(true)
			}
			code, desc, retryAfter, ok := c.closingError()
			if ok != tc.wantOk {
				t.Fatalf("ok=%v want %v", ok, tc.wantOk)
			}
			if !ok {
				return
			}
			if code != tc.wantCode {
				t.Errorf("code=%d want %d", code, tc.wantCode)
			}
			if desc != tc.wantDescSubstr && !contains([]byte(desc), tc.wantDescSubstr) {
				t.Errorf("desc=%q want substring %q", desc, tc.wantDescSubstr)
			}
			_ = retryAfter
		})
	}
}

func TestDispatch_ClosingShortCircuits(t *testing.T) {
	// A closing client returns 500 "restart" for ANY method, including unknown
	// ones, before method resolution (Client.cpp:13315-13317 precedes 13322).
	c := NewClient(Params{}, "1:tok")
	c.closing.Store(true)

	q := server.NewQuery()
	q.Method = "getme" // a registered method, but closing must override it

	status, body := c.Dispatch(context.Background(), q)
	if status != 500 {
		t.Fatalf("status=%d want 500", status)
	}
	if !contains(body, "Internal Server Error: restart") {
		t.Fatalf("body=%s missing 'Internal Server Error: restart'", body)
	}
}

func TestDispatch_ClosingOverridesUnknownMethod(t *testing.T) {
	c := NewClient(Params{}, "1:tok")
	c.closing.Store(true)

	q := server.NewQuery()
	q.Method = "totallyBogus"

	status, body := c.Dispatch(context.Background(), q)
	// Closing wins over the 404 unknown-method path.
	if status != 500 {
		t.Fatalf("status=%d want 500 (closing must precede 404)", status)
	}
	if contains(body, "method not found") {
		t.Fatalf("closing did not override 404: %s", body)
	}
}
