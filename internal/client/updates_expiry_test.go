package client

import (
	"testing"
	"time"
)

// Regression (A2): update TTLs are per-type (Client.cpp add_update timeout table)
// and messages/reactions/member changes older than 24h are dropped, not enqueued.
func TestUpdateExpirySeconds(t *testing.T) {
	now := int32(time.Now().Unix())

	// Fresh message: ~24h TTL.
	ttl, ok := updateExpirySeconds(map[string]any{"message": map[string]any{"date": float64(now)}})
	if !ok || ttl < 86000 || ttl > 86400 {
		t.Errorf("fresh message: ttl=%d ok=%v, want ~86400/true", ttl, ok)
	}
	// Stale message (>24h old) must be dropped.
	if _, ok := updateExpirySeconds(map[string]any{"message": map[string]any{"date": float64(now - 25*3600)}}); ok {
		t.Error("stale message (>24h) should be dropped (ok=false)")
	}

	// Fixed-TTL types ignore the date.
	for typ, want := range map[string]int32{
		"callback_query":       150,
		"inline_query":         30,
		"chosen_inline_result": 600,
		"shipping_query":       150,
		"pre_checkout_query":   150,
		"poll":                 86400,
		"poll_answer":          86400,
		"business_connection":  86400,
	} {
		if ttl, ok := updateExpirySeconds(map[string]any{typ: map[string]any{}}); !ok || ttl != want {
			t.Errorf("%s: ttl=%d ok=%v, want %d/true", typ, ttl, ok, want)
		}
	}
}
