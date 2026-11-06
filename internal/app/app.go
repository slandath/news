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
	"sync"
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

// FeedCacheMaxEntries bounds the feed cache the way ArticleCacheMaxEntries
// bounds the article cache. Feed keys are operator- or user-added feed URLs,
// an open set once the add form exists, so the cap keeps one-off URLs from
// growing memory without bound.
const FeedCacheMaxEntries = 100

// ArticleCacheMaxEntries bounds the article cache. Article keys are
// caller-controlled URLs with an open key space, unlike the closed list of
// feed URLs, so the cap keeps one-off URLs from growing memory without bound.
const ArticleCacheMaxEntries = 1000

// MaxUserFeeds bounds how many feeds POST /add accepts. Each accepted URL
// grows the in-memory list, the persisted file, and the home page, so the
// open form needs a ceiling. Built-in feeds are not counted.
const MaxUserFeeds = 100

// fetchCached returns the cached value for key with its remaining lifetime,
// or calls fetch and stores the result when the fetch reports it cacheable.
// A nil cache always fetches without storing. Errors come only from fetch;
// a miss is not an error.
func fetchCached[V any](c *cache.Cache[V], key string, ttl time.Duration, fetch func() (V, bool, error)) (V, time.Duration, error) {
	if c != nil {
		if value, remaining, ok := c.Get(key); ok {
			slog.Info("cache hit", "key", key)

			return value, remaining, nil
		}
	}

	slog.Debug("cache miss", "key", key)

	value, cacheable, err := fetch()
	if err != nil {
		var zero V

		return zero, ttl, err
	}

	if cacheable && c != nil {
		c.Set(key, value)
	}

	return value, ttl, nil
}

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
	Feeds         []feeds.Feed
	Configs       map[string]scrape.SiteConfig
	Parser        rss.Parser
	Scraper       *scrape.Scraper
	Favicon       []byte
	FeedCache     *cache.Cache[*gofeed.Feed]
	ArticleCache  *cache.Cache[Article]
	FeedStorePath string
	UserFeeds     []feeds.Feed
}

// App holds the injected dependencies for the handlers.
type App struct {
	mu           sync.RWMutex
	feeds        []feeds.Feed
	configs      map[string]scrape.SiteConfig
	parser       rss.Parser
	scraper      *scrape.Scraper
	favicon      []byte
	feedCache    *cache.Cache[*gofeed.Feed]
	articleCache *cache.Cache[Article]
	storePath    string
	userFeeds    []feeds.Feed
}

// New returns the application handler. UserFeeds are appended after Feeds
// with duplicates of an earlier entry skipped, so a stored feed that
// matches a built-in URL (or another stored feed) renders one card, and a
// dropped copy is not re-persisted on the next save.
func New(deps Deps) http.Handler {
	combined := make([]feeds.Feed, 0, len(deps.Feeds)+len(deps.UserFeeds))
	combined = append(combined, deps.Feeds...)

	users := make([]feeds.Feed, 0, len(deps.UserFeeds))

	for _, feed := range deps.UserFeeds {
		duplicate := false

		for _, kept := range combined {
			if sameFeedURL(feed.URL, kept.URL) {
				duplicate = true
				break
			}
		}

		if duplicate {
			continue
		}

		users = append(users, feed)
		combined = append(combined, feed)
	}

	app := &App{
		feeds:        combined,
		configs:      deps.Configs,
		parser:       deps.Parser,
		scraper:      deps.Scraper,
		favicon:      deps.Favicon,
		feedCache:    deps.FeedCache,
		articleCache: deps.ArticleCache,
		storePath:    deps.FeedStorePath,
		userFeeds:    users,
	}

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", app.wrap(app.handleHome))
	mux.Handle("GET /feed", app.wrap(app.handleFeed))
	mux.Handle("GET /article", app.wrap(app.handleArticle))
	mux.Handle("GET /add", app.wrap(app.handleAddForm))
	mux.Handle("POST /add", app.wrap(app.handleAddFeed))
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

		// Handlers log their own failures with context; the per-request
		// info line above already records the outcome by status.
		err := next(recorder, r)
		if err == nil {
			return
		}

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
// sameFeedURL reports whether two feed URLs name the same feed. Scheme and
// host compare case-insensitively; userinfo and query compare exactly, so
// differently credentialed URLs stay distinct. A default port (:443 on
// https, :80 on http) matches a bare host, and an empty path matches "/",
// so resubmits under either spelling are still duplicates. Unparsable URLs
// fall back to exact string matching.
func sameFeedURL(a, b string) bool {
	first, errA := url.Parse(a)
	second, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return a == b
	}

	firstUser, secondUser := "", ""
	if first.User != nil {
		firstUser = first.User.String()
	}

	if second.User != nil {
		secondUser = second.User.String()
	}

	return strings.EqualFold(first.Scheme, second.Scheme) &&
		strings.EqualFold(first.Hostname(), second.Hostname()) &&
		normalizeFeedPort(first) == normalizeFeedPort(second) &&
		firstUser == secondUser &&
		normalizeFeedPath(first) == normalizeFeedPath(second) &&
		first.RawQuery == second.RawQuery
}

// normalizeFeedPort returns "" for the scheme's default port, so an explicit
// :443 (or :80) still matches a bare host. The port compares separately
// from the hostname: an IPv6 address ending in ":8443" must stay distinct
// from a ":8443" port on the same address.
func normalizeFeedPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		if (strings.EqualFold(u.Scheme, "https") && port == "443") ||
			(strings.EqualFold(u.Scheme, "http") && port == "80") {
			return ""
		}

		return port
	}

	return ""
}

// normalizeFeedPath treats an empty path as "/", which is what the origin
// server sees for both spellings.
func normalizeFeedPath(u *url.URL) string {
	if u.EscapedPath() == "" {
		return "/"
	}

	return u.EscapedPath()
}

func (a *App) configuredFeed(feedURL string) (feeds.Feed, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, feed := range a.feeds {
		if sameFeedURL(feed.URL, feedURL) {
			return feed, true
		}
	}

	return feeds.Feed{}, false
}

// homeSections copies the feed list into sections under the read lock. The
// response is written after the lock is released, so a slow client cannot
// hold the lock while the body streams.
func (a *App) homeSections() []templates.FeedSection {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Sections merge by title in first-seen order, so News renders before
	// Sports while each group keeps display order — even when user-added
	// feeds interleave with the hardcoded list.
	var sections []templates.FeedSection
	index := make(map[string]int)

	for _, feed := range a.feeds {
		i, ok := index[feed.Section]
		if !ok {
			i = len(sections)
			index[feed.Section] = i
			sections = append(sections, templates.FeedSection{Title: feed.Section})
		}

		sections[i].Feeds = append(sections[i].Feeds, templates.FeedLink{
			Title: feed.Title,
			Href:  feedHref(feed.URL),
		})
	}

	return sections
}

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) error {
	sections := a.homeSections()

	return writeHTML(r.Context(), w, http.StatusOK, templates.Home(templates.HomeProps{Sections: sections}))
}

// addFormError re-renders the add-feed form with status 400 and the message
// inline. It is the one place a 400 carries HTML instead of plain text: a
// form has to show the message where the user can act on it.
func addFormError(w http.ResponseWriter, r *http.Request, url, section, problem string) error {
	// Every validation rejection logs here, so no branch needs its own
	// slog.Error call to satisfy the error-surface rule.
	slog.Error("add-feed rejected", "url", url, "section", section, "reason", problem)

	return writeHTML(r.Context(), w, http.StatusBadRequest, templates.Add(templates.AddForm{
		Error:   problem,
		URL:     url,
		Section: section,
	}))
}

func (a *App) handleAddForm(w http.ResponseWriter, r *http.Request) error {
	return writeHTML(r.Context(), w, http.StatusOK, templates.Add(templates.AddForm{}))
}

// tryAddFeed appends the validated feed under the write lock. It reports
// whether the URL was already listed, whether the user-feed list is full,
// or a persistence error. Callers render the response after the lock is
// released, so a slow client cannot hold it.
func (a *App) tryAddFeed(added feeds.Feed) (duplicate, full bool, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Re-check under the write lock: two concurrent adds of the same URL
	// must not both land.
	for _, feed := range a.feeds {
		if sameFeedURL(feed.URL, added.URL) {
			return true, false, nil
		}
	}

	if len(a.userFeeds) >= MaxUserFeeds {
		return false, true, nil
	}

	if a.storePath != "" {
		next := make([]feeds.Feed, 0, len(a.userFeeds)+1)
		next = append(next, a.userFeeds...)
		next = append(next, added)

		if err := feeds.SaveUserFeeds(a.storePath, next); err != nil {
			return false, false, err
		}

		a.userFeeds = next
	}

	a.feeds = append(a.feeds, added)

	return false, false, nil
}

func (a *App) handleAddFeed(w http.ResponseWriter, r *http.Request) error {
	// A cross-site page can auto-submit a form to a local or LAN instance.
	// Browsers label such requests; direct API use sends no header and
	// stays allowed.
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return addFormError(w, r, "", feeds.SectionNews, "Requests must come from this site.")
	}

	if err := r.ParseForm(); err != nil {
		return addFormError(w, r, "", feeds.SectionNews, "Could not read the form. Please try again.")
	}

	rawURL := strings.TrimSpace(r.PostFormValue("url"))
	section := r.PostFormValue("section")
	if section == "" {
		section = feeds.SectionNews
	}

	if rawURL == "" {
		return addFormError(w, r, rawURL, section, "Enter a feed URL.")
	}

	source, err := url.Parse(rawURL)
	if err != nil || source.Scheme != "https" || source.Host == "" {
		return addFormError(w, r, rawURL, section, "Feed URL must be an https:// address.")
	}

	if section != feeds.SectionNews && section != feeds.SectionSports {
		return addFormError(w, r, rawURL, feeds.SectionNews, "Section must be News or Sports.")
	}

	if _, ok := a.configuredFeed(rawURL); ok {
		return addFormError(w, r, rawURL, section, "That feed is already listed.")
	}

	parsed, err := a.parser.ParseURLWithContext(rawURL, r.Context())
	if err != nil {
		slog.Error("fetch new feed", "url", rawURL, "err", err)

		return addFormError(w, r, rawURL, section, "Could not fetch that feed. Check the URL and try again.")
	}

	title := strings.TrimSpace(parsed.Title)
	if title == "" {
		return addFormError(w, r, rawURL, section, "That feed has no title and cannot be listed.")
	}

	added := feeds.Feed{Title: title, URL: rawURL, Section: section}

	// Decide and store under the lock. Render any response after unlocking,
	// so a slow client cannot hold the lock.
	duplicate, full, err := a.tryAddFeed(added)
	if err != nil {
		slog.Error("save user feeds", "err", err)

		return &httpError{Status: http.StatusInternalServerError, Msg: "Internal Server Error"}
	}

	if full {
		return addFormError(w, r, rawURL, section, "This server already lists the maximum number of feeds.")
	}

	if duplicate {
		return addFormError(w, r, rawURL, section, "That feed is already listed.")
	}

	// Seed the cache with the just-fetched feed so the redirect target
	// renders without a second origin fetch.
	if a.feedCache != nil {
		a.feedCache.Set(rawURL, parsed)
	}

	slog.Info("feed added", "url", rawURL, "section", section)

	w.Header().Set("Location", feedHref(rawURL))
	w.WriteHeader(http.StatusSeeOther)

	return nil
}

func (a *App) handleFeed(w http.ResponseWriter, r *http.Request) error {
	feedURL := r.URL.Query().Get("url")
	if feedURL == "" {
		a.mu.RLock()
		if len(a.feeds) > 0 {
			feedURL = a.feeds[0].URL
		}
		a.mu.RUnlock()
	}
	if feedURL == "" {
		// Unreachable while the feed list is non-empty; the empty-list
		// guard stays so a misconfiguration fails closed.
		slog.Error("feed request with no feeds configured")
		writeText(w, http.StatusBadRequest, "URL undefined")
		return nil
	}

	// Only configured feeds are fetchable. The URL param is client-controlled
	// and the parser fetches whatever it is handed, so an unrestricted value
	// turns the server into a fetch proxy for internal services. The article
	// path already restricts hosts to scrape.Configs the same way.
	configured, ok := a.configuredFeed(feedURL)
	if !ok {
		slog.Error("unconfigured feed url", "url", feedURL)
		writeText(w, http.StatusBadRequest, "Invalid feed URL")
		return nil
	}

	// Use the configured spelling from here on: case or fragment variants
	// of the same feed must share one cache entry and one fetch target.
	feedURL = configured.URL

	feed, lifetime, err := fetchCached(a.feedCache, feedURL, FeedCacheTTL, func() (*gofeed.Feed, bool, error) {
		parsed, err := a.parser.ParseURLWithContext(feedURL, r.Context())
		if err != nil {
			return nil, false, err
		}

		return parsed, true, nil
	})
	if err != nil {
		slog.Error("fetch feed", "url", feedURL, "err", err)
		return &httpError{Status: http.StatusBadGateway, Msg: "Bad Feed"}
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
		slog.Error("article request without url")
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
		slog.Error("article request with invalid protocol", "url", urlParam)
		writeText(w, http.StatusBadRequest, "Invalid URL protocol")
		return nil
	}
	// url.Parse accepts "http://" with an empty host; the empty-host check
	// below rejects it as "Invalid URL".
	if source.Host == "" {
		slog.Error("article request with invalid url", "url", urlParam)
		writeText(w, http.StatusBadRequest, "Invalid URL")
		return nil
	}

	sourceURL := source.String()

	// Hostnames are lowercased before the scrape.Configs lookup.
	domain := strings.TrimPrefix(strings.ToLower(source.Hostname()), "www.")
	config, ok := a.configs[domain]
	if !ok {
		redirect(w, sourceURL)
		return nil
	}

	// fresh reports the origin status of an uncached fetch; a hit counts as
	// 2xx since only 2xx bodies are ever stored. Only a 2xx body is cached
	// or advertised as cacheable: an error page must not linger for an hour
	// in the server cache or in a browser.
	fresh := http.StatusOK

	entry, lifetime, err := fetchCached(a.articleCache, sourceURL, ArticleCacheTTL, func() (Article, bool, error) {
		title, paragraphs, status, err := a.scraper.Extract(r.Context(), sourceURL, config)
		if err != nil {
			return Article{}, false, err
		}

		fresh = status

		return Article{Title: title, Paragraphs: paragraphs}, status >= 200 && status < 300, nil
	})
	if err != nil {
		if scrape.IsTimeout(err) {
			slog.Error("article fetch timeout", "url", sourceURL, "err", err)
			return &httpError{Status: http.StatusGatewayTimeout, Msg: "Request Timeout"}
		}

		slog.Error("scrape article", "url", sourceURL, "err", err)
		redirect(w, sourceURL)
		return nil
	}

	title, paragraphs := entry.Title, entry.Paragraphs

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
