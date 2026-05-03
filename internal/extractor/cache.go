package extractor

import (
	"container/list"
	"sync"
)

// RecordCache is a thread-safe fixed-capacity LRU keyed by SHA-256 hex string.
// Capacity 0 disables caching entirely (Put becomes a no-op).
type RecordCache struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List               // front = most recent, back = least recent
	index    map[string]*list.Element // key -> *list.Element holding *cacheEntry
}

type cacheEntry struct {
	key     string
	records []Record
}

// NewRecordCache returns a cache that evicts the least-recently-used entry
// once size exceeds capacity. capacity <= 0 disables caching.
func NewRecordCache(capacity int) *RecordCache {
	return &RecordCache{
		capacity: capacity,
		ll:       list.New(),
		index:    make(map[string]*list.Element),
	}
}

// Get returns the cached records for key, promoting the entry to most-recent.
func (c *RecordCache) Get(key string) ([]Record, bool) {
	if c.capacity <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.index[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*cacheEntry).records, true
}

// Put inserts or refreshes an entry, evicting the least-recently-used entry
// if capacity is exceeded.
func (c *RecordCache) Put(key string, records []Record) {
	if c.capacity <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.index[key]; ok {
		el.Value.(*cacheEntry).records = records
		c.ll.MoveToFront(el)
		return
	}

	el := c.ll.PushFront(&cacheEntry{key: key, records: records})
	c.index[key] = el

	for c.ll.Len() > c.capacity {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.ll.Remove(oldest)
		delete(c.index, oldest.Value.(*cacheEntry).key)
	}
}

// Len returns the current number of cached entries (mainly for tests).
func (c *RecordCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
