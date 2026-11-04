package feeds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	return feeds, nil
}

// SaveUserFeeds writes feeds to path, replacing it atomically enough for a
// single-operator server: write to a temp file in the same directory, then
// rename over the target.
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

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)

		return fmt.Errorf("close user feeds: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)

		return fmt.Errorf("replace user feeds: %w", err)
	}

	return nil
}
