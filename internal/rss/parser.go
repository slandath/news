// Package rss parses RSS, Atom, and JSON feeds.
package rss

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/slandath/news/internal/useragent"
)

// Parser is the narrow surface the feed handler needs. *gofeed.Parser
// satisfies it as-is, so tests can substitute a stub without an adapter.
//
// The context argument is what keeps a slow feed server from pinning a
// request forever: cancelling it aborts the fetch.
type Parser interface {
	ParseURLWithContext(feedURL string, ctx context.Context) (*gofeed.Feed, error)
}

var _ Parser = (*gofeed.Parser)(nil)

// maxFeedBytes caps how much of a feed response is read. Feeds are a few
// hundred KB at most; the cap keeps a hostile or broken endpoint from making
// one request allocate without bound.
const maxFeedBytes = 10 << 20 // 10 MiB

// feedTimeout covers connect, headers, and body for one feed fetch. A feed
// stalled by a slow or broken endpoint would otherwise hold a request
// goroutine for as long as the remote keeps trickling bytes.
const feedTimeout = 10 * time.Second

// New returns the production parser.
func New() *gofeed.Parser {
	parser := gofeed.NewParser()
	parser.MaxByteSize = maxFeedBytes

	// gofeed's ParseURLWithContext uses http.DefaultClient when Client is
	// nil. Hand it an explicit client so the timeout applies and outlets
	// see the app's User-Agent; main installs DialPublicIP on
	// http.DefaultTransport separately so only public IPs are dialed.
	// An https feed must not downgrade to http mid-redirect: the
	// cleartext hop is fetched (and its URL persisted) without the
	// scheme the operator approved. Tests replace this field with a stub
	// transport.
	parser.Client = &http.Client{
		Timeout:   feedTimeout,
		Transport: useragent.Transport(nil),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}

			// Compare against the immediately previous request, not the
			// first: an http → https → http chain ends cleartext and must
			// still be refused.
			if previous := via[len(via)-1]; previous.URL.Scheme == "https" && req.URL.Scheme != "https" {
				return errors.New("refusing redirect from https to non-https")
			}

			return nil
		},
	}

	return parser
}
