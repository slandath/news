package rss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Parses the hostile-payload fixture with the production parser, offline.
// This pins the field mapping the templates depend on — in particular that
// Published keeps the raw date string, which is what rss-parser exposed as
// item.pubDate — and proves the fixture itself is valid input.
func TestParseFixtureFeed(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "testdata", "fixture.rss"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close fixture: %v", err)
		}
	})

	feed, err := New().Parse(file)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	if diff := cmp.Diff("Fixture Feed & Wire", feed.Title); diff != "" {
		t.Errorf("feed title mismatch (-want +got):\n%s", diff)
	}
	if got, want := len(feed.Items), 2; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}

	first := feed.Items[0]
	if diff := cmp.Diff("Item <script>one</script>", first.Title); diff != "" {
		t.Errorf("item title mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff("https://example.com/story-1", first.Link); diff != "" {
		t.Errorf("item link mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff("Wed, 21 Oct 2026 10:00:00 GMT", first.Published); diff != "" {
		t.Errorf("item published mismatch (-want +got):\n%s", diff)
	}
	if first.PublishedParsed == nil {
		t.Error("PublishedParsed is nil, want a parsed date")
	}

	second := feed.Items[1]
	if diff := cmp.Diff("Item two && more", second.Title); diff != "" {
		t.Errorf("item title mismatch (-want +got):\n%s", diff)
	}
	if second.Description != "" {
		t.Errorf("description = %q, want empty for an item without one", second.Description)
	}
}
