package convert

import (
	"testing"

	"github.com/mtgo-labs/mtgo/tg"
)

// TestInlineQueryChatType verifies the MTProto InlineQueryPeerType → Bot API
// chat_type mapping, traced from TDLib InlineQueriesManager.cpp:2455 composed
// with bot-api JsonInlineQuery (Client.cpp:5376).
func TestInlineQueryChatType(t *testing.T) {
	cases := []struct {
		pt   tg.InlineQueryPeerTypeClass
		want string
	}{
		{nil, ""},
		{&tg.InlineQueryPeerTypeSameBotPm{}, "sender"},
		{&tg.InlineQueryPeerTypeBotPm{}, "private"},
		{&tg.InlineQueryPeerTypePm{}, "private"},
		{&tg.InlineQueryPeerTypeChat{}, "group"},
		{&tg.InlineQueryPeerTypeMegagroup{}, "supergroup"},
		{&tg.InlineQueryPeerTypeBroadcast{}, "channel"},
	}
	for _, tc := range cases {
		if got := InlineQueryChatType(tc.pt); got != tc.want {
			t.Errorf("InlineQueryChatType(%T) = %q, want %q", tc.pt, got, tc.want)
		}
	}
}

func TestGeoPointLocation(t *testing.T) {
	if got := GeoPointLocation(nil); got != nil {
		t.Errorf("nil geo = %v, want nil", got)
	}
	if got := GeoPointLocation(&tg.GeoPointEmpty{}); got != nil {
		t.Errorf("empty geo = %v, want nil", got)
	}
	loc := GeoPointLocation(&tg.GeoPoint{Lat: 12.3456789, Long: -98.7654321, AccuracyRadius: 50})
	if loc == nil {
		t.Fatal("GeoPoint returned nil location")
	}
	if got := float64(loc.Latitude); got < 12.345678 || got > 12.345679 {
		t.Errorf("Latitude = %v, want ~12.345679", loc.Latitude)
	}
	if got := float64(loc.Longitude); got < -98.765433 || got > -98.765432 {
		t.Errorf("Longitude = %v, want ~-98.765432", loc.Longitude)
	}
	if got := float64(loc.HorizontalAccuracy); got != 50 {
		t.Errorf("HorizontalAccuracy = %v, want 50", loc.HorizontalAccuracy)
	}
}
