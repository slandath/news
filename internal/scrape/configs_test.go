package scrape

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-cmp/cmp"
)

// articleFixtures are the hostile-payload fixtures from tests/fixtures.ts,
// saved verbatim under testdata/. The expected values are what cheerio's
// .text() produced for them: markup gone, inner text kept.
//
// They are written out literally rather than derived from the fixtures so a
// selector that starts pulling in markup fails loudly instead of agreeing with
// whatever the wrapper happens to return.
var articleFixtures = map[string]struct {
	file       string
	title      string
	paragraphs []string
}{
	"clickondetroit.com": {
		file:       "clickondetroit.html",
		title:      "alert(1)Boy rescues cat",
		paragraphs: []string{"The kitten is alert(1)doing fine.", "Neighbors are relieved."},
	},
	"cnbc.com": {
		file:       "cnbc.html",
		title:      "Stocks alert(1)rally on earnings",
		paragraphs: []string{"Futures point higher alert(1)at the open.", "Analysts expect a quiet week."},
	},
	"nbcnews.com": {
		file:       "nbcnews.html",
		title:      "Storm alert(1)heads north",
		paragraphs: []string{"Residents hunkered down overnight.", "Power was restored by morning."},
	},
	"pcgamer.com": {
		file:       "pcgamer.html",
		title:      "Best rigs of alert(1)2026",
		paragraphs: []string{"The new cards are alert(1)remarkably cool.", "Prices may drop next quarter."},
	},
	"slashfilm.com": {
		file:       "slashfilm.html",
		title:      "Sequels alert(1)return this fall",
		paragraphs: []string{"Production wrapped alert(1)last week.", "The trailer drops Friday."},
	},
}

func TestConfigsCoverExactlyTheScrapingSites(t *testing.T) {
	got := make([]string, 0, len(Configs))
	for domain := range Configs {
		got = append(got, domain)
	}
	sort.Strings(got)

	want := []string{
		"clickondetroit.com",
		"cnbc.com",
		"nbcnews.com",
		"pcgamer.com",
		"slashfilm.com",
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("configured domains mismatch (-want +got):\n%s", diff)
	}
}

func TestConfigsIgnoreOutletsWithoutConfigs(t *testing.T) {
	for _, domain := range []string{"tmz.com", "news.ycombinator.com", "news.google.com"} {
		if _, ok := Configs[domain]; ok {
			t.Errorf("Configs[%q] exists, want no config (these rely on the redirect fallback)", domain)
		}
	}
}

func TestConfigsExtractTitleAndParagraphs(t *testing.T) {
	for domain, fx := range articleFixtures {
		t.Run(domain, func(t *testing.T) {
			cfg, ok := Configs[domain]
			if !ok {
				t.Fatalf("no config for %s", domain)
			}

			doc := loadFixture(t, fx.file)
			title := doc.Find(cfg.Title).Text()
			article := cfg.ArticleWrapper(doc)

			if diff := cmp.Diff(fx.title, title); diff != "" {
				t.Errorf("title mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(fx.paragraphs, article); diff != "" {
				t.Errorf("paragraphs mismatch (-want +got):\n%s", diff)
			}
			if len(article) != 2 {
				t.Errorf("len(paragraphs) = %d, want 2", len(article))
			}
		})
	}
}

// Article and ArticleWrapper each carry a selector. They must stay in sync, or
// callers that probe Article would disagree with what actually gets rendered.
func TestArticleSelectorMatchesWrapper(t *testing.T) {
	for domain, fx := range articleFixtures {
		t.Run(domain, func(t *testing.T) {
			cfg := Configs[domain]
			doc := loadFixture(t, fx.file)

			if got, want := doc.Find(cfg.Article).Length(), len(cfg.ArticleWrapper(doc)); got != want {
				t.Errorf("Article %q matches %d elements, wrapper returned %d", cfg.Article, got, want)
			}
		})
	}
}

var tagPattern = regexp.MustCompile(`(?i)</?[a-z][^>]*>`)

// This is the security regression test: no markup, no event handlers, no
// script bodies-with-tags may survive extraction.
func TestConfigsStripAllMarkup(t *testing.T) {
	for domain, fx := range articleFixtures {
		t.Run(domain, func(t *testing.T) {
			cfg := Configs[domain]
			doc := loadFixture(t, fx.file)

			extracted := append([]string{doc.Find(cfg.Title).Text()}, cfg.ArticleWrapper(doc)...)
			for _, text := range extracted {
				if tagPattern.MatchString(text) {
					t.Errorf("markup survived extraction: %q", text)
				}
				if strings.Contains(text, "onerror=") {
					t.Errorf("event handler survived extraction: %q", text)
				}
			}
		})
	}
}

func loadFixture(t *testing.T, name string) *goquery.Document {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}

	return doc
}
