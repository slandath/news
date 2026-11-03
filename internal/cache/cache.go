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
	// maxEntries bounds the map when positive. Zero means unbounded and is
	// only safe when the key space is closed, since expired entries for keys
	// that are never read again are otherwise reaped only under pressure.
	maxEntries int
	now        func() time.Time
}

// New returns an empty unbounded Cache whose entries expire ttl after being
// set. Prefer NewWithLimit when keys are caller-controlled.
func New[V any](ttl time.Duration) *Cache[V] {
	return NewWithClock[V](ttl, time.Now)
}

// NewWithClock returns an empty unbounded Cache that reads the current time
// from now. It exists so tests can control expiry deterministically.
func NewWithClock[V any](ttl time.Duration, now func() time.Time) *Cache[V] {
	return &Cache[V]{
		items: make(map[string]entry[V]),
		ttl:   ttl,
		now:   now,
	}
}

// SetMaxEntries caps the number of entries at n. A non-positive n means
// unbounded, which is only safe when the key space is closed. The cap is
// enforced on Set: expired entries are swept first, then the entries
// expiring soonest are evicted until within the limit.
func (c *Cache[V]) SetMaxEntries(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maxEntries = n
}

// Get returns the cached value and its remaining lifetime, or false when the
// key is absent or its entry has expired. Expired entries are removed. The
// remaining lifetime is always positive on a hit.
func (c *Cache[V]) Get(key string) (V, time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		var zero V

		return zero, 0, false
	}

	remaining := e.expires.Sub(c.now())
	if remaining <= 0 {
		delete(c.items, key)

		var zero V

		return zero, 0, false
	}

	return e.value, remaining, true
}

// Set stores value under key, expiring ttl after this call. When maxEntries
// is positive and the map is over the limit, expired entries are swept first
// and then the entries expiring soonest are evicted until within the limit.
func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = entry[V]{value: value, expires: c.now().Add(c.ttl)}

	if c.maxEntries > 0 {
		c.evictLocked()
	}
}

// evictLocked drops expired entries, then the soonest-expiring ones, until
// the map fits maxEntries. The caller must hold c.mu.
func (c *Cache[V]) evictLocked() {
	now := c.now()
	for key, e := range c.items {
		if !now.Before(e.expires) {
			delete(c.items, key)
		}
	}

	for len(c.items) > c.maxEntries {
		earliest := ""
		var expires time.Time
		first := true

		for key, e := range c.items {
			if first || e.expires.Before(expires) {
				earliest, expires, first = key, e.expires, false
			}
		}

		delete(c.items, earliest)
	}
}

// Clear drops every entry.
func (c *Cache[V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]entry[V])
}
