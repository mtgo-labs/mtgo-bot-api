package client

import (
	"encoding/json"
	"testing"
)

// BenchmarkInjectUpdateID measures the per-update overhead of the update_id
// injection path: set update_id on a pre-built map and marshal to JSON (the
// build callback inside pushUpdate). This mirrors what was formerly the
// standalone injectUpdateID function — now inlined at push time.
func BenchmarkInjectUpdateID(b *testing.B) {
	obj := map[string]any{
		"message": map[string]any{
			"message_id": 42,
			"date":       1700000000,
			"chat":       map[string]any{"id": 123, "type": "private"},
			"from":       map[string]any{"id": 456, "is_bot": false, "first_name": "Test"},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj["update_id"] = int64(999)
		if _, err := json.Marshal(obj); err != nil {
			b.Fatal(err)
		}
	}
}
