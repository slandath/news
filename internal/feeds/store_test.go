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

func TestLoadUserFeedsRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty title", body: `[{"title":"","url":"https://example.com/a","section":"News"}]`},
		{name: "empty url", body: `[{"title":"A","url":"","section":"News"}]`},
		{name: "whitespace title", body: `[{"title":"   ","url":"https://example.com/a","section":"News"}]`},
		{name: "whitespace url", body: `[{"title":"A","url":"  ","section":"News"}]`},
		{name: "bad section", body: `[{"title":"A","url":"https://example.com/a","section":"Weather"}]`},
		{name: "missing section", body: `[{"title":"A","url":"https://example.com/a"}]`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "feeds.json")
			if err := os.WriteFile(path, []byte(test.body), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			if _, err := LoadUserFeeds(path); err == nil {
				t.Errorf("LoadUserFeeds accepted %s, want an error", test.body)
			}
		})
	}
}

// A failed save leaves the existing file untouched: the temp file is
// created in the target directory, so a staging failure happens before the
// rename. Read-only directories do not stop root, so this runs non-root
// only; root runs exercise the success paths instead.
func TestSaveOverExistingFilePreservesItOnFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directories do not block root")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "feeds.json")
	original := []Feed{
		{Title: "Tennis", URL: "https://feeds.bbci.co.uk/sport/tennis/rss.xml", Section: SectionSports},
	}

	if err := SaveUserFeeds(path, original); err != nil {
		t.Fatalf("SaveUserFeeds: %v", err)
	}

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Errorf("restore dir mode: %v", err)
		}
	})

	if err := SaveUserFeeds(path, []Feed{
		{Title: "Other", URL: "https://example.com/other", Section: SectionNews},
	}); err == nil {
		t.Fatal("SaveUserFeeds into a read-only directory succeeded, want an error")
	}

	got, err := LoadUserFeeds(path)
	if err != nil {
		t.Fatalf("LoadUserFeeds: %v", err)
	}

	if diff := cmp.Diff(original, got); diff != "" {
		t.Errorf("existing file changed after failed save (-want +got):\n%s", diff)
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
