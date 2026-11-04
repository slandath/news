package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	htmlpkg "html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/slandath/news/internal/app"
	"github.com/slandath/news/internal/feeds"
	"github.com/slandath/news/internal/fixtures"
	"github.com/slandath/news/internal/rss"
	"github.com/slandath/news/internal/scrape"
)

// stubTransport stands in for the network. It records every request so tests
// can assert what the handlers asked for, and serves the testdata corpus by
// default — so the real gofeed parser and the real scraper both run.
type stubTransport struct {
	handle   func(*http.Request) (*http.Response, error)
	requests []string
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.requests = append(s.requests, req.URL.String())

	return s.handle(req)
}

func TestMain(m *testing.M) {
	// Handlers log every request; keep the test output readable.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func newStubTransport(t *testing.T) *stubTransport {
	t.Helper()

	rssBody, err := fixtures.Read(fixtures.RSS)
	if err != nil {
		t.Fatalf("read %s: %v", fixtures.RSS, err)
	}

	articles := make(map[string][]byte, len(fixtures.Articles))
	for domain, article := range fixtures.Articles {
		body, err := fixtures.Read(article.File)
		if err != nil {
			t.Fatalf("read %s: %v", article.File, err)
		}
		articles[domain] = body
	}

	feedURLs := make(map[string]struct{}, len(feeds.All))
	for _, feed := range feeds.All {
		feedURLs[feed.URL] = struct{}{}
	}

	return &stubTransport{handle: func(req *http.Request) (*http.Response, error) {
		var (
			body []byte
			ok   bool
		)

		// Feed URLs are matched first: slashfilm's feed shares its host with
		// its article fixture.
		if _, isFeed := feedURLs[req.URL.String()]; isFeed {
			body, ok = rssBody, true
		} else {
			body, ok = articles[strings.TrimPrefix(req.URL.Hostname(), "www.")]
		}
		if !ok {
			return nil, fmt.Errorf("no fixture for %s", req.URL)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    req,
		}, nil
	}}
}

func newTestHandler(t *testing.T, transport http.RoundTripper) http.Handler {
	t.Helper()

	parser := rss.New()
	parser.Client = &http.Client{Transport: transport}

	return app.New(app.Deps{
		Feeds:   feeds.All,
		Configs: scrape.Configs,
		Parser:  parser,
		Scraper: scrape.New(&http.Client{Transport: transport}),
		Favicon: []byte("icon"),
	})
}

func do(t *testing.T, handler http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil))

	return rec
}

var hrefPattern = regexp.MustCompile(`href="([^"]*)"`)

// hrefs returns href attribute values in document order, entity-decoded, so
// assertions compare against real URLs rather than HTML-escaped ones.
func hrefs(body string) []string {
	matches := hrefPattern.FindAllStringSubmatch(body, -1)

	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, htmlpkg.UnescapeString(match[1]))
	}

	return out
}

func feedHref(feedURL string) string {
	return "/feed?" + url.Values{"url": {feedURL}}.Encode()
}

func feedByTitle(t *testing.T, title string) feeds.Feed {
	t.Helper()

	for _, feed := range feeds.All {
		if feed.Title == title {
			return feed
		}
	}

	t.Fatalf("no feed titled %q", title)

	return feeds.Feed{}
}

func TestHomeListsEveryFeed(t *testing.T) {
	handler := newTestHandler(t, newStubTransport(t))
	rec := do(t, handler, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "News Feeds") {
		t.Error("home page is missing its heading")
	}
	for _, feed := range feeds.All {
		if want := `href="` + feedHref(feed.URL) + `"`; !strings.Contains(body, want) {
			t.Errorf("home page is missing %s", want)
		}
	}
}

func TestFeedFetchesRequestedFeed(t *testing.T) {
	transport := newStubTransport(t)
	handler := newTestHandler(t, transport)
	feedURL := feeds.All[0].URL

	rec := do(t, handler, "/feed?url="+url.QueryEscape(feedURL))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(transport.requests) != 1 || transport.requests[0] != feedURL {
		t.Errorf("fetched %v, want exactly [%s]", transport.requests, feedURL)
	}

	body := rec.Body.String()

	// The RSS fixture's first item title is hostile; the real parser decodes
	// it and the template must escape it.
	if !strings.Contains(body, "Item &lt;script&gt;one&lt;/script&gt;") {
		t.Error("item title was not escaped")
	}
	if !strings.Contains(body, "Item two &amp;&amp; more") {
		t.Error("second item title is missing")
	}
	if strings.Contains(body, "<script>") {
		t.Error("raw script tag survived rendering")
	}

	// A configured feed's title wins over the feed's own <title>.
	if !strings.Contains(body, "<title>/Film</title>") {
		t.Error("configured feed title did not override the feed's own title")
	}

	articleURLs := make(map[string]bool, 2)
	for _, href := range hrefs(body) {
		if !strings.HasPrefix(href, "/article?") {
			continue
		}

		parsed, err := url.Parse(href)
		if err != nil {
			t.Fatalf("parse href %q: %v", href, err)
		}
		if got, want := parsed.Query().Get("feed"), feedURL; got != want {
			t.Errorf("feed param = %q, want %q", got, want)
		}
		articleURLs[parsed.Query().Get("url")] = true
	}

	for _, want := range []string{"https://example.com/story-1", "https://example.com/story-2"} {
		if !articleURLs[want] {
			t.Errorf("no /article link for %s", want)
		}
	}
}

func TestFeedDefaultsToFirstConfiguredFeed(t *testing.T) {
	transport := newStubTransport(t)
	handler := newTestHandler(t, transport)

	rec := do(t, handler, "/feed")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if want := feeds.All[0].URL; len(transport.requests) != 1 || transport.requests[0] != want {
		t.Errorf("fetched %v, want exactly [%s]", transport.requests, want)
	}
}

func TestFeedFallsBackToFirstFeedWhenURLIsEmpty(t *testing.T) {
	transport := newStubTransport(t)
	handler := newTestHandler(t, transport)

	rec := do(t, handler, "/feed?url=")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (empty url falls back, it is not a 400)", rec.Code, http.StatusOK)
	}
	if want := feeds.All[0].URL; len(transport.requests) != 1 || transport.requests[0] != want {
		t.Errorf("fetched %v, want exactly [%s]", transport.requests, want)
	}
}

func TestFeedReturns502WhenParserFails(t *testing.T) {
	transport := &stubTransport{handle: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("feed unavailable")
	}}
	handler := newTestHandler(t, transport)

	rec := do(t, handler, "/feed?url="+url.QueryEscape(feeds.All[0].URL))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if !strings.Contains(rec.Body.String(), "Bad Feed") {
		t.Error("error page is missing \"Bad Feed\"")
	}
}

func TestFeedReturns400WhenFeedListIsEmpty(t *testing.T) {
	// Defensive branch: unreachable while feeds.All is non-empty, but kept for
	// parity with the TypeScript app.
	handler := app.New(app.Deps{
		Parser:  rss.New(),
		Scraper: scrape.New(&http.Client{Transport: newStubTransport(t)}),
	})

	rec := do(t, handler, "/feed")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got, want := rec.Body.String(), "URL undefined"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestFeedRejectsUnconfiguredFeedURL(t *testing.T) {
	transport := newStubTransport(t)
	handler := newTestHandler(t, transport)
	notInList := "https://example.com/feed.xml"

	rec := do(t, handler, "/feed?url="+url.QueryEscape(notInList))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got, want := rec.Body.String(), "Invalid feed URL"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if len(transport.requests) != 0 {
		t.Errorf("made %d outbound requests, want none", len(transport.requests))
	}
}

func TestArticleRendersEachConfiguredSite(t *testing.T) {
	for domain, article := range fixtures.Articles {
		t.Run(domain, func(t *testing.T) {
			transport := newStubTransport(t)
			handler := newTestHandler(t, transport)
			source := "https://www." + domain + "/story"

			rec := do(t, handler, "/article?url="+url.QueryEscape(source))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if len(transport.requests) != 1 || transport.requests[0] != source {
				t.Errorf("fetched %v, want exactly [%s]", transport.requests, source)
			}

			body := rec.Body.String()
			if !strings.Contains(body, article.Title) {
				t.Errorf("article is missing title %q", article.Title)
			}
			for _, para := range article.Paragraphs {
				if !strings.Contains(body, para) {
					t.Errorf("article is missing paragraph %q", para)
				}
			}
			for _, banned := range []string{"<script>", "onerror=", "<img"} {
				if strings.Contains(body, banned) {
					t.Errorf("response contains %q", banned)
				}
			}
		})
	}
}

func TestArticleLinksBackToOriginatingFeed(t *testing.T) {
	handler := newTestHandler(t, newStubTransport(t))
	cnbc := feedByTitle(t, "CNBC")

	target := "/article?" + url.Values{
		"url":  {"https://www.cnbc.com/story"},
		"feed": {cnbc.URL},
	}.Encode()

	rec := do(t, handler, target)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, feedHref(cnbc.URL)) {
		t.Error("article is missing the feed breadcrumb link")
	}
	if !strings.Contains(body, "CNBC") {
		t.Error("article is missing the feed title")
	}
}

func TestArticleReturns400WhenURLIsMissing(t *testing.T) {
	transport := newStubTransport(t)
	handler := newTestHandler(t, transport)

	rec := do(t, handler, "/article")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got, want := rec.Body.String(), "No URL found"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if len(transport.requests) != 0 {
		t.Errorf("made %d outbound requests, want none", len(transport.requests))
	}
}

func TestArticleRejectsNonHTTPProtocols(t *testing.T) {
	transport := newStubTransport(t)
	handler := newTestHandler(t, transport)

	rec := do(t, handler, "/article?url="+url.QueryEscape("ftp://example.com/story"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got, want := rec.Body.String(), "Invalid URL protocol"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if len(transport.requests) != 0 {
		t.Errorf("made %d outbound requests, want none", len(transport.requests))
	}
}

func TestArticleRejectsMalformedURL(t *testing.T) {
	transport := newStubTransport(t)
	handler := newTestHandler(t, transport)

	rec := do(t, handler, "/article?url="+url.QueryEscape("http://"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got, want := rec.Body.String(), "Invalid URL"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if len(transport.requests) != 0 {
		t.Errorf("made %d outbound requests, want none", len(transport.requests))
	}
}

func TestArticleRedirectsUnconfiguredDomain(t *testing.T) {
	transport := newStubTransport(t)
	handler := newTestHandler(t, transport)
	source := "https://www.tmz.com/story"

	rec := do(t, handler, "/article?url="+url.QueryEscape(source))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != source {
		t.Errorf("Location = %q, want %q", got, source)
	}
	if len(transport.requests) != 0 {
		t.Errorf("made %d outbound requests, want none", len(transport.requests))
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestArticleReturns504WhenFetchTimesOut(t *testing.T) {
	transport := &stubTransport{handle: func(*http.Request) (*http.Response, error) {
		return nil, timeoutError{}
	}}
	handler := newTestHandler(t, transport)

	rec := do(t, handler, "/article?url="+url.QueryEscape("https://www.pcgamer.com/story"))

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
	if !strings.Contains(rec.Body.String(), "Request Timeout") {
		t.Error("error page is missing \"Request Timeout\"")
	}
}

// An origin error status redirects to the source instead of rendering the
// error page's body as if it were the article.
func TestArticleRedirectsWhenOriginErrors(t *testing.T) {
	body, err := fixtures.Read(fixtures.Articles["pcgamer.com"].File)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	transport := &stubTransport{handle: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    req,
		}, nil
	}}
	handler := newTestHandler(t, transport)
	source := "https://www.pcgamer.com/story"

	rec := do(t, handler, "/article?url="+url.QueryEscape(source))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	if got := rec.Header().Get("Location"); got != source {
		t.Errorf("Location = %q, want %q", got, source)
	}
}

func TestArticleRedirectsOnFetchError(t *testing.T) {
	transport := &stubTransport{handle: func(*http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	}}
	handler := newTestHandler(t, transport)
	source := "https://www.nbcnews.com/story"

	rec := do(t, handler, "/article?url="+url.QueryEscape(source))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != source {
		t.Errorf("Location = %q, want %q", got, source)
	}
}

func TestPanicBecomesInternalServerError(t *testing.T) {
	// A nil parser makes handleFeed panic, which is the cheapest way to
	// exercise the recovery middleware through the public API.
	handler := app.New(app.Deps{
		Feeds:   feeds.All,
		Configs: scrape.Configs,
		Scraper: scrape.New(&http.Client{Transport: newStubTransport(t)}),
	})

	rec := do(t, handler, "/feed")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got, want := rec.Body.String(), "Internal Server Error"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestHealth(t *testing.T) {
	handler := newTestHandler(t, newStubTransport(t))

	rec := do(t, handler, "/health")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), "ok"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestFavicon(t *testing.T) {
	handler := newTestHandler(t, newStubTransport(t))

	rec := do(t, handler, "/favicon.ico")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get("Content-Type"), "image/x-icon"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if got, want := rec.Body.String(), "icon"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}
