package client

import (
	"container/list"
	"sync"

	apitypes "github.com/mtgo-labs/mtgo-bot-api/internal/types"
)

// msgCache is a bounded LRU of recently-seen Bot API messages, keyed by
// (chat_id, message_id). It mirrors the official server's in-memory store of
// messages it has received — used to populate pinned_message / reply_to_message
// content for messages the bot has already seen (reference Client.cpp:1760 /
// get_reply_to_message_info). A cache miss leaves existing behavior unchanged
// (the inaccessible-message fallback for pinned_message, the omitted field for
// reply_to_message).
//
// It is intentionally nil-safe: every method is a no-op when the receiver is
// nil, so callers (e.g. buildUpdateObject in tests) can pass nil freely.
type msgCache struct {
	mu    sync.Mutex
	cap   int
	items map[msgCacheKey]*list.Element
	ll    *list.List // most-recently-used at the front
	// docs is a reverse index from a downloadable media id (document id or
	// photo id) to the message that carried it, used to refresh expired
	// file_references (mirrors TDLib's FileSourceMessage). It is kept coherent
	// with the LRU: entries are dropped here when their message is evicted.
	docs map[int64]msgCacheKey
}

type msgCacheKey struct {
	chatID int64
	msgID  int64
}

type msgCacheEntry struct {
	key      msgCacheKey
	msg      *apitypes.Message
	mediaIDs []int64 // downloadable media ids indexed from this message
}

// defaultMsgCacheCap bounds memory for the per-bot message store. The official
// server keeps a large set; this is a conservative per-bot ceiling.
const defaultMsgCacheCap = 10000

// newMsgCache returns a bounded LRU message cache. cap<=0 falls back to the
// default.
func newMsgCache(cap int) *msgCache {
	if cap <= 0 {
		cap = defaultMsgCacheCap
	}
	return &msgCache{
		cap:   cap,
		items: make(map[msgCacheKey]*list.Element),
		ll:    list.New(),
		docs:  make(map[int64]msgCacheKey),
	}
}

// put stores msg for (chatID, msgID), marking it most-recently-used and evicting
// the least-recently-used entry when over capacity. mediaIDs are the
// downloadable media ids (document/photo ids) carried by msg, indexed so an
// expired file_reference can be refreshed from this message later. Nil messages
// and zero ids are ignored.
func (c *msgCache) put(chatID, msgID int64, msg *apitypes.Message, mediaIDs []int64) {
	if c == nil || msg == nil || msgID == 0 {
		return
	}
	key := msgCacheKey{chatID, msgID}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		entry := el.Value.(*msgCacheEntry)
		entry.msg = msg
		c.setMediaIDsLocked(entry, mediaIDs)
		return
	}
	entry := &msgCacheEntry{key: key, msg: msg}
	c.setMediaIDsLocked(entry, mediaIDs)
	el := c.ll.PushFront(entry)
	c.items[key] = el
	for c.ll.Len() > c.cap {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.evictLocked(oldest)
	}
}

// sourceByMediaID returns the (chatID, msgID) of the message that carried the
// given downloadable media id, for file_reference refresh. ok is false on a miss.
func (c *msgCache) sourceByMediaID(id int64) (chatID, msgID int64, ok bool) {
	if c == nil || id == 0 {
		return 0, 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	key, hit := c.docs[id]
	return key.chatID, key.msgID, hit
}

// setMediaIDsLocked replaces entry's indexed media ids, reconciling the reverse
// docs map. Caller holds c.mu.
func (c *msgCache) setMediaIDsLocked(entry *msgCacheEntry, mediaIDs []int64) {
	c.unindexLocked(entry)
	entry.mediaIDs = mediaIDs
	for _, id := range mediaIDs {
		c.docs[id] = entry.key
	}
}

// unindexLocked removes entry's media ids from the reverse docs map. Caller
// holds c.mu.
func (c *msgCache) unindexLocked(entry *msgCacheEntry) {
	for _, id := range entry.mediaIDs {
		if cur, ok := c.docs[id]; ok && cur == entry.key {
			delete(c.docs, id)
		}
	}
}

// evictLocked removes the oldest list element and its index entries. Caller
// holds c.mu.
func (c *msgCache) evictLocked(el *list.Element) {
	entry := el.Value.(*msgCacheEntry)
	c.unindexLocked(entry)
	c.ll.Remove(el)
	delete(c.items, entry.key)
}

// get returns the cached message for (chatID, msgID) and marks it
// most-recently-used. Returns (nil, false) on miss.
func (c *msgCache) get(chatID, msgID int64) (*apitypes.Message, bool) {
	if c == nil || msgID == 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[msgCacheKey{chatID, msgID}]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*msgCacheEntry).msg, true
}

// len returns the number of cached messages (primarily a test helper).
func (c *msgCache) len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
