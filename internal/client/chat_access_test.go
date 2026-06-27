package client

import (
	"context"
	"testing"

	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

func mustSavePeer(t *testing.T, s *storage.Store, p storage.Peer) {
	t.Helper()
	if err := s.SavePeer(context.Background(), p); err != nil {
		t.Fatalf("save peer: %v", err)
	}
}

// TestCheckWriteAccess drives the pre-flight with a seeded store: each cached
// status (kicked/left/deleted/upgraded) yields the exact friendly string, an
// unknown chat falls through (nil), and a member chat is not blocked.
func TestCheckWriteAccess(t *testing.T) {
	s, err := storage.Open(t.TempDir(), "123")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Channel (broadcast) the bot left: chat_id -100111 → store id 111.
	mustSavePeer(t, s, storage.Peer{ID: 111, Type: storage.PeerTypeChannel})
	_ = s.SaveBotMemberStatus(context.Background(), 111, "left")
	_ = s.SaveChatFlags(context.Background(), 111, false, false, "")
	// Supergroup the bot was kicked from: -100222 → 222, megagroup.
	mustSavePeer(t, s, storage.Peer{ID: 222, Type: storage.PeerTypeChannel})
	_ = s.SaveBotMemberStatus(context.Background(), 222, "kicked")
	_ = s.SaveChatFlags(context.Background(), 222, true, false, "")
	// Basic group the bot left: -333 → store id 333.
	mustSavePeer(t, s, storage.Peer{ID: 333, Type: storage.PeerTypeChat})
	_ = s.SaveBotMemberStatus(context.Background(), 333, "left")
	// Basic group deleted: -444 → 444.
	mustSavePeer(t, s, storage.Peer{ID: 444, Type: storage.PeerTypeChat})
	_ = s.SaveChatFlags(context.Background(), 444, false, true, "")
	// Group upgraded to a supergroup: -555 → 555, migrated_to "-100999".
	mustSavePeer(t, s, storage.Peer{ID: 555, Type: storage.PeerTypeChat})
	_ = s.SaveChatFlags(context.Background(), 555, false, true, "-100999")
	// Channel the bot is a member of: -100666 → 666 (must NOT be blocked).
	mustSavePeer(t, s, storage.Peer{ID: 666, Type: storage.PeerTypeChannel})
	_ = s.SaveBotMemberStatus(context.Background(), 666, "member")

	c := &Client{store: s}

	cases := []struct {
		chatID, want string
		code         int
		migrateTo    int64
	}{
		{"-100111", "Forbidden: bot is not a member of the channel chat", 403, 0},
		{"-100222", "Forbidden: bot was kicked from the supergroup chat", 403, 0},
		{"-333", "Forbidden: bot is not a member of the group chat", 403, 0},
		{"-444", "Forbidden: the group chat was deleted", 403, 0},
		{"-555", "Bad Request: group chat was upgraded to a supergroup chat", 400, -100999},
	}
	for _, tc := range cases {
		q := newQ("sendmessage", map[string]string{"chat_id": tc.chatID, "text": "x"})
		err := c.checkWriteAccess(context.Background(), q)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.chatID)
			continue
		}
		e, ok := err.(*Error)
		if !ok {
			t.Errorf("%s: error type %T, want *Error", tc.chatID, err)
			continue
		}
		if e.Code != tc.code || e.Description != tc.want {
			t.Errorf("%s: got %d %q, want %d %q", tc.chatID, e.Code, e.Description, tc.code, tc.want)
		}
		if tc.migrateTo != 0 {
			if e.Params == nil || e.Params.MigrateToChatID != tc.migrateTo {
				t.Errorf("%s: migrate_to_chat_id = %v, want %d", tc.chatID, e.Params, tc.migrateTo)
			}
		}
	}

	// Fall-through safety: unknown / invalid / @username chat_ids → nil (never block).
	for _, cid := range []string{"-100777777", "-88888", "12345", "@someone", "notanumber", ""} {
		q := newQ("sendmessage", map[string]string{"chat_id": cid, "text": "x"})
		if err := c.checkWriteAccess(context.Background(), q); err != nil {
			t.Errorf("%q: expected nil (fall-through), got %v", cid, err)
		}
	}
	// Member channel → nil.
	q := newQ("sendmessage", map[string]string{"chat_id": "-100666", "text": "x"})
	if err := c.checkWriteAccess(context.Background(), q); err != nil {
		t.Errorf("member channel: expected nil, got %v", err)
	}
	// nil store → always nil (cold-start / no persistence).
	c2 := &Client{}
	if err := c2.checkWriteAccess(context.Background(), newQ("sendmessage", map[string]string{"chat_id": "-100111", "text": "x"})); err != nil {
		t.Errorf("nil store: expected nil, got %v", err)
	}
}
