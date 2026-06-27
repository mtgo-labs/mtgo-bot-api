package stats

import (
	"fmt"
	"sync"
	"testing"
)

// TestStatsConcurrent exercises the Stats type under concurrent access:
// multiple goroutines call RecordRequest, RecordUpdate, and SetBotConnected
// simultaneously with different tokens, plus concurrent reads via GetGlobal and
// GetBotStats. Run with -race.
func TestStatsConcurrent(t *testing.T) {
	s := New()
	const goroutines = 10
	const iterations = 500

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			token := fmt.Sprintf("bot%d:secret", g)
			for i := 0; i < iterations; i++ {
				s.RecordRequest(token, i%2 == 0)
				s.RecordUpdate(token)
				s.SetBotConnected(token, i%3 == 0)
				// Concurrent reads — must not race with writes.
				_ = s.GetGlobal()
				_ = s.GetBotStats()
			}
		}(g)
	}
	wg.Wait()

	// Verify final counts are consistent.
	snap := s.GetGlobal()
	wantRequests := int64(goroutines * iterations)
	if snap.TotalRequests != wantRequests {
		t.Errorf("TotalRequests = %d, want %d", snap.TotalRequests, wantRequests)
	}
	wantUpdates := int64(goroutines * iterations)
	if snap.TotalUpdates != wantUpdates {
		t.Errorf("TotalUpdates = %d, want %d", snap.TotalUpdates, wantUpdates)
	}
}
