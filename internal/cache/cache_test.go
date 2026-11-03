package cache_test

import (
	"sync"
	"testing"
	"time"

	"github.com/slandath/news/internal/cache"
)

func TestGetMissingKeyMisses(t *testing.T) {
	c := cache.New[string](time.Minute)

	if _, ok := c.Get("nope"); ok {
		t.Error("Get on empty cache hit, want miss")
	}
}

func TestSetThenGetHits(t *testing.T) {
	c := cache.New[string](time.Minute)
	c.Set("a", "1")
	c.Set("b", "2")

	if got, ok := c.Get("a"); !ok || got != "1" {
		t.Errorf("Get(a) = %q, %v; want %q, true", got, ok, "1")
	}

	if got, ok := c.Get("b"); !ok || got != "2" {
		t.Errorf("Get(b) = %q, %v; want %q, true", got, ok, "2")
	}
}

func TestSetOverwritesValue(t *testing.T) {
	c := cache.New[string](time.Minute)
	c.Set("a", "1")
	c.Set("a", "2")

	if got, ok := c.Get("a"); !ok || got != "2" {
		t.Errorf("Get(a) = %q, %v; want %q, true", got, ok, "2")
	}
}

func TestExpiredEntryMisses(t *testing.T) {
	now := time.Now()
	c := cache.NewWithClock[string](time.Minute, func() time.Time { return now })

	c.Set("a", "1")

	now = now.Add(time.Minute)

	if _, ok := c.Get("a"); ok {
		t.Error("Get after TTL hit, want miss")
	}
}

func TestClearDropsEntries(t *testing.T) {
	c := cache.New[string](time.Minute)
	c.Set("a", "1")
	c.Clear()

	if _, ok := c.Get("a"); ok {
		t.Error("Get after Clear hit, want miss")
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	c := cache.New[int](time.Minute)

	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)

		go func() {
			defer wg.Done()
			c.Set("k", i)
			_, _ = c.Get("k")
		}()
	}

	wg.Wait()
}
