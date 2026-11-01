// Package rss parses RSS, Atom, and JSON feeds.
package rss

import (
	"context"

	"github.com/mmcdole/gofeed"
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

// New returns the production parser.
func New() *gofeed.Parser {
	parser := gofeed.NewParser()
	parser.MaxByteSize = maxFeedBytes
	return parser
}
