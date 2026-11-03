// Package app wires the HTTP routes to the feed parser, the article scraper,
// and the page templates.
package app

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	"github.com/slandath/news/internal/feeds"
	"github.com/slandath/news/internal/rss"
	"github.com/slandath/news/internal/scrape"
	"github.com/slandath/news/internal/templates"
)

// Deps is everything the handlers need. Nothing is read from package-level
// state, so tests can build an app with stubs for any subset.
type Deps struct {
	Feeds   []feeds.Feed
	Configs map[string]scrape.SiteConfig
	Parser  rss.Parser
	Scraper *scrape.Scraper
	Favicon []byte
}

// App holds the injected dependencies for the handlers.
type App struct {
	feeds   []feeds.Feed
	configs map[string]scrape.SiteConfig
	parser  rss.Parser
	scraper *scrape.Scraper
	favicon []byte
}

// New returns the application handler.
func New(deps Deps) http.Handler {
	app := &App{
		feeds:   deps.Feeds,
		configs: deps.Configs,
		parser:  deps.Parser,
		scraper: deps.Scraper,
		favicon: deps.Favicon,
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

	feed, err := a.parser.ParseURLWithContext(feedURL, r.Context())
	if err != nil {
		return &httpError{Status: http.StatusBadGateway, Msg: "Bad Feed"}
	}

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

	title, paragraphs, err := a.scraper.Extract(r.Context(), sourceURL, config)
	if err != nil {
		if scrape.IsTimeout(err) {
			return &httpError{Status: http.StatusGatewayTimeout, Msg: "Request Timeout"}
		}

		slog.Error("scrape article", "url", sourceURL, "err", err)
		redirect(w, sourceURL)
		return nil
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
