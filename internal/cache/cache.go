// Package cache provides a small generic in-memory TTL cache.
package cache

import (
	"sync"
	"time"
)

// entry is one cached value with its expiry time.
type entry[V any] struct {
	value   V
	expires time.Time
}

// Cache is a goroutine-safe map of string keys to values that expire after
// a fixed TTL. Expiry is lazy: stale entries are dropped on read. There is
// no background cleanup goroutine.
type Cache[V any] struct {
	mu    sync.Mutex
	items map[string]entry[V]
	ttl   time.Duration
	now   func() time.Time
}

// New returns an empty Cache whose entries expire ttl after being set.
func New[V any](ttl time.Duration) *Cache[V] {
	return NewWithClock[V](ttl, time.Now)
}

// NewWithClock returns an empty Cache that reads the current time from now.
// It exists so tests can control expiry deterministically.
func NewWithClock[V any](ttl time.Duration, now func() time.Time) *Cache[V] {
	return &Cache[V]{
		items: make(map[string]entry[V]),
		ttl:   ttl,
		now:   now,
	}
}

// Get returns the cached value for key, or false when the key is absent or
// its entry has expired. Expired entries are removed.
func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		var zero V

		return zero, false
	}

	if !c.now().Before(e.expires) {
		delete(c.items, key)

		var zero V

		return zero, false
	}

	return e.value, true
}

// Set stores value under key, expiring ttl after this call.
func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = entry[V]{value: value, expires: c.now().Add(c.ttl)}
}

// Clear drops every entry.
func (c *Cache[V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]entry[V])
}
