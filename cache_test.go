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
