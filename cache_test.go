package cachekit

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "value")

	val, ok := c.Get("key")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "value" {
		t.Fatalf("expected 'value', got %q", val)
	}
}

func TestGetMissing(t *testing.T) {
	c := New[string](10, 0)
	_, ok := c.Get("missing")
	if ok {
		t.Fatal("expected key to not exist")
	}
}

func TestDelete(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "value")

	deleted := c.Delete("key")
	if !deleted {
		t.Fatal("expected Delete to return true")
	}

	_, ok := c.Get("key")
	if ok {
		t.Fatal("expected key to be deleted")
	}

	deleted = c.Delete("nonexistent")
	if deleted {
		t.Fatal("expected Delete to return false for missing key")
	}
}

func TestHas(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "value")

	if !c.Has("key") {
		t.Fatal("expected Has to return true")
	}
	if c.Has("missing") {
		t.Fatal("expected Has to return false")
	}
}

func TestClear(t *testing.T) {
	c := New[string](10, 0)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Clear()

	if c.Size() != 0 {
		t.Fatalf("expected size 0 after Clear, got %d", c.Size())
	}
}

func TestKeys(t *testing.T) {
	c := New[string](10, 0)
	c.Set("a", "1")
	c.Set("b", "2")

	keys := c.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}

func TestSize(t *testing.T) {
	c := New[string](10, 0)
	if c.Size() != 0 {
		t.Fatalf("expected size 0, got %d", c.Size())
	}
	c.Set("a", "1")
	if c.Size() != 1 {
		t.Fatalf("expected size 1, got %d", c.Size())
	}
}

// TTL tests

func TestDefaultTTLExpiration(t *testing.T) {
	c := New[string](10, 50*time.Millisecond)
	c.Set("key", "value")

	val, ok := c.Get("key")
	if !ok || val != "value" {
		t.Fatal("expected key to exist before TTL")
	}

	time.Sleep(60 * time.Millisecond)

	_, ok = c.Get("key")
	if ok {
		t.Fatal("expected key to be expired")
	}
}

func TestPerEntryTTL(t *testing.T) {
	c := New[string](10, time.Hour) // long default TTL
	c.Set("short", "val", WithTTL(50*time.Millisecond))
	c.Set("long", "val")

	time.Sleep(60 * time.Millisecond)

	_, ok := c.Get("short")
	if ok {
		t.Fatal("expected short-TTL entry to be expired")
	}

	_, ok = c.Get("long")
	if !ok {
		t.Fatal("expected long-TTL entry to still exist")
	}
}

func TestZeroTTLNoExpiration(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "value")

	// Zero TTL means no expiration
	time.Sleep(10 * time.Millisecond)
	_, ok := c.Get("key")
	if !ok {
		t.Fatal("expected key to never expire with zero TTL")
	}
}

func TestExpiredKeysNotInKeys(t *testing.T) {
	c := New[string](10, 50*time.Millisecond)
	c.Set("expires", "val")
	c.Set("stays", "val", WithTTL(time.Hour))

	time.Sleep(60 * time.Millisecond)

	keys := c.Keys()
	for _, k := range keys {
		if k == "expires" {
			t.Fatal("expired key should not appear in Keys()")
		}
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
}

// LRU eviction tests

func TestLRUEviction(t *testing.T) {
	c := New[string](3, 0)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("c", "3")
	c.Set("d", "4") // should evict "a" (LRU)

	_, ok := c.Get("a")
	if ok {
		t.Fatal("expected 'a' to be evicted as LRU")
	}

	_, ok = c.Get("d")
	if !ok {
		t.Fatal("expected 'd' to exist")
	}
}

func TestGetPromotesEntry(t *testing.T) {
	c := New[string](3, 0)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("c", "3")

	// Access "a" to make it most recently used
	c.Get("a")

	c.Set("d", "4") // should evict "b" (now LRU), not "a"

	_, ok := c.Get("a")
	if !ok {
		t.Fatal("expected 'a' to survive after Get promotion")
	}

	_, ok = c.Get("b")
	if ok {
		t.Fatal("expected 'b' to be evicted as LRU")
	}
}

func TestEvictPrefersExpired(t *testing.T) {
	c := New[string](3, 0)
	c.Set("a", "1", WithTTL(50*time.Millisecond))
	c.Set("b", "2")
	c.Set("c", "3")

	time.Sleep(60 * time.Millisecond)

	c.Set("d", "4") // should evict expired "a" first

	_, ok := c.Get("b")
	if !ok {
		t.Fatal("expected 'b' to survive (expired entry evicted first)")
	}
	_, ok = c.Get("c")
	if !ok {
		t.Fatal("expected 'c' to survive (expired entry evicted first)")
	}
}

// Tag tests

func TestTagInvalidation(t *testing.T) {
	c := New[string](10, 0)
	c.Set("a", "1", WithTags("users"))
	c.Set("b", "2", WithTags("users", "admin"))
	c.Set("c", "3", WithTags("posts"))

	count := c.InvalidateByTag("users")
	if count != 2 {
		t.Fatalf("expected 2 removed, got %d", count)
	}

	_, ok := c.Get("a")
	if ok {
		t.Fatal("expected 'a' to be invalidated")
	}
	_, ok = c.Get("b")
	if ok {
		t.Fatal("expected 'b' to be invalidated")
	}
	_, ok = c.Get("c")
	if !ok {
		t.Fatal("expected 'c' to survive")
	}
}

func TestInvalidateByTagNoMatch(t *testing.T) {
	c := New[string](10, 0)
	c.Set("a", "1", WithTags("users"))

	count := c.InvalidateByTag("nonexistent")
	if count != 0 {
		t.Fatalf("expected 0 removed, got %d", count)
	}
}

func TestMultipleTagsOnEntry(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "val", WithTags("tag1", "tag2"))

	count := c.InvalidateByTag("tag2")
	if count != 1 {
		t.Fatalf("expected 1 removed, got %d", count)
	}
	_, ok := c.Get("key")
	if ok {
		t.Fatal("expected key to be invalidated by tag2")
	}
}

// Overwrite tests

func TestOverwriteUpdatesValue(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "old")
	c.Set("key", "new")

	val, ok := c.Get("key")
	if !ok || val != "new" {
		t.Fatalf("expected 'new', got %q", val)
	}
	if c.Size() != 1 {
		t.Fatalf("expected size 1 after overwrite, got %d", c.Size())
	}
}

func TestOverwriteWithDifferentTTL(t *testing.T) {
	c := New[string](10, time.Hour)
	c.Set("key", "v1")
	c.Set("key", "v2", WithTTL(50*time.Millisecond))

	val, ok := c.Get("key")
	if !ok || val != "v2" {
		t.Fatalf("expected 'v2', got %q", val)
	}

	time.Sleep(60 * time.Millisecond)

	_, ok = c.Get("key")
	if ok {
		t.Fatal("expected key to expire with new short TTL")
	}
}

func TestOverwriteWithDifferentTags(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "v1", WithTags("tag1"))
	c.Set("key", "v2", WithTags("tag2"))

	// Old tag should no longer work
	count := c.InvalidateByTag("tag1")
	if count != 0 {
		t.Fatal("expected old tag to not match after overwrite")
	}

	// New tag should work
	count = c.InvalidateByTag("tag2")
	if count != 1 {
		t.Fatalf("expected 1 removed by new tag, got %d", count)
	}
}

func TestRapidSequentialInsertions(t *testing.T) {
	c := New[int](10, 0)
	for i := 0; i < 100; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}

	// Only 10 entries should remain
	if c.Size() != 10 {
		t.Fatalf("expected 10 entries after rapid inserts, got %d", c.Size())
	}

	// The last 10 entries should be present
	for i := 90; i < 100; i++ {
		_, ok := c.Get(fmt.Sprintf("key%d", i))
		if !ok {
			t.Fatalf("expected key%d to exist", i)
		}
	}
}

// Concurrency tests

func TestConcurrentAccess(t *testing.T) {
	c := New[int](100, time.Second)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Set(fmt.Sprintf("key%d", i), i)
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Get(fmt.Sprintf("key%d", i))
			c.Has(fmt.Sprintf("key%d", i))
			c.Keys()
		}(i)
	}

	wg.Wait()
}

func TestConcurrentInvalidateAndRead(t *testing.T) {
	c := New[string](100, 0)
	for i := 0; i < 50; i++ {
		c.Set(fmt.Sprintf("key%d", i), "val", WithTags("all"))
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		c.InvalidateByTag("all")
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			c.Get(fmt.Sprintf("key%d", i))
		}
	}()
	wg.Wait()
}

// Edge cases

func TestMaxSizeZeroPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for maxSize 0")
		}
	}()
	New[string](0, 0)
}

func TestMaxSizeNegativePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for negative maxSize")
		}
	}()
	New[string](-1, 0)
}

// GetOrSet tests

func TestGetOrSetMiss(t *testing.T) {
	c := New[string](10, 0)
	calls := 0
	val := c.GetOrSet("key", func() string {
		calls++
		return "computed"
	})
	if val != "computed" {
		t.Fatalf("expected 'computed', got %q", val)
	}
	if calls != 1 {
		t.Fatalf("expected factory called once, got %d", calls)
	}
	// Verify it was cached
	v, ok := c.Get("key")
	if !ok || v != "computed" {
		t.Fatal("expected key to be cached after GetOrSet")
	}
}

func TestGetOrSetHit(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "existing")
	calls := 0
	val := c.GetOrSet("key", func() string {
		calls++
		return "computed"
	})
	if val != "existing" {
		t.Fatalf("expected 'existing', got %q", val)
	}
	if calls != 0 {
		t.Fatalf("expected factory not called, got %d calls", calls)
	}
}

func TestGetOrSetWithOptions(t *testing.T) {
	c := New[string](10, 0)
	c.GetOrSet("key", func() string { return "val" }, WithTTL(50*time.Millisecond))

	_, ok := c.Get("key")
	if !ok {
		t.Fatal("expected key to exist before TTL")
	}

	time.Sleep(60 * time.Millisecond)

	_, ok = c.Get("key")
	if ok {
		t.Fatal("expected key to expire with TTL from GetOrSet")
	}
}

// GetMany tests

func TestGetMany(t *testing.T) {
	c := New[string](10, 0)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("c", "3")

	result := c.GetMany([]string{"a", "c", "missing"})
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result["a"] != "1" {
		t.Fatalf("expected '1' for 'a', got %q", result["a"])
	}
	if result["c"] != "3" {
		t.Fatalf("expected '3' for 'c', got %q", result["c"])
	}
	if _, ok := result["missing"]; ok {
		t.Fatal("expected 'missing' to be absent")
	}
}

func TestGetManyEmpty(t *testing.T) {
	c := New[string](10, 0)
	result := c.GetMany([]string{})
	if len(result) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(result))
	}
}

func TestGetManySkipsExpired(t *testing.T) {
	c := New[string](10, 50*time.Millisecond)
	c.Set("expires", "val")
	c.Set("stays", "val", WithTTL(time.Hour))

	time.Sleep(60 * time.Millisecond)

	result := c.GetMany([]string{"expires", "stays"})
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if _, ok := result["stays"]; !ok {
		t.Fatal("expected 'stays' in results")
	}
}

// OnEvict tests

func TestOnEvictLRU(t *testing.T) {
	c := New[string](3, 0)
	var evictedKey string
	var evictedVal string
	c.OnEvict(func(key string, value string) {
		evictedKey = key
		evictedVal = value
	})

	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("c", "3")
	c.Set("d", "4") // evicts "a"

	if evictedKey != "a" {
		t.Fatalf("expected evicted key 'a', got %q", evictedKey)
	}
	if evictedVal != "1" {
		t.Fatalf("expected evicted value '1', got %q", evictedVal)
	}
}

func TestOnEvictTTL(t *testing.T) {
	c := New[string](10, 50*time.Millisecond)
	var evictedKey string
	c.OnEvict(func(key string, value string) {
		evictedKey = key
	})

	c.Set("key", "val")
	time.Sleep(60 * time.Millisecond)

	// Get triggers TTL eviction
	c.Get("key")

	if evictedKey != "key" {
		t.Fatalf("expected evicted key 'key', got %q", evictedKey)
	}
}

func TestOnEvictNotCalledOnDelete(t *testing.T) {
	c := New[string](10, 0)
	called := false
	c.OnEvict(func(key string, value string) {
		called = true
	})

	c.Set("key", "val")
	c.Delete("key")

	if called {
		t.Fatal("expected OnEvict not to be called on explicit Delete")
	}
}

// Stats tests

func TestStatsHitsAndMisses(t *testing.T) {
	c := New[string](10, 0)
	c.Set("key", "val")

	c.Get("key")     // hit
	c.Get("key")     // hit
	c.Get("missing") // miss

	stats := c.Stats()
	if stats.Hits != 2 {
		t.Fatalf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestStatsEvictions(t *testing.T) {
	c := New[string](2, 0)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("c", "3") // evicts "a"
	c.Set("d", "4") // evicts "b"

	stats := c.Stats()
	if stats.Evictions != 2 {
		t.Fatalf("expected 2 evictions, got %d", stats.Evictions)
	}
}

func TestStatsTTLEviction(t *testing.T) {
	c := New[string](10, 50*time.Millisecond)
	c.Set("key", "val")

	time.Sleep(60 * time.Millisecond)
	c.Get("key") // triggers TTL eviction

	stats := c.Stats()
	if stats.Evictions != 1 {
		t.Fatalf("expected 1 eviction, got %d", stats.Evictions)
	}
}

func TestStatsInitiallyZero(t *testing.T) {
	c := New[string](10, 0)
	stats := c.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Evictions != 0 {
		t.Fatalf("expected all stats to be 0, got %+v", stats)
	}
}

// DeleteWhere tests

func TestDeleteWhere(t *testing.T) {
	c := New[int](10, 0)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4)

	removed := c.DeleteWhere(func(key string, value int) bool {
		return value%2 == 0
	})
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}
	if c.Size() != 2 {
		t.Fatalf("expected size 2, got %d", c.Size())
	}

	_, ok := c.Get("a")
	if !ok {
		t.Fatal("expected 'a' to survive")
	}
	_, ok = c.Get("c")
	if !ok {
		t.Fatal("expected 'c' to survive")
	}
}

func TestDeleteWhereNoMatch(t *testing.T) {
	c := New[string](10, 0)
	c.Set("a", "1")

	removed := c.DeleteWhere(func(key string, value string) bool {
		return false
	})
	if removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
	if c.Size() != 1 {
		t.Fatalf("expected size 1, got %d", c.Size())
	}
}

func TestDeleteWhereAll(t *testing.T) {
	c := New[string](10, 0)
	c.Set("a", "1")
	c.Set("b", "2")

	removed := c.DeleteWhere(func(key string, value string) bool {
		return true
	})
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}
	if c.Size() != 0 {
		t.Fatalf("expected size 0, got %d", c.Size())
	}
}

func TestDeleteWhereByKeyPrefix(t *testing.T) {
	c := New[string](10, 0)
	c.Set("user:1", "alice")
	c.Set("user:2", "bob")
	c.Set("post:1", "hello")

	removed := c.DeleteWhere(func(key string, value string) bool {
		return len(key) > 4 && key[:5] == "user:"
	})
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}
	_, ok := c.Get("post:1")
	if !ok {
		t.Fatal("expected 'post:1' to survive")
	}
}
