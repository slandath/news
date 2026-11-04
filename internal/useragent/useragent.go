// Package useragent identifies the app's outbound requests. Outlets see
// this string instead of Go's default User-Agent, so operators can tell
// the traffic is a feed reader rather than an anonymous bot.
package useragent

import "net/http"

// UserAgent is sent on every feed fetch and article scrape.
const UserAgent = "news-reader (+https://news.tomslanda.com)"

// userAgentTransport stamps the app's User-Agent on every request it carries.
type userAgentTransport struct {
	next http.RoundTripper
}

// Transport returns a RoundTripper that sets UserAgent on every request and
// carries it with next. A nil next means http.DefaultTransport.
func Transport(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}

	return &userAgentTransport{next: next}
}

// RoundTrip clones the request so the caller's headers are untouched, sets
// the User-Agent, and carries the request with the wrapped transport.
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("User-Agent", UserAgent)

	return t.next.RoundTrip(clone)
}
