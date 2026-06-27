package client

import (
	"context"
	"testing"
	"time"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
)

func TestComputeFlood(t *testing.T) {
	const total int64 = 200_000_000 // large total → minDelay clamps to 0.9s
	now := 1000.0

	cases := []struct {
		name        string
		last        float64
		wantOut     floodOutcome
		wantNewLast float64
	}{
		{
			name:        "fresh bucket — proceeds, bucket resets to now-1.0",
			last:        0,
			wantOut:     floodProceed,
			wantNewLast: now - 1.0,
		},
		{
			name:    "backlog over 5s — flood limited (bucket unchanged)",
			last:    now + 6.0,
			wantOut: floodLimited,
			// newLast returned but NOT stored by the caller for floodLimited
		},
		{
			name:        "small backlog under threshold — delay",
			last:        now + 0.5, // last+minDelay(0.9) = now+1.4 > now → delay 1.4s
			wantOut:     floodDelay,
			wantNewLast: now + 1.4,
		},
		{
			name:        "backlog exactly at threshold boundary (now+5) — not limited (strict >)",
			last:        now + 5.0,
			wantOut:     floodDelay,
			wantNewLast: now + 5.0 + 0.9, // last+minDelay
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, newLast, delay := computeFlood(total, tc.last, now)
			if out != tc.wantOut {
				t.Fatalf("outcome=%v want %v (delay=%v)", out, tc.wantOut, delay)
			}
			if out != floodLimited {
				if !approx(newLast, tc.wantNewLast, 1e-9) {
					t.Errorf("newLast=%v want %v", newLast, tc.wantNewLast)
				}
			}
			if out == floodDelay && delay <= 0 {
				t.Errorf("floodDelay but delay=%v", delay)
			}
			if out == floodProceed && delay != 0 {
				t.Errorf("floodProceed but delay=%v (must be 0)", delay)
			}
		})
	}
}

func TestComputeFlood_MinDelayClamp(t *testing.T) {
	// total just over threshold (100001 bytes) → total*1e-7 ≈ 0.01 → clamps to 0.2.
	last := 0.0
	now := 1000.0
	_, newLast, _ := computeFlood(100001, last, now)
	// fresh bucket → newLast = max(0+0.2, now-1.0) = now-1.0
	if !approx(newLast, now-1.0, 1e-9) {
		t.Errorf("clamped minDelay: newLast=%v want %v", newLast, now-1.0)
	}
}

func TestApplyUploadFloodLimit_SkipConditions(t *testing.T) {
	ctx := context.Background()

	// Local mode — never throttled.
	c := NewClient(Params{LocalMode: true}, "1:tok")
	q := server.NewQuery()
	q.Files["doc"] = server.File{FieldName: "doc", Size: 10_000_000}
	if _, _, proceed := c.applyUploadFloodLimit(ctx, q); !proceed {
		t.Fatal("local mode should proceed")
	}

	// No files — proceeds.
	c2 := NewClient(Params{}, "1:tok")
	q2 := server.NewQuery()
	if _, _, proceed := c2.applyUploadFloodLimit(ctx, q2); !proceed {
		t.Fatal("no files should proceed")
	}

	// Files under threshold — proceeds.
	c3 := NewClient(Params{}, "1:tok")
	q3 := server.NewQuery()
	q3.Files["doc"] = server.File{FieldName: "doc", Size: 50_000}
	if _, _, proceed := c3.applyUploadFloodLimit(ctx, q3); !proceed {
		t.Fatal("under-threshold files should proceed")
	}
}

func TestApplyUploadFloodLimit_LimitedReturns429(t *testing.T) {
	// Force the flood path by pre-seeding a bucket value beyond the 5s backlog.
	c := NewClient(Params{}, "1:tok")
	const total int64 = 1_000_000
	c.floodBuckets[total] = nowMonotonic() + 10.0 // far beyond now+5

	q := server.NewQuery()
	q.Files["doc"] = server.File{FieldName: "doc", Size: total}

	start := time.Now()
	status, body, proceed := c.applyUploadFloodLimit(context.Background(), q)
	elapsed := time.Since(start)

	if proceed {
		t.Fatal("expected proceed=false (flood limited)")
	}
	if status != 429 {
		t.Fatalf("status=%d want 429", status)
	}
	want := "Too Many Requests: retry after 60"
	if !contains(body, want) {
		t.Fatalf("body=%s missing %q", body, want)
	}
	if !contains(body, `"retry_after":60`) {
		t.Fatalf("body=%s missing retry_after:60", body)
	}
	// Must have slept ~3s (fail_query_flood_limit_exceeded SleepActor 3.0).
	if elapsed < 2*time.Second {
		t.Errorf("flood path returned too fast: %v (expected ~3s sleep)", elapsed)
	}
}

func approx(a, b, tol float64) bool {
	if a-b > tol || b-a > tol {
		return false
	}
	return true
}
