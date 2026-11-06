package scrape

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/slandath/news/internal/fixtures"
)

func TestConfigsCoverExactlyTheScrapingSites(t *testing.T) {
	got := make([]string, 0, len(Configs))
	for domain := range Configs {
		got = append(got, domain)
	}
	sort.Strings(got)

	want := []string{
		"bbc.co.uk",
		"clickondetroit.com",
		"cnbc.com",
		"espn.com",
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
	for domain, fx := range fixtures.Articles {
		t.Run(domain, func(t *testing.T) {
			cfg, ok := Configs[domain]
			if !ok {
				t.Fatalf("no config for %s", domain)
			}

			doc := loadFixture(t, fx.File)
			title := doc.Find(cfg.Title).Text()
			article := texts(doc, cfg.Article)

			if diff := cmp.Diff(fx.Title, title); diff != "" {
				t.Errorf("title mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(fx.Paragraphs, article); diff != "" {
				t.Errorf("paragraphs mismatch (-want +got):\n%s", diff)
			}
			if len(article) != 2 {
				t.Errorf("len(paragraphs) = %d, want 2", len(article))
			}
		})
	}
}

var tagPattern = regexp.MustCompile(`(?i)</?[a-z][^>]*>`)

// This is the security regression test: no markup, no event handlers, no
// script bodies-with-tags may survive extraction.
func TestConfigsStripAllMarkup(t *testing.T) {
	for domain, fx := range fixtures.Articles {
		t.Run(domain, func(t *testing.T) {
			cfg := Configs[domain]
			doc := loadFixture(t, fx.File)

			extracted := append([]string{doc.Find(cfg.Title).Text()}, texts(doc, cfg.Article)...)
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

// Every config needs a fixture and every fixture needs a config, so adding a
// scrape target without test coverage — or deleting a config and leaving its
// fixture behind — fails here.
func TestConfigsAndFixturesPair(t *testing.T) {
	for domain := range Configs {
		if _, ok := fixtures.Articles[domain]; !ok {
			t.Errorf("config for %s has no testdata fixture", domain)
		}
	}
	for domain := range fixtures.Articles {
		if _, ok := Configs[domain]; !ok {
			t.Errorf("fixture for %s has no config", domain)
		}
	}
}
