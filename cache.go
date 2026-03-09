// Package cachekit provides a generic in-memory LRU cache with TTL and tag-based invalidation.
package cachekit

import (
	"sync"
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

// Cache is a thread-safe in-memory LRU cache with TTL and tag support.
type Cache[V any] struct {
	mu         sync.RWMutex
	items      map[string]*entry[V]
	head       *entry[V]
	tail       *entry[V]
	maxSize    int
	defaultTTL time.Duration
}

// New creates a new Cache with the given max size and default TTL.
// A zero TTL means entries don't expire by default.
func New[V any](maxSize int, defaultTTL time.Duration) *Cache[V] {
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
		var zero V
		return zero, false
	}

	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		c.remove(e)
		var zero V
		return zero, false
	}

	c.moveToFront(e)
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
	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *Cache[V]) evict() {
	// Try to evict an expired entry first
	now := time.Now()
	for _, e := range c.items {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			c.remove(e)
			return
		}
	}
	// Otherwise evict LRU
	if c.tail != nil {
		c.remove(c.tail)
	}
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
