package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

func BenchmarkChat(b *testing.B) {
	peers := []tg.PeerClass{
		&tg.PeerUser{UserID: 123},
		&tg.PeerChat{ChatID: 456},
		&tg.PeerChannel{ChannelID: 789},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Chat(peers[i%len(peers)])
	}
}
