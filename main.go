package main

import (
	"context"
	_ "embed"
	"errors"
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

	handler := app.New(app.Deps{
		Feeds:        feeds.All,
		Configs:      scrape.Configs,
		Parser:       rss.New(),
		Scraper:      scrape.New(&http.Client{Timeout: articleTimeout}),
		Favicon:      favicon,
		FeedCache:    cache.New[*gofeed.Feed](app.FeedCacheTTL),
		ArticleCache: articleCache,
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
