// Package cachekit provides a generic in-memory LRU cache with TTL and tag-based invalidation.
package cachekit

import (
	"sync"
	"sync/atomic"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
	tags      map[string]struct{}
	key       string
	prev      *entry[V]
	next      *entry[V]
}

// CacheStats holds hit, miss, and eviction counters.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
}

// Cache is a thread-safe in-memory LRU cache with TTL and tag support.
type Cache[V any] struct {
	mu         sync.RWMutex
	items      map[string]*entry[V]
	head       *entry[V]
	tail       *entry[V]
	maxSize    int
	defaultTTL time.Duration
	onEvict    func(key string, value V)
	hits       atomic.Int64
	misses     atomic.Int64
	evictions  atomic.Int64
}

// New creates a new Cache with the given max size and default TTL.
// A zero TTL means entries don't expire by default.
func New[V any](maxSize int, defaultTTL time.Duration) *Cache[V] {
	if maxSize <= 0 {
		panic("cachekit: maxSize must be greater than 0")
	}
	return &Cache[V]{
		items:      make(map[string]*entry[V], maxSize),
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
	}
}

// Set adds or updates an entry in the cache.
func (c *Cache[V]) Set(key string, value V, opts ...SetOption) {
	cfg := setConfig{ttl: c.defaultTTL}
	for _, opt := range opts {
		opt(&cfg)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.items[key]; ok {
		e.value = value
		e.tags = toSet(cfg.tags)
		if cfg.ttl > 0 {
			e.expiresAt = time.Now().Add(cfg.ttl)
		} else {
			e.expiresAt = time.Time{}
		}
		c.moveToFront(e)
		return
	}

	if len(c.items) >= c.maxSize {
		c.evict()
	}

	e := &entry[V]{
		value: value,
		tags:  toSet(cfg.tags),
		key:   key,
	}
	if cfg.ttl > 0 {
		e.expiresAt = time.Now().Add(cfg.ttl)
	}

	c.items[key] = e
	c.pushFront(e)
}

// Get retrieves a value from the cache. Returns the value and true if found and not expired.
func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		var zero V
		return zero, false
	}

	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		c.evictEntry(e)
		c.misses.Add(1)
		var zero V
		return zero, false
	}

	c.moveToFront(e)
	c.hits.Add(1)
	return e.value, true
}

// Has checks if a key exists and is not expired.
func (c *Cache[V]) Has(key string) bool {
	_, ok := c.Get(key)
	return ok
}

// Delete removes an entry by key. Returns true if the key was found.
func (c *Cache[V]) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		return false
	}
	c.remove(e)
	return true
}

// InvalidateByTag removes all entries with the given tag. Returns the count removed.
func (c *Cache[V]) InvalidateByTag(tag string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toRemove []*entry[V]
	for _, e := range c.items {
		if _, ok := e.tags[tag]; ok {
			toRemove = append(toRemove, e)
		}
	}
	for _, e := range toRemove {
		c.remove(e)
	}
	return len(toRemove)
}

// Clear removes all entries.
func (c *Cache[V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*entry[V], c.maxSize)
	c.head = nil
	c.tail = nil
}

// Size returns the number of entries in the cache.
func (c *Cache[V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Keys returns all non-expired keys.
func (c *Cache[V]) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	keys := make([]string, 0, len(c.items))
	for k, e := range c.items {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

// Linked list operations

func (c *Cache[V]) pushFront(e *entry[V]) {
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

func (c *Cache[V]) moveToFront(e *entry[V]) {
	if c.head == e {
		return
	}
	c.detach(e)
	c.pushFront(e)
}

func (c *Cache[V]) detach(e *entry[V]) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
}

func (c *Cache[V]) remove(e *entry[V]) {
	c.detach(e)
	delete(c.items, e.key)
}

// evictEntry removes an entry and fires the eviction callback.
func (c *Cache[V]) evictEntry(e *entry[V]) {
	key := e.key
	value := e.value
	c.remove(e)
	c.evictions.Add(1)
	if c.onEvict != nil {
		c.onEvict(key, value)
	}
}

func (c *Cache[V]) evict() {
	// Try to evict an expired entry first
	now := time.Now()
	for _, e := range c.items {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			c.evictEntry(e)
			return
		}
	}
	// Otherwise evict LRU
	if c.tail != nil {
		c.evictEntry(c.tail)
	}
}

// GetOrSet returns the cached value for key, or calls factory to compute and cache it.
func (c *Cache[V]) GetOrSet(key string, factory func() V, opts ...SetOption) V {
	if val, ok := c.Get(key); ok {
		return val
	}
	val := factory()
	c.Set(key, val, opts...)
	return val
}

// GetMany returns all cached values for the given keys. Missing or expired keys are omitted.
func (c *Cache[V]) GetMany(keys []string) map[string]V {
	result := make(map[string]V, len(keys))
	for _, key := range keys {
		if val, ok := c.Get(key); ok {
			result[key] = val
		}
	}
	return result
}

// OnEvict registers a callback that fires when entries are evicted (LRU or TTL).
func (c *Cache[V]) OnEvict(fn func(key string, value V)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvict = fn
}

// Stats returns cache hit, miss, and eviction counters.
func (c *Cache[V]) Stats() CacheStats {
	return CacheStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
	}
}

// DeleteWhere removes all entries matching the predicate. Returns the number of entries removed.
func (c *Cache[V]) DeleteWhere(predicate func(key string, value V) bool) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toRemove []*entry[V]
	for _, e := range c.items {
		if predicate(e.key, e.value) {
			toRemove = append(toRemove, e)
		}
	}
	for _, e := range toRemove {
		c.remove(e)
	}
	return len(toRemove)
}

// SetOption configures a Set call.
type SetOption func(*setConfig)

type setConfig struct {
	ttl  time.Duration
	tags []string
}

// WithTTL overrides the default TTL for this entry.
func WithTTL(d time.Duration) SetOption {
	return func(c *setConfig) { c.ttl = d }
}

// WithTags associates tags with the entry.
func WithTags(tags ...string) SetOption {
	return func(c *setConfig) { c.tags = tags }
}

func toSet(tags []string) map[string]struct{} {
	s := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		s[t] = struct{}{}
	}
	return s
}
