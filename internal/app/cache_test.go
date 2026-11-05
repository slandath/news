package app_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/slandath/news/internal/app"
	"github.com/slandath/news/internal/cache"
	"github.com/slandath/news/internal/feeds"
	"github.com/slandath/news/internal/fixtures"
	"github.com/slandath/news/internal/rss"
	"github.com/slandath/news/internal/scrape"
)

// newCachedTestHandler mirrors newTestHandler but with the feed and article
// caches enabled, so tests can prove a second identical request is served
// without another outbound fetch.
func newCachedTestHandler(t *testing.T, transport http.RoundTripper) http.Handler {
	t.Helper()

	parser := rss.New()
	parser.Client = &http.Client{Transport: transport}

	return app.New(app.Deps{
		Feeds:        feeds.All,
		Configs:      scrape.Configs,
		Parser:       parser,
		Scraper:      scrape.New(&http.Client{Transport: transport}),
		Favicon:      []byte("icon"),
		FeedCache:    cache.New[*gofeed.Feed](app.FeedCacheTTL),
		ArticleCache: cache.New[app.Article](app.ArticleCacheTTL),
	})
}

func TestFeedSecondRequestHitsCache(t *testing.T) {
	transport := newStubTransport(t)
	handler := newCachedTestHandler(t, transport)
	target := "/feed?url=" + url.QueryEscape(feeds.All[0].URL)

	first := do(t, handler, target)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	second := do(t, handler, target)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusOK)
	}

	if len(transport.requests) != 1 {
		t.Errorf("made %d outbound requests, want 1 (second should hit the cache)", len(transport.requests))
	}

	if got, want := second.Header().Get("Cache-Control"), "public, max-age=300"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

// A case-variant spelling of a configured feed shares its cache entry: the
// handler canonicalizes to the configured URL before lookup and fetch, so
// the variant renders with no second outbound request.
func TestFeedVariantSpellingHitsCache(t *testing.T) {
	transport := newStubTransport(t)
	handler := newCachedTestHandler(t, transport)
	canonical := feeds.All[0].URL

	first := do(t, handler, "/feed?url="+url.QueryEscape(canonical))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	variant := do(t, handler, "/feed?url="+url.QueryEscape("https://WWW.SLASHFILM.COM/feed/"))
	if variant.Code != http.StatusOK {
		t.Fatalf("variant status = %d, want %d", variant.Code, http.StatusOK)
	}

	if len(transport.requests) != 1 {
		t.Errorf("made %d outbound requests, want 1 (variant should hit the canonical entry)", len(transport.requests))
	}

	if first.Body.String() != variant.Body.String() {
		t.Error("variant body differs from the canonical body")
	}
}

func TestArticleSecondRequestHitsCache(t *testing.T) {
	transport := newStubTransport(t)
	handler := newCachedTestHandler(t, transport)
	target := "/article?url=" + url.QueryEscape("https://www.pcgamer.com/story")

	first := do(t, handler, target)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	second := do(t, handler, target)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusOK)
	}

	if len(transport.requests) != 1 {
		t.Errorf("made %d outbound requests, want 1 (second should hit the cache)", len(transport.requests))
	}

	if got, want := second.Header().Get("Cache-Control"), "public, max-age=3600"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}

	if first.Body.String() != second.Body.String() {
		t.Error("cached article body differs from the original")
	}
}

// An origin error redirects instead of rendering, and the failure is never
// cached: once the origin recovers, the article renders instead of serving
// a stale error.
func TestArticleNon2xxBodyIsNotCached(t *testing.T) {
	body, err := fixtures.Read(fixtures.Articles["pcgamer.com"].File)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	status := http.StatusServiceUnavailable
	transport := &stubTransport{handle: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    req,
		}, nil
	}}
	handler := newCachedTestHandler(t, transport)
	target := "/article?url=" + url.QueryEscape("https://www.pcgamer.com/story")

	first := do(t, handler, target)
	if first.Code != http.StatusFound {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusFound)
	}

	if got := first.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q, want empty for a non-2xx origin", got)
	}

	status = http.StatusOK

	second := do(t, handler, target)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusOK)
	}

	if len(transport.requests) != 2 {
		t.Errorf("made %d outbound requests, want 2 (failures must not cache)", len(transport.requests))
	}

	if !bytes.Contains(second.Body.Bytes(), []byte(fixtures.Articles["pcgamer.com"].Title)) {
		t.Error("recovered article body is missing, want the fresh fetch rendered")
	}
}
