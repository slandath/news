package scrape

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// readFixture returns the raw bytes of a hostile-payload fixture from
// testdata/, shared by the extraction and fetch tests.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	return body
}

func loadFixture(t *testing.T, name string) *goquery.Document {
	t.Helper()

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(readFixture(t, name)))
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}

	return doc
}
