package client

import (
	"testing"

	"github.com/mtgo-labs/mtgo/telegram"
	mtgotypes "github.com/mtgo-labs/mtgo/telegram/types"
	"github.com/mtgo-labs/mtgo/tg"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// Regression (InlineQuery location/chat_type): an inline-query update must emit
// the location (from the shared geo point) and chat_type (from the peer type),
// not just id/from/query/offset.
func TestBuildInlineQueryEmitsLocationAndChatType(t *testing.T) {
	upd := &telegram.Update{
		InlineQuery: &mtgotypes.InlineQuery{
			ID:       5,
			UserID:   7,
			Query:    "hi",
			Offset:   "0",
			Geo:      &tg.GeoPoint{Lat: 1.5, Long: 2.5, AccuracyRadius: 30},
			PeerType: &tg.InlineQueryPeerTypeChat{},
		},
		Users: map[int64]*mtgotypes.User{7: {ID: 7, FirstName: "Bob"}},
	}

	obj := buildUpdateObject(upd, 0, nil)
	iq, ok := obj["inline_query"].(*apitypes.InlineQuery)
	if !ok {
		t.Fatalf("inline_query = %T, want *apitypes.InlineQuery", obj["inline_query"])
	}
	if iq.ChatType != "group" {
		t.Errorf("chat_type = %q, want group", iq.ChatType)
	}
	if iq.Location == nil ||
		float64(iq.Location.Latitude) != 1.5 ||
		float64(iq.Location.Longitude) != 2.5 ||
		float64(iq.Location.HorizontalAccuracy) != 30 {
		t.Errorf("location = %+v, want {1.5, 2.5, hacc=30}", iq.Location)
	}

	// No geo / no peer type → both fields omitted.
	upd2 := &telegram.Update{
		InlineQuery: &mtgotypes.InlineQuery{ID: 6, UserID: 7, Query: "x", Offset: ""},
		Users:       map[int64]*mtgotypes.User{7: {ID: 7, FirstName: "Bob"}},
	}
	iq2 := buildUpdateObject(upd2, 0, nil)["inline_query"].(*apitypes.InlineQuery)
	if iq2.Location != nil || iq2.ChatType != "" {
		t.Errorf("expected omitted location/chat_type, got loc=%+v chat_type=%q", iq2.Location, iq2.ChatType)
	}
}
