package templates

import (
	"bytes"
	"context"
	htmlpkg "html"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"
)

func render(t *testing.T, component templ.Component) string {
	t.Helper()

	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	return buf.String()
}

func feedHref(rawURL string) string {
	return "/feed?" + url.Values{"url": {rawURL}}.Encode()
}

func articleHref(rawURL, feedURL string) string {
	return "/article?" + url.Values{"url": {rawURL}, "feed": {feedURL}}.Encode()
}

var hrefPattern = regexp.MustCompile(`href="([^"]*)"`)

// hrefs returns the href attribute values in document order, entity-decoded,
// so assertions compare against real URLs rather than HTML-escaped ones.
func hrefs(body string) []string {
	matches := hrefPattern.FindAllStringSubmatch(body, -1)

	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, htmlpkg.UnescapeString(match[1]))
	}

	return out
}

// navHrefs returns only root-relative hrefs, ignoring stylesheet links.
func navHrefs(body string) []string {
	var out []string
	for _, href := range hrefs(body) {
		if strings.HasPrefix(href, "/") {
			out = append(out, href)
		}
	}

	return out
}

func articleProps() ArticleProps {
	return ArticleProps{
		Title:      "Headline <script>alert(1)</script>",
		Paragraphs: []string{"Paragraph with <script>alert(1)</script> inline", "Plain paragraph"},
		SourceURL:  "https://example.com/story",
		FeedTitle:  "CNBC",
		FeedHref:   feedHref("https://example.com/cnbc/rss"),
	}
}

func TestArticleRendersDocument(t *testing.T) {
	body := render(t, Article(articleProps()))

	for _, want := range []string{
		"<!doctype html>",
		`<p class="para">Paragraph with &lt;script&gt;alert(1)&lt;/script&gt; inline</p>`,
		"Plain paragraph",
		`href="https://example.com/story"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered article is missing %q", want)
		}
	}
}

func TestArticleEscapesTitle(t *testing.T) {
	body := render(t, Article(articleProps()))

	if !strings.Contains(body, "Headline &lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("title was not escaped")
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("raw script tag survived rendering")
	}
}

func TestArticleTruncatesTitle(t *testing.T) {
	longTitle := strings.Repeat("VeryLongWord", 6) + " <script>alert(1)</script>"

	props := articleProps()
	props.Title = longTitle
	body := render(t, Article(props))

	// The card title truncates at 60 runes.
	if want := truncate(longTitle, 60) + "</h1>"; !strings.Contains(body, want) {
		t.Errorf("article title is missing %q", want)
	}
}

func TestArticleOmitsFeedLinkWithoutFeed(t *testing.T) {
	props := articleProps()
	props.FeedTitle = ""
	props.FeedHref = ""

	if body := render(t, Article(props)); strings.Contains(body, "/feed?") {
		t.Error("article rendered a feed link without a feed")
	}
}

func feedProps() FeedProps {
	return FeedProps{
		Title: "Fixture Feed",
		Items: []FeedItem{
			{
				Title:     "First <script>alert(1)</script> item",
				Href:      articleHref("https://example.com/story-1", "https://example.com/cnbc/rss"),
				Published: "Wed, 21 Oct 2026 10:00:00 GMT",
			},
			{
				Title:     "Link-less item",
				Published: "Wed, 21 Oct 2026 11:00:00 GMT",
			},
		},
	}
}

func TestFeedRendersTitleAndLinkedItems(t *testing.T) {
	body := render(t, Feed(feedProps()))

	if !strings.Contains(body, "Fixture Feed") {
		t.Error("feed title is missing")
	}

	var found bool
	for _, href := range hrefs(body) {
		if !strings.HasPrefix(href, "/article?") {
			continue
		}
		found = true

		parsed, err := url.Parse(href)
		if err != nil {
			t.Fatalf("parse href %q: %v", href, err)
		}
		if got, want := parsed.Query().Get("url"), "https://example.com/story-1"; got != want {
			t.Errorf("url param = %q, want %q", got, want)
		}
		if got, want := parsed.Query().Get("feed"), "https://example.com/cnbc/rss"; got != want {
			t.Errorf("feed param = %q, want %q", got, want)
		}
	}

	if !found {
		t.Fatal("no /article href rendered")
	}
}

func TestFeedRendersLinklessItemAsText(t *testing.T) {
	body := render(t, Feed(feedProps()))

	if !strings.Contains(body, "Link-less item") {
		t.Error("link-less item title is missing")
	}
	if got, want := strings.Count(body, `href="/article?`), 1; got != want {
		t.Errorf("rendered %d article anchors, want %d", got, want)
	}
}

func TestFeedEscapesHostileItemTitles(t *testing.T) {
	body := render(t, Feed(feedProps()))

	if !strings.Contains(body, "First &lt;script&gt;alert(1)&lt;/script&gt; item") {
		t.Error("item title was not escaped")
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("raw script tag survived rendering")
	}
}

func homeProps() HomeProps {
	return HomeProps{Feeds: []FeedLink{
		{Title: "CNBC & Markets", Href: feedHref("https://example.com/cnbc/rss")},
		{Title: "Hacker News <script>alert(1)</script>", Href: feedHref("https://news.ycombinator.com/rss")},
	}}
}

func TestHomeRendersEveryFeedLink(t *testing.T) {
	body := render(t, Home(homeProps()))

	if !strings.Contains(body, "News Feeds") {
		t.Error("page heading is missing")
	}

	want := []string{
		"/",
		feedHref("https://example.com/cnbc/rss"),
		feedHref("https://news.ycombinator.com/rss"),
	}
	if diff := cmp.Diff(want, navHrefs(body)); diff != "" {
		t.Errorf("hrefs mismatch (-want +got):\n%s", diff)
	}
	if !strings.Contains(body, "CNBC &amp; Markets") {
		t.Error("feed title was not escaped")
	}
}

func TestHomeEscapesHostileFeedTitles(t *testing.T) {
	body := render(t, Home(homeProps()))

	if !strings.Contains(body, "Hacker News &lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("feed title was not escaped")
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("raw script tag survived rendering")
	}
}

func TestErrorRendersEscapedMessage(t *testing.T) {
	body := render(t, Error(ErrorProps{
		Title:   "Failure",
		Message: "Bad things <script>alert(1)</script> happened",
	}))

	if !strings.Contains(body, "<h1>Failure</h1>") {
		t.Error("error title is missing")
	}
	if !strings.Contains(body, "Bad things &lt;script&gt;alert(1)&lt;/script&gt; happened") {
		t.Error("error message was not escaped")
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("raw script tag survived rendering")
	}
}

func TestTruncateCountsRunes(t *testing.T) {
	tests := []struct {
		name string
		text string
		max  int
		want string
	}{
		{name: "shorter than max", text: "short", max: 30, want: "short"},
		{name: "exactly max", text: "12345", max: 5, want: "12345"},
		{name: "empty", text: "", max: 30, want: ""},
		{name: "ascii over max", text: strings.Repeat("a", 31), max: 30, want: strings.Repeat("a", 30) + "..."},
		{name: "multibyte over max", text: strings.Repeat("日", 31), max: 30, want: strings.Repeat("日", 30) + "..."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := truncate(test.text, test.max)
			if got != test.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", test.text, test.max, got, test.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncate produced invalid UTF-8: %q", got)
			}
		})
	}
}

// SiteStyle must set an explicit page background: some pages declare a
// color-scheme, and dark text without a background is unreadable when the
// browser paints a dark canvas.
func TestSiteStyleSetsPageBackground(t *testing.T) {
	body := render(t, SiteStyle())

	if !strings.Contains(body, "background: #e9edf1;") {
		t.Error("shared style is missing an explicit page background")
	}
}
