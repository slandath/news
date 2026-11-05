package feeds

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadUserFeeds reads user-added feeds from path. A missing file means an
// empty list, not an error. A corrupt file is an error: failing startup
// loudly beats silently dropping feeds.
func LoadUserFeeds(path string) ([]Feed, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read user feeds: %w", err)
	}

	var feeds []Feed

	if err := json.Unmarshal(body, &feeds); err != nil {
		return nil, fmt.Errorf("parse user feeds: %w", err)
	}

	for i, feed := range feeds {
		if strings.TrimSpace(feed.Title) == "" || strings.TrimSpace(feed.URL) == "" {
			return nil, fmt.Errorf("user feed %d: title and url are required", i)
		}

		if feed.Section != SectionNews && feed.Section != SectionSports {
			return nil, fmt.Errorf("user feed %d: section must be %q or %q", i, SectionNews, SectionSports)
		}
	}

	return feeds, nil
}

// SaveUserFeeds writes feeds to path. The write is atomic enough for a
// single server instance — temp file in the same directory, synced, then
// renamed over the target — but it is NOT safe for concurrent writers:
// each save builds from its own in-memory list, so two processes sharing
// one file can overwrite each other's feeds. Run a single instance per
// FEEDS_FILE.
func SaveUserFeeds(path string, feeds []Feed) error {
	body, err := json.MarshalIndent(feeds, "", "  ")
	if err != nil {
		return fmt.Errorf("encode user feeds: %w", err)
	}

	// The temp file lives beside the target so the rename never crosses
	// filesystems.
	tmp, err := os.CreateTemp(filepath.Dir(path), "user-feeds-*.json")
	if err != nil {
		return fmt.Errorf("stage user feeds: %w", err)
	}

	tmpName := tmp.Name()

	if _, err := tmp.Write(append(body, '\n')); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)

		return fmt.Errorf("write user feeds: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)

		return fmt.Errorf("sync user feeds: %w", err)
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)

		return fmt.Errorf("close user feeds: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)

		return fmt.Errorf("replace user feeds: %w", err)
	}

	// The rename has committed. Sync the directory so the entry survives a
	// crash, but only log failures: the file is already replaced, so an
	// error here must not fail the save and desync memory from disk. The
	// sync can also fail on platforms that do not support it.
	if dir, err := os.Open(filepath.Dir(path)); err != nil {
		slog.Warn("open user feeds dir", "err", err)
	} else {
		if err := dir.Sync(); err != nil {
			slog.Warn("sync user feeds dir", "err", err)
		}

		_ = dir.Close()
	}

	return nil
}
