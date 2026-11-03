// Package app wires the HTTP routes to the feed parser, the article scraper,
// and the page templates.
package app

import (
	"bytes"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/slandath/news/internal/cache"
	"github.com/slandath/news/internal/feeds"
	"github.com/slandath/news/internal/rss"
	"github.com/slandath/news/internal/scrape"
	"github.com/slandath/news/internal/templates"
)

// FeedCacheTTL bounds how long a parsed feed is reused: most feeds update
// every 15-60 minutes, so 5 minutes keeps pages fresh without refetching per
// visitor. ArticleCacheTTL is longer because published articles rarely change.
const (
	FeedCacheTTL    = 5 * time.Minute
	ArticleCacheTTL = time.Hour
)

// ArticleCacheMaxEntries bounds the article cache. Article keys are
// caller-controlled URLs with an open key space, unlike the closed list of
// feed URLs, so the cap keeps one-off URLs from growing memory without bound.
const ArticleCacheMaxEntries = 1000

// cacheControl builds a Cache-Control header from a remaining lifetime.
// Ceiling keeps a just-stored entry advertising exactly its TTL instead of
// one second less, and a hit near expiry advertising only what is left, so a
// browser or CDN never holds the response longer than the server would.
func cacheControl(lifetime time.Duration) string {
	secs := int(math.Ceil(lifetime.Seconds()))
	if secs < 0 {
		secs = 0
	}

	return "public, max-age=" + strconv.Itoa(secs)
}

// Article is one scraped article: plain title and body paragraphs, never
// markup. It is the value type of the article cache.
type Article struct {
	Title      string
	Paragraphs []string
}

// Deps is everything the handlers need. Nothing is read from package-level
// state, so tests can build an app with stubs for any subset. A nil
// FeedCache or ArticleCache disables that cache.
type Deps struct {
	Feeds        []feeds.Feed
	Configs      map[string]scrape.SiteConfig
	Parser       rss.Parser
	Scraper      *scrape.Scraper
	Favicon      []byte
	FeedCache    *cache.Cache[*gofeed.Feed]
	ArticleCache *cache.Cache[Article]
}

// App holds the injected dependencies for the handlers.
type App struct {
	feeds        []feeds.Feed
	configs      map[string]scrape.SiteConfig
	parser       rss.Parser
	scraper      *scrape.Scraper
	favicon      []byte
	feedCache    *cache.Cache[*gofeed.Feed]
	articleCache *cache.Cache[Article]
}

// New returns the application handler.
func New(deps Deps) http.Handler {
	app := &App{
		feeds:        deps.Feeds,
		configs:      deps.Configs,
		parser:       deps.Parser,
		scraper:      deps.Scraper,
		favicon:      deps.Favicon,
		feedCache:    deps.FeedCache,
		articleCache: deps.ArticleCache,
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", app.wrap(app.handleHome))
	mux.Handle("GET /feed", app.wrap(app.handleFeed))
	mux.Handle("GET /article", app.wrap(app.handleArticle))
	mux.Handle("GET /favicon.ico", app.wrap(app.handleFavicon))
	mux.Handle("GET /health", app.wrap(app.handleHealth))
	// Unqualified "/" is the least specific pattern, so it only sees what
	// nothing above matched: every method on every unknown path.
	mux.Handle("/", app.wrap(app.handleNotFound))

	return mux
}

// handler is a request handler that reports an error instead of writing one.
// A non-nil error must be returned before anything has been written.
type handler func(http.ResponseWriter, *http.Request) error

// wrap adapts a handler to http.Handler: it renders returned errors, turns
// panics into a 500, and logs one line per request.
func (a *App) wrap(next handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic serving request",
					"method", r.Method, "path", r.URL.Path,
					"panic", recovered, "stack", string(debug.Stack()))
				// If the response was already started this write is dropped by
				// net/http, which is the best that can be done at this point.
				writeText(recorder, http.StatusInternalServerError, "Internal Server Error")
			}

			slog.Info("request",
				"method", r.Method, "path", r.URL.Path,
				"status", recorder.status, "duration", time.Since(start).String())
		}()

		err := next(recorder, r)
		if err == nil {
			return
		}

		slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)

		var httpErr *httpError
		if errors.As(err, &httpErr) {
			page := templates.Error(templates.ErrorProps{Title: "Error", Message: httpErr.Msg})
			if renderErr := writeHTML(r.Context(), recorder, httpErr.Status, page); renderErr != nil {
				slog.Error("render error page", "err", renderErr)
			}
			return
		}

		writeText(recorder, http.StatusInternalServerError, "Internal Server Error")
	})
}

// statusRecorder captures the status code for the request log.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}

	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	return r.ResponseWriter.Write(body)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func feedHref(feedURL string) string {
	return "/feed?" + url.Values{"url": {feedURL}}.Encode()
}

func articleHref(articleURL, feedURL string) string {
	return "/article?" + url.Values{"url": {articleURL}, "feed": {feedURL}}.Encode()
}

// configuredFeed returns the entry whose URL matches, if any.
func (a *App) configuredFeed(feedURL string) (feeds.Feed, bool) {
	for _, feed := range a.feeds {
		if feed.URL == feedURL {
			return feed, true
		}
	}

	return feeds.Feed{}, false
}

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) error {
	links := make([]templates.FeedLink, 0, len(a.feeds))
	for _, feed := range a.feeds {
		links = append(links, templates.FeedLink{
			Title: feed.Title,
			Href:  feedHref(feed.URL),
		})
	}

	return writeHTML(r.Context(), w, http.StatusOK, templates.Home(templates.HomeProps{Feeds: links}))
}

func (a *App) handleFeed(w http.ResponseWriter, r *http.Request) error {
	feedURL := r.URL.Query().Get("url")
	if feedURL == "" && len(a.feeds) > 0 {
		feedURL = a.feeds[0].URL
	}
	if feedURL == "" {
		// Unreachable while the feed list is non-empty; kept for parity with
		// the TypeScript app's defensive branch.
		writeText(w, http.StatusBadRequest, "URL undefined")
		return nil
	}

	// Only configured feeds are fetchable. The URL param is client-controlled
	// and the parser fetches whatever it is handed, so an unrestricted value
	// turns the server into a fetch proxy for internal services. The article
	// path already restricts hosts to scrape.Configs the same way.
	configured, ok := a.configuredFeed(feedURL)
	if !ok {
		writeText(w, http.StatusBadRequest, "Invalid feed URL")
		return nil
	}

	var feed *gofeed.Feed

	lifetime := FeedCacheTTL

	if a.feedCache != nil {
		var (
			remaining time.Duration
			ok        bool
		)

		if feed, remaining, ok = a.feedCache.Get(feedURL); ok {
			slog.Info("feed cache hit", "url", feedURL)
			lifetime = remaining
		} else {
			slog.Debug("feed cache miss", "url", feedURL)
		}
	}

	if feed == nil {
		parsed, err := a.parser.ParseURLWithContext(feedURL, r.Context())
		if err != nil {
			return &httpError{Status: http.StatusBadGateway, Msg: "Bad Feed"}
		}

		feed = parsed

		if a.feedCache != nil {
			a.feedCache.Set(feedURL, feed)
		}
	}

	w.Header().Set("Cache-Control", cacheControl(lifetime))

	// A configured feed's own title wins over whatever the feed calls itself,
	// so the page and the navigation agree.
	title := configured.Title

	items := make([]templates.FeedItem, 0, len(feed.Items))
	for _, item := range feed.Items {
		rendered := templates.FeedItem{
			Title:     item.Title,
			Published: item.Published,
		}
		if item.Link != "" {
			rendered.Href = articleHref(item.Link, feedURL)
		}
		items = append(items, rendered)
	}

	return writeHTML(r.Context(), w, http.StatusOK, templates.Feed(templates.FeedProps{
		Title: title,
		Items: items,
	}))
}

func (a *App) handleArticle(w http.ResponseWriter, r *http.Request) error {
	urlParam := r.URL.Query().Get("url")
	feedURL := r.URL.Query().Get("feed")

	if urlParam == "" {
		writeText(w, http.StatusBadRequest, "No URL found")
		return nil
	}

	source, err := url.Parse(urlParam)
	if err != nil {
		slog.Error("parse article url", "url", urlParam, "err", err)
		writeText(w, http.StatusBadRequest, "Invalid URL")
		return nil
	}
	if source.Scheme != "http" && source.Scheme != "https" {
		writeText(w, http.StatusBadRequest, "Invalid URL protocol")
		return nil
	}
	// url.Parse is lenient where the WHATWG parser is not: "http://" parses
	// fine and yields an empty host, while the TypeScript app rejected it as
	// "Invalid URL".
	if source.Host == "" {
		writeText(w, http.StatusBadRequest, "Invalid URL")
		return nil
	}

	sourceURL := source.String()

	// Hostnames are lowercased to match the WHATWG URL parser, which the
	// TypeScript app relied on.
	domain := strings.TrimPrefix(strings.ToLower(source.Hostname()), "www.")
	config, ok := a.configs[domain]
	if !ok {
		redirect(w, sourceURL)
		return nil
	}

	var (
		title      string
		paragraphs []string
		cached     bool
	)

	lifetime := ArticleCacheTTL

	if a.articleCache != nil {
		var (
			entry     Article
			remaining time.Duration
			ok        bool
		)

		if entry, remaining, ok = a.articleCache.Get(sourceURL); ok {
			slog.Info("article cache hit", "host", source.Host, "path", source.Path)

			title, paragraphs, cached = entry.Title, entry.Paragraphs, true
			lifetime = remaining
		} else {
			slog.Debug("article cache miss", "host", source.Host, "path", source.Path)
		}
	}

	// fresh reports the origin status of an uncached fetch. Only a 2xx body
	// is cached or advertised as cacheable: an error page must not linger
	// for an hour in the server cache or in a browser.
	fresh := http.StatusOK

	if !cached {
		var (
			err    error
			status int
		)

		title, paragraphs, status, err = a.scraper.Extract(r.Context(), sourceURL, config)
		if err != nil {
			if scrape.IsTimeout(err) {
				return &httpError{Status: http.StatusGatewayTimeout, Msg: "Request Timeout"}
			}

			slog.Error("scrape article", "url", sourceURL, "err", err)
			redirect(w, sourceURL)
			return nil
		}

		fresh = status

		if fresh >= 200 && fresh < 300 && a.articleCache != nil {
			a.articleCache.Set(sourceURL, Article{Title: title, Paragraphs: paragraphs})
		}
	}

	if fresh >= 200 && fresh < 300 {
		w.Header().Set("Cache-Control", cacheControl(lifetime))
	}

	props := templates.ArticleProps{
		Title:      title,
		Paragraphs: paragraphs,
		SourceURL:  sourceURL,
	}
	if feed, found := a.configuredFeed(feedURL); found {
		props.FeedTitle = feed.Title
		props.FeedHref = feedHref(feed.URL)
	}

	return writeHTML(r.Context(), w, http.StatusOK, templates.Article(props))
}

func (a *App) handleFavicon(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "favicon.ico", time.Time{}, bytes.NewReader(a.favicon))

	return nil
}

func (a *App) handleNotFound(w http.ResponseWriter, r *http.Request) error {
	return writeHTML(r.Context(), w, http.StatusNotFound, templates.Error(templates.ErrorProps{
		Title:   "Not Found",
		Message: "Page not found.",
	}))
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) error {
	writeText(w, http.StatusOK, "ok")

	return nil
}
