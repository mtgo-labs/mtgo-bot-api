package client

import (
	"context"
	"errors"
	"testing"

	"github.com/mtgo-labs/mtgo/tg"

	"github.com/mtgo-labs/mtgo-bot-api/internal/storage"
)

// usernameInvoker is a tg.Invoker that returns a canned ContactsResolvedPeer for
// contacts.resolveUsername (or an error), and (nil,nil) otherwise — enough to
// exercise warmUsernamePeers without a live connection.
type usernameInvoker struct {
	peer *tg.ContactsResolvedPeer
	err  error
}

func (u *usernameInvoker) RPCInvoke(_ context.Context, input tg.TLObject, _ func(*tg.Reader) (tg.TLObject, error)) (tg.TLObject, error) {
	if _, ok := input.(*tg.ContactsResolveUsernameRequest); ok {
		if u.err != nil {
			return nil, u.err
		}
		return u.peer, nil
	}
	return nil, nil
}

func (u *usernameInvoker) RPCInvokeRaw(_ context.Context, _ tg.TLObject) ([]byte, error) {
	return nil, nil
}

func newUsernameClient(t *testing.T, inv *usernameInvoker) *Client {
	t.Helper()
	store, err := storage.Open(t.TempDir(), "1")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return &Client{
		botID: "1",
		ready: true,
		rpc:   tg.NewRPCClient(inv),
		store: store,
		msgs:  newMsgCache(defaultMsgCacheCap),
	}
}

// Regression (G2.3): a @username chat_id is resolved live via
// contacts.resolveUsername, cached, and rewritten to the numeric Bot API
// chat_id so the handler's peer resolution succeeds. Non-peer args are
// untouched, and the resolved peer is cached so the next call skips the RPC.
func TestWarmUsernamePeersRewritesChannel(t *testing.T) {
	c := newUsernameClient(t, &usernameInvoker{peer: &tg.ContactsResolvedPeer{
		Peer:  &tg.PeerChannel{ChannelID: 1234567890},
		Chats: []tg.ChatClass{&tg.Channel{ID: 1234567890, AccessHash: 777, Username: "somechannel"}},
	}})
	q := newQ("sendmessage", map[string]string{"chat_id": "@somechannel", "text": "hi @notapeer"})
	c.warmUsernamePeers(context.Background(), q)

	want := peerBotAPIChatID(storage.Peer{ID: 1234567890, Type: storage.PeerTypeChannel})
	if q.Args["chat_id"] != want {
		t.Errorf("chat_id = %q, want %q (rewritten from @somechannel)", q.Args["chat_id"], want)
	}
	if q.Args["text"] != "hi @notapeer" {
		t.Errorf("text = %q (non-peer @-arg must be untouched)", q.Args["text"])
	}
	if p, err := c.store.GetPeerByUsername(context.Background(), "somechannel"); err != nil || p.ID != 1234567890 || p.AccessHash != 777 {
		t.Errorf("resolved peer not cached: %+v %v", p, err)
	}
}

func TestWarmUsernamePeersRewritesUser(t *testing.T) {
	c := newUsernameClient(t, &usernameInvoker{peer: &tg.ContactsResolvedPeer{
		Peer:  &tg.PeerUser{UserID: 42},
		Users: []tg.UserClass{&tg.User{ID: 42, AccessHash: 9, Username: "alice"}},
	}})
	q := newQ("sendmessage", map[string]string{"chat_id": "@alice"})
	c.warmUsernamePeers(context.Background(), q)
	if q.Args["chat_id"] != "42" {
		t.Errorf("chat_id = %q, want 42", q.Args["chat_id"])
	}
}

// An @username that can't be resolved is left untouched for the handler to
// surface as a 400 (no rewrite to a bogus id).
func TestWarmUsernamePeersUnresolvedLeftUntouched(t *testing.T) {
	c := newUsernameClient(t, &usernameInvoker{err: errors.New("USERNAME_INVALID")})
	q := newQ("sendmessage", map[string]string{"chat_id": "@ghost"})
	c.warmUsernamePeers(context.Background(), q)
	if q.Args["chat_id"] != "@ghost" {
		t.Errorf("unresolved chat_id = %q, want @ghost", q.Args["chat_id"])
	}
}

func TestPeerBotAPIChatID(t *testing.T) {
	cases := []struct {
		p    storage.Peer
		want string
	}{
		{storage.Peer{ID: 42, Type: storage.PeerTypeUser}, "42"},
		{storage.Peer{ID: 77, Type: storage.PeerTypeChat}, "-77"},
		{storage.Peer{ID: 5, Type: storage.PeerTypeChannel}, "-1000000000005"},
	}
	for _, tc := range cases {
		if got := peerBotAPIChatID(tc.p); got != tc.want {
			t.Errorf("peerBotAPIChatID(%+v) = %q, want %q", tc.p, got, tc.want)
		}
	}
}
