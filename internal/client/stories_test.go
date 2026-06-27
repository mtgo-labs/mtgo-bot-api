package client

import (
	"context"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/server"
	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func storyQuery(args map[string]string) *server.Query {
	q := server.NewQuery()
	q.Args = args
	return q
}

func TestStoryChat(t *testing.T) {
	cases := []struct {
		name    string
		peer    tg.PeerClass
		reqPeer tg.InputPeerClass
		wantID  int64
		wantTyp apitypes.ChatType
	}{
		{"user", &tg.PeerUser{UserID: 7}, nil, 7, apitypes.ChatTypePrivate},
		{"channel", &tg.PeerChannel{ChannelID: 5}, nil, -1000000000005, apitypes.ChatTypeSupergroup},
		{"chat", &tg.PeerChat{ChatID: 9}, nil, -9, apitypes.ChatTypeGroup},
		{"fallback-req-user", nil, &tg.InputPeerUser{UserID: 42}, 42, apitypes.ChatTypePrivate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := storyChat(tc.peer, tc.reqPeer)
			if got.ID != tc.wantID || got.Type != tc.wantTyp {
				t.Errorf("storyChat = {ID:%d Type:%s}, want {ID:%d Type:%s}", got.ID, got.Type, tc.wantID, tc.wantTyp)
			}
		})
	}
}

func TestExtractStoryFromUpdates(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		res := &tg.Updates{Updates: []tg.UpdateClass{
			&tg.UpdateStory{Peer: &tg.PeerUser{UserID: 7}, Story: &tg.StoryItem{ID: 5}},
		}}
		id, peer := extractStoryFromUpdates(res)
		if id != 5 {
			t.Fatalf("id = %d, want 5", id)
		}
		if pu, ok := peer.(*tg.PeerUser); !ok || pu.UserID != 7 {
			t.Errorf("peer = %v, want PeerUser{7}", peer)
		}
	})
	t.Run("none", func(t *testing.T) {
		if id, _ := extractStoryFromUpdates(&tg.Updates{}); id != 0 {
			t.Errorf("id = %d, want 0 for empty updates", id)
		}
		if id, _ := extractStoryFromUpdates(nil); id != 0 {
			t.Errorf("id = %d, want 0 for nil", id)
		}
	})
}

func TestStoryResult(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		res := &tg.Updates{Updates: []tg.UpdateClass{
			&tg.UpdateStory{Peer: &tg.PeerUser{UserID: 7}, Story: &tg.StoryItem{ID: 5}},
		}}
		s, err := storyResult(res, &tg.InputPeerUser{UserID: 7})
		if err != nil {
			t.Fatal(err)
		}
		if s.ID != 5 || s.Chat.ID != 7 || s.Chat.Type != apitypes.ChatTypePrivate {
			t.Errorf("Story = %+v", s)
		}
	})
	t.Run("no story → 500", func(t *testing.T) {
		if _, err := storyResult(&tg.Updates{}, nil); err == nil {
			t.Error("expected error when no story in response")
		}
	})
}

// storyContentMedia validation paths return before the upload step, so a bare
// Client (no connection) is sufficient.
func TestStoryContentMediaValidation(t *testing.T) {
	c := &Client{}
	ctx := context.Background()
	cases := []struct {
		args map[string]string
		want string
	}{
		{nil, "story content isn't specified"},
		{map[string]string{"content": "not json"}, "can't parse story content JSON object"},
		{map[string]string{"content": `{"type":"gif"}`}, "invalid story content type specified"},
	}
	for _, tc := range cases {
		_, err := c.storyContentMedia(ctx, storyQuery(tc.args))
		if err == nil || !contains([]byte(err.Error()), tc.want) {
			t.Errorf("args=%v: err=%v, want containing %q", tc.args, err, tc.want)
		}
	}
}
