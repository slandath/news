package feeds

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadUserFeedsMissingFileIsEmpty(t *testing.T) {
	got, err := LoadUserFeeds(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("LoadUserFeeds: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("loaded %d feeds, want none", len(got))
	}
}

func TestLoadUserFeedsRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feeds.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := LoadUserFeeds(path); err == nil {
		t.Error("LoadUserFeeds accepted corrupt JSON, want an error")
	}
}

func TestSaveAndLoadUserFeedsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feeds.json")
	want := []Feed{
		{Title: "Tennis", URL: "https://feeds.bbci.co.uk/sport/tennis/rss.xml", Section: SectionSports},
		{Title: "Custom", URL: "https://example.com/feed", Section: SectionNews},
	}

	if err := SaveUserFeeds(path, want); err != nil {
		t.Fatalf("SaveUserFeeds: %v", err)
	}

	got, err := LoadUserFeeds(path)
	if err != nil {
		t.Fatalf("LoadUserFeeds: %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("round trip mismatch (-want +got):\n%s", diff)
	}
}
