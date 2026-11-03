package cache_test

import (
	"sync"
	"testing"
	"time"

	"github.com/slandath/news/internal/cache"
)

func TestGetMissingKeyMisses(t *testing.T) {
	c := cache.New[string](time.Minute)

	if _, _, ok := c.Get("nope"); ok {
		t.Error("Get on empty cache hit, want miss")
	}
}

func TestSetThenGetHits(t *testing.T) {
	c := cache.New[string](time.Minute)
	c.Set("a", "1")
	c.Set("b", "2")

	if got, _, ok := c.Get("a"); !ok || got != "1" {
		t.Errorf("Get(a) = %q, %v; want %q, true", got, ok, "1")
	}

	if got, _, ok := c.Get("b"); !ok || got != "2" {
		t.Errorf("Get(b) = %q, %v; want %q, true", got, ok, "2")
	}
}

func TestGetReportsRemainingLifetime(t *testing.T) {
	now := time.Now()
	c := cache.NewWithClock[string](time.Minute, func() time.Time { return now })
	c.Set("a", "1")

	now = now.Add(30 * time.Second)

	_, remaining, ok := c.Get("a")
	if !ok {
		t.Fatal("Get missed, want hit")
	}

	if got, want := remaining, 30*time.Second; got != want {
		t.Errorf("remaining = %v, want %v", got, want)
	}
}

func TestSetOverwritesValue(t *testing.T) {
	c := cache.New[string](time.Minute)
	c.Set("a", "1")
	c.Set("a", "2")

	if got, _, ok := c.Get("a"); !ok || got != "2" {
		t.Errorf("Get(a) = %q, %v; want %q, true", got, ok, "2")
	}
}

func TestExpiredEntryMisses(t *testing.T) {
	now := time.Now()
	c := cache.NewWithClock[string](time.Minute, func() time.Time { return now })

	c.Set("a", "1")

	now = now.Add(time.Minute)

	if _, _, ok := c.Get("a"); ok {
		t.Error("Get after TTL hit, want miss")
	}
}

func TestLimitEvictsSoonestExpiring(t *testing.T) {
	now := time.Now()
	c := cache.NewWithClock[string](time.Minute, func() time.Time { return now })
	c.SetMaxEntries(2)

	c.Set("a", "1")

	now = now.Add(time.Second)
	c.Set("b", "2")

	now = now.Add(time.Second)
	c.Set("c", "3")

	if _, _, ok := c.Get("a"); ok {
		t.Error("Get(a) hit, want the soonest-expiring entry evicted")
	}

	for _, key := range []string{"b", "c"} {
		if _, _, ok := c.Get(key); !ok {
			t.Errorf("Get(%s) missed, want hit", key)
		}
	}
}

func TestLimitSweepsExpiredFirst(t *testing.T) {
	now := time.Now()
	c := cache.NewWithClock[string](time.Minute, func() time.Time { return now })
	c.SetMaxEntries(2)

	c.Set("a", "1")
	c.Set("b", "2")

	now = now.Add(time.Minute)
	c.Set("c", "3")

	if _, _, ok := c.Get("a"); ok {
		t.Error("Get(a) hit, want the expired entry swept")
	}

	if _, _, ok := c.Get("b"); ok {
		t.Error("Get(b) hit, want the expired entry swept")
	}

	if got, _, ok := c.Get("c"); !ok || got != "3" {
		t.Errorf("Get(c) = %q, %v; want %q, true", got, ok, "3")
	}
}

func TestClearDropsEntries(t *testing.T) {
	c := cache.New[string](time.Minute)
	c.Set("a", "1")
	c.Clear()

	if _, _, ok := c.Get("a"); ok {
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
			_, _, _ = c.Get("k")
		}()
	}

	wg.Wait()
}
