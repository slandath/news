package scrape

import (
	"bytes"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/slandath/news/internal/fixtures"
)

func loadFixture(t *testing.T, name string) *goquery.Document {
	t.Helper()

	body, err := fixtures.Read(name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}

	return doc
}
