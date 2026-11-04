package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/slandath/news/internal/app"
	"github.com/slandath/news/internal/cache"
	"github.com/slandath/news/internal/feeds"
	"github.com/slandath/news/internal/rss"
	"github.com/slandath/news/internal/scrape"
	"github.com/slandath/news/internal/ssrf"
)

//go:embed favicon.ico
var favicon []byte

const (
	defaultPort     = "3000"
	shutdownTimeout = 10 * time.Second

	// articleTimeout covers connect, headers, and body for one article fetch.
	articleTimeout = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Railway injects PORT. Binding to ":"+port listens on all interfaces,
	// which the platform healthcheck and proxy require.
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	articleCache := cache.New[app.Article](app.ArticleCacheTTL)
	articleCache.SetMaxEntries(app.ArticleCacheMaxEntries)

	// Every outbound fetch goes through http.DefaultTransport: feed
	// parsing, article scraping, and the add-feed validation fetch. URLs
	// ultimately come from users, so install the SSRF guard process-wide:
	// with the stock transport a submitted URL could reach loopback, LAN,
	// or link-local addresses, and with a proxy set the guard would vet
	// the proxy instead of the real destination. There is deliberately no
	// proxy support for server-side fetches; a deployment that needs one
	// is an explicit future change, not silent breakage.
	guarded := http.DefaultTransport.(*http.Transport).Clone()
	guarded.Proxy = nil
	guarded.DialContext = ssrf.DialPublicIP
	http.DefaultTransport = guarded

	// User-added feeds persist in a JSON file so they survive restarts. A
	// corrupt file fails startup loudly rather than silently dropping feeds.
	storePath := os.Getenv("FEEDS_FILE")
	if storePath == "" {
		storePath = "user-feeds.json"
	}

	userFeeds, err := feeds.LoadUserFeeds(storePath)
	if err != nil {
		return fmt.Errorf("load user feeds: %w", err)
	}

	allFeeds := make([]feeds.Feed, 0, len(feeds.All)+len(userFeeds))
	allFeeds = append(allFeeds, feeds.All...)
	allFeeds = append(allFeeds, userFeeds...)

	feedCache := cache.New[*gofeed.Feed](app.FeedCacheTTL)
	feedCache.SetMaxEntries(app.FeedCacheMaxEntries)

	handler := app.New(app.Deps{
		Feeds:         allFeeds,
		UserFeeds:     userFeeds,
		Configs:       scrape.Configs,
		Parser:        rss.New(),
		Scraper:       scrape.New(&http.Client{Timeout: articleTimeout}),
		Favicon:       favicon,
		FeedCache:     feedCache,
		ArticleCache:  articleCache,
		FeedStorePath: storePath,
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
