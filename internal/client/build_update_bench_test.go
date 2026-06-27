package client

import (
	"testing"

	"github.com/mtgo-labs/mtgo/telegram"
	mtgotypes "github.com/mtgo-labs/mtgo/telegram/types"
	"github.com/mtgo-labs/mtgo/tg"
)

func BenchmarkBuildUpdateObject(b *testing.B) {
	upd := &telegram.Update{
		Message: &mtgotypes.Message{Raw: &tg.Message{
			ID:      42,
			Date:    1700000000,
			Message: "hello",
			PeerID:  &tg.PeerUser{UserID: 123},
			FromID:  &tg.PeerUser{UserID: 123},
		}},
		Users: map[int64]*mtgotypes.User{
			123: {ID: 123, FirstName: "Alice", Username: "alice"},
		},
	}
	cache := newMsgCache(100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildUpdateObject(upd, 0, cache)
	}
}
