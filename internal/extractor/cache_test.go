package extractor

import (
	"sync"
	"testing"
)

func TestLRUCache_PutGet(t *testing.T) {
	c := NewRecordCache(2)
	c.Put("a", ExtractOutput{Records: []Record{{"defendant": "X"}}})
	got, ok := c.Get("a")
	if !ok {
		t.Fatal("expected hit for key a")
	}
	if got.Records[0]["defendant"] != "X" {
		t.Fatalf("got %v", got)
	}
}

func TestLRUCache_EvictsOldest(t *testing.T) {
	c := NewRecordCache(2)
	c.Put("a", ExtractOutput{Records: []Record{{"k": "1"}}})
	c.Put("b", ExtractOutput{Records: []Record{{"k": "2"}}})
	c.Put("c", ExtractOutput{Records: []Record{{"k": "3"}}}) // should evict "a"

	if _, ok := c.Get("a"); ok {
		t.Fatal("a should have been evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("b should still be present")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("c should still be present")
	}
}

func TestLRUCache_GetPromotes(t *testing.T) {
	c := NewRecordCache(2)
	c.Put("a", ExtractOutput{Records: []Record{{"k": "1"}}})
	c.Put("b", ExtractOutput{Records: []Record{{"k": "2"}}})
	_, _ = c.Get("a")                                        // touch a -> a now most-recent
	c.Put("c", ExtractOutput{Records: []Record{{"k": "3"}}}) // should evict b, not a

	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted (least recent)")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should still be present after promotion")
	}
}

func TestLRUCache_ZeroCapacity_IsNoOp(t *testing.T) {
	c := NewRecordCache(0)
	c.Put("a", ExtractOutput{Records: []Record{{"k": "1"}}})
	if _, ok := c.Get("a"); ok {
		t.Fatal("zero-capacity cache must never store")
	}
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	c := NewRecordCache(50)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			c.Put(key, ExtractOutput{Records: []Record{{"k": key}}})
			_, _ = c.Get(key)
		}(i)
	}
	wg.Wait()
	// no race / panic = pass; run with -race
}
