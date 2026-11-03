package app_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/slandath/news/internal/app"
	"github.com/slandath/news/internal/cache"
	"github.com/slandath/news/internal/feeds"
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
