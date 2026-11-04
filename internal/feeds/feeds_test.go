package feeds

import (
	"slices"
	"strings"
	"testing"
)

func TestAllHasTheExpectedFeeds(t *testing.T) {
	if got, want := len(All), 13; got != want {
		t.Fatalf("len(All) = %d, want %d", got, want)
	}
}

// The display order is hardcoded (see the comment on All). This asserts the
// exact shipped order, which is what a localeCompare sort produced — not what
// sort.Strings would produce.
func TestAllIsInDisplayOrder(t *testing.T) {
	want := []string{
		"/Film",
		"AP News",
		"ClickOnDetroit",
		"CNBC",
		"Hacker News",
		"NBC News",
		"PC Gamer",
		"TMZ",
		"NBA",
		"NFL",
		"Tennis",
		"Boxing",
		"F1",
	}

	got := make([]string, len(All))
	for i, feed := range All {
		got[i] = feed.Title
	}

	if !slices.Equal(got, want) {
		t.Errorf("display order = %v, want %v", got, want)
	}
}

// /feed falls back to All[0] when the url parameter is empty or missing, so
// the default feed changing silently would be a user-visible regression.
// News feeds render before Sports feeds, each group in display order, and
// every feed belongs to a known section.
func TestAllSectionsGroupNewsBeforeSports(t *testing.T) {
	news, sports := 0, 0

	for _, feed := range All {
		switch feed.Section {
		case SectionNews:
			if sports > 0 {
				t.Errorf("%q is a news feed after sports feeds, want all news first", feed.Title)
			}
			news++
		case SectionSports:
			sports++
		default:
			t.Errorf("%q has unknown section %q", feed.Title, feed.Section)
		}
	}

	if news != 8 {
		t.Errorf("news feeds = %d, want 8", news)
	}

	if sports != 5 {
		t.Errorf("sports feeds = %d, want 5", sports)
	}
}

func TestFirstFeedIsTheDefault(t *testing.T) {
	if len(All) == 0 {
		t.Fatal("All is empty, so /feed has no default")
	}
	if got, want := All[0].Title, "/Film"; got != want {
		t.Errorf("default feed = %q, want %q", got, want)
	}
}

func TestAllUsesHTTPSOnly(t *testing.T) {
	for _, feed := range All {
		if !strings.HasPrefix(feed.URL, "https://") {
			t.Errorf("%s: url = %q, want https://", feed.Title, feed.URL)
		}
	}
}

func TestAllHasUniqueURLsAndTitles(t *testing.T) {
	urls := make(map[string]string, len(All))
	titles := make(map[string]string, len(All))

	for _, feed := range All {
		if prev, ok := urls[feed.URL]; ok {
			t.Errorf("duplicate url %q shared by %q and %q", feed.URL, prev, feed.Title)
		}
		urls[feed.URL] = feed.Title

		if prev, ok := titles[feed.Title]; ok {
			t.Errorf("duplicate title %q shared by %q and %q", feed.Title, prev, feed.URL)
		}
		titles[feed.Title] = feed.URL
	}
}
