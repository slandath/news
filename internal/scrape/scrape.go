package scrape

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// maxArticleBytes caps how much of an article page is read. Beyond this the
// page is truncated and whatever parsed is used: the cap exists so a hostile
// or broken page cannot make one request allocate without bound.
const maxArticleBytes = 10 << 20 // 10 MiB

// Scraper fetches article pages and extracts their text.
type Scraper struct {
	client *http.Client
}

// New returns a Scraper that uses client for every fetch. The client must not
// be nil, and should carry a Timeout: the article handler relies on it to turn
// a page that never arrives into a 504.
func New(client *http.Client) *Scraper {
	return &Scraper{client: client}
}

// Extract fetches rawURL and returns the page title and body paragraphs
// selected by cfg, along with the origin's HTTP status. Both are plain text,
// never markup. A zero status means no response was received.
//
// The status is reported but deliberately not acted on here. The TypeScript
// version parsed whatever body came back, error pages included, and changing
// that would turn some currently-rendered pages into redirects. Callers
// decide what the status means for them, for example whether a result is
// worth caching.
func (s *Scraper) Extract(ctx context.Context, rawURL string, cfg SiteConfig) (title string, paragraphs []string, statusCode int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", nil, 0, fmt.Errorf("fetch article: %w", err)
	}
	// The body is read to EOF below, so a close error cannot affect the result.
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArticleBytes))
	if err != nil {
		return "", nil, resp.StatusCode, fmt.Errorf("read article: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", nil, resp.StatusCode, fmt.Errorf("parse article: %w", err)
	}

	return doc.Find(cfg.Title).Text(), cfg.ArticleWrapper(doc), resp.StatusCode, nil
}

// IsTimeout reports whether err means the page did not arrive in time, as
// opposed to being unreachable. The handler renders 504 for the former and
// redirects to the source for the latter.
//
// Both mechanisms are covered because they produce different error types: an
// http.Client.Timeout surfaces as a *url.Error implementing net.Error, while a
// cancelled request context surfaces as context.DeadlineExceeded.
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
