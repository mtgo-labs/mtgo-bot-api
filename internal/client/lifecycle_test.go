package client

import (
	"testing"
	"time"
)

func TestCloseGateDecision(t *testing.T) {
	serverStart := time.Unix(1_700_000_000, 0)

	cases := []struct {
		name      string
		clientAge time.Duration // age of client at "now"
		created   time.Duration // how long after serverStart the client was created
		wantLimit bool
		// wantRetryFloor/wantRetryCeil bracket the retry_after when limited.
	}{
		{
			name:    "client created immediately after boot, young — NOT limited (created clause fails)",
			created: 0, clientAge: 30 * time.Second,
			wantLimit: false,
		},
		{
			name:    "client created >10min after boot, young (<10min) — LIMITED",
			created: 15 * time.Minute, clientAge: 2 * time.Minute,
			wantLimit: true,
		},
		{
			name:    "client created >10min after boot, aged >=10min — NOT limited",
			created: 15 * time.Minute, clientAge: 11 * time.Minute,
			wantLimit: false,
		},
		{
			name:    "client created exactly at 10min boundary — NOT limited (After is strict)",
			created: 10 * time.Minute, clientAge: 1 * time.Minute,
			wantLimit: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := serverStart.Add(tc.created)
			now := startTime.Add(tc.clientAge)
			limited, retry := closeGateDecision(startTime, serverStart, now)
			if limited != tc.wantLimit {
				t.Fatalf("limited=%v want %v (retry=%d)", limited, tc.wantLimit, retry)
			}
			if limited {
				want := int((closeWindow - tc.clientAge).Seconds())
				if retry != want {
					t.Errorf("retry_after=%d want %d", retry, want)
				}
				// retry_after must be positive and within (0, 600].
				if retry <= 0 || retry > 600 {
					t.Errorf("retry_after out of range: %d", retry)
				}
			}
		})
	}
}
