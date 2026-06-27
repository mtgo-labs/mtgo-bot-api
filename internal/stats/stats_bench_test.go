package stats

import "testing"

// BenchmarkRecordRequest measures the per-request overhead of stats tracking
// (atomic counter increments + per-bot map update under RWMutex).
func BenchmarkRecordRequest(b *testing.B) {
	s := New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.RecordRequest("123:ABC", true)
	}
}
