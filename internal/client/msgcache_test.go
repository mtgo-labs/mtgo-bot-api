package client

import (
	"sync"
	"testing"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

func TestMsgCachePutGet(t *testing.T) {
	c := newMsgCache(8)
	if got := c.len(); got != 0 {
		t.Fatalf("initial len = %d, want 0", got)
	}
	msg := &apitypes.Message{MessageID: 42, Chat: apitypes.Chat{ID: 7}, Text: "hi"}
	c.put(7, 42, msg, nil)

	got, ok := c.get(7, 42)
	if !ok {
		t.Fatal("miss after put")
	}
	if got.Text != "hi" {
		t.Fatalf("got %q, want %q", got.Text, "hi")
	}
	if c.len() != 1 {
		t.Fatalf("len = %d, want 1", c.len())
	}
}

func TestMsgCacheMiss(t *testing.T) {
	c := newMsgCache(8)
	if _, ok := c.get(1, 99); ok {
		t.Fatal("hit on uncached key")
	}
}

func TestMsgCacheEvictionLRU(t *testing.T) {
	c := newMsgCache(2)
	c.put(1, 1, &apitypes.Message{Text: "a"}, nil)
	c.put(1, 2, &apitypes.Message{Text: "b"}, nil)
	// Touch (1,1) so (1,2) becomes least-recently-used.
	c.get(1, 1)
	c.put(1, 3, &apitypes.Message{Text: "c"}, nil) // should evict (1,2)

	if _, ok := c.get(1, 2); ok {
		t.Fatal("(1,2) should have been evicted as LRU")
	}
	if _, ok := c.get(1, 1); !ok {
		t.Fatal("(1,1) should still be present")
	}
	if _, ok := c.get(1, 3); !ok {
		t.Fatal("(1,3) should be present")
	}
	if c.len() != 2 {
		t.Fatalf("len = %d, want 2", c.len())
	}
}

func TestMsgCacheOverwriteUpdatesRecency(t *testing.T) {
	c := newMsgCache(2)
	c.put(1, 1, &apitypes.Message{Text: "a"}, nil)
	c.put(1, 2, &apitypes.Message{Text: "b"}, nil)
	// Re-putting (1,1) should refresh it; (1,2) becomes LRU.
	c.put(1, 1, &apitypes.Message{Text: "a2"}, nil)
	c.put(1, 3, &apitypes.Message{Text: "c"}, nil) // evicts (1,2)
	if _, ok := c.get(1, 2); ok {
		t.Fatal("(1,2) should have been evicted")
	}
	got, _ := c.get(1, 1)
	if got.Text != "a2" {
		t.Fatalf("overwrite lost: got %q", got.Text)
	}
}

func TestMsgCacheNilSafe(t *testing.T) {
	var c *msgCache
	c.put(1, 1, &apitypes.Message{}, nil) // must not panic
	if _, ok := c.get(1, 1); ok {
		t.Fatal("nil cache should miss")
	}
	if c.len() != 0 {
		t.Fatal("nil cache len should be 0")
	}
}

func TestMsgCacheIgnoresZeroIDAndNil(t *testing.T) {
	c := newMsgCache(8)
	c.put(1, 0, &apitypes.Message{Text: "x"}, nil)
	c.put(1, 5, nil, nil)
	if c.len() != 0 {
		t.Fatalf("len = %d, want 0 (zero id / nil msg ignored)", c.len())
	}
}

// TestMsgCacheMediaIndex verifies the media-id → source reverse index used to
// refresh expired file_references: registration, lookup, and cleanup on LRU
// eviction (an evicted message's media ids must no longer resolve).
func TestMsgCacheMediaIndex(t *testing.T) {
	c := newMsgCache(2)
	c.put(10, 100, &apitypes.Message{Text: "doc"}, []int64{777})

	if chatID, msgID, ok := c.sourceByMediaID(777); !ok || chatID != 10 || msgID != 100 {
		t.Fatalf("sourceByMediaID(777) = (%d,%d,%v), want (10,100,true)", chatID, msgID, ok)
	}
	if _, _, ok := c.sourceByMediaID(999); ok {
		t.Fatal("sourceByMediaID(999) should miss")
	}

	// Fill the cache so (10,100) is evicted; its media id must be cleaned up.
	c.put(10, 201, &apitypes.Message{Text: "x"}, nil)
	c.put(10, 202, &apitypes.Message{Text: "y"}, nil) // evicts (10,100)
	if _, _, ok := c.sourceByMediaID(777); ok {
		t.Fatal("sourceByMediaID(777) should miss after its message was evicted")
	}
}

// TestMsgCacheMediaIndexOverwrite verifies re-putting a message with different
// media replaces the indexed ids (stale ids are dropped).
func TestMsgCacheMediaIndexOverwrite(t *testing.T) {
	c := newMsgCache(8)
	c.put(5, 50, &apitypes.Message{Text: "a"}, []int64{1, 2})
	c.put(5, 50, &apitypes.Message{Text: "b"}, []int64{3})
	if _, _, ok := c.sourceByMediaID(1); ok {
		t.Fatal("stale media id 1 should be unindexed after overwrite")
	}
	if chatID, _, ok := c.sourceByMediaID(3); !ok || chatID != 5 {
		t.Fatalf("sourceByMediaID(3) = (%d,%v), want (5,true)", chatID, ok)
	}
}

// TestMsgCacheConcurrent hammers a msgCache from many goroutines doing
// put/get/sourceByMediaID/len simultaneously to surface data races. Each
// goroutine uses a distinct chatID and overlapping msgIDs to exercise
// eviction under contention. Run with -race.
func TestMsgCacheConcurrent(t *testing.T) {
	c := newMsgCache(100)
	const goroutines = 10
	const iterations = 1000

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				chatID := int64(g)
				msgID := int64(i%200 + 1) // keep within cap to exercise eviction
				mediaID := int64(g*100000 + i)
				msg := &apitypes.Message{MessageID: msgID, Chat: apitypes.Chat{ID: chatID}}
				c.put(chatID, msgID, msg, []int64{mediaID})
				c.get(chatID, msgID)
				c.sourceByMediaID(mediaID)
				_ = c.len()
			}
		}(g)
	}
	wg.Wait()

	if got := c.len(); got > 100 {
		t.Errorf("len = %d, exceeds cap 100", got)
	}
}

func TestNewClientUsesConfiguredMsgCacheCap(t *testing.T) {
	c := NewClient(Params{MsgCacheCap: 3}, "123:abc")
	if c.msgs.cap != 3 {
		t.Fatalf("msg cache cap = %d, want 3", c.msgs.cap)
	}
}
