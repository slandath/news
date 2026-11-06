package rss

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/slandath/news/internal/useragent"
)

// Parses the hostile-payload fixture with the production parser, offline.
// This pins the field mapping the templates depend on — in particular that
// Published keeps the raw date string, which is what rss-parser exposed as
// item.pubDate — and proves the fixture itself is valid input.
func TestParseFixtureFeed(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "testdata", "fixture.rss"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close fixture: %v", err)
		}
	})

	feed, err := New().Parse(file)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	if diff := cmp.Diff("Fixture Feed & Wire", feed.Title); diff != "" {
		t.Errorf("feed title mismatch (-want +got):\n%s", diff)
	}
	if got, want := len(feed.Items), 2; got != want {
		t.Fatalf("len(items) = %d, want %d", got, want)
	}

	first := feed.Items[0]
	if diff := cmp.Diff("Item <script>one</script>", first.Title); diff != "" {
		t.Errorf("item title mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff("https://example.com/story-1", first.Link); diff != "" {
		t.Errorf("item link mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff("Wed, 21 Oct 2026 10:00:00 GMT", first.Published); diff != "" {
		t.Errorf("item published mismatch (-want +got):\n%s", diff)
	}
	if first.PublishedParsed == nil {
		t.Error("PublishedParsed is nil, want a parsed date")
	}

	second := feed.Items[1]
	if diff := cmp.Diff("Item two && more", second.Title); diff != "" {
		t.Errorf("item title mismatch (-want +got):\n%s", diff)
	}
	if second.Description != "" {
		t.Errorf("description = %q, want empty for an item without one", second.Description)
	}
}

// captureTransport stands in for http.DefaultTransport so the test observes
// the exact request the production client sends, without any network.
type captureTransport struct {
	gotUA string
	body  []byte
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.gotUA = req.Header.Get("User-Agent")

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(c.body)),
		Request:    req,
	}, nil
}

// redirectTransport serves a fixed redirect chain by path so the
// production CheckRedirect runs without any network.
type redirectTransport struct {
	locations map[string]string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if location, ok := t.locations[req.URL.Path]; ok {
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": {location}},
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Request:    req,
		}, nil
	}

	return nil, io.ErrUnexpectedEOF
}

// An https feed must not end on cleartext, wherever the downgrade hides:
// the first hop, or a later hop after an upgrade. The production
// CheckRedirect is kept; only the transport is stubbed.
func TestParserRefusesDowngradeAnywhereInChain(t *testing.T) {
	tests := []struct {
		name      string
		start     string
		locations map[string]string
		want      string
	}{
		{
			name:  "direct downgrade",
			start: "https://example.com/start.xml",
			locations: map[string]string{
				"/start.xml": "http://example.com/end.xml",
			},
			want: "refusing redirect",
		},
		{
			name:  "downgrade after upgrade",
			start: "http://example.com/start.xml",
			locations: map[string]string{
				"/start.xml": "https://example.com/mid.xml",
				"/mid.xml":   "http://example.com/end.xml",
			},
			want: "refusing redirect",
		},
		{
			name:  "chain stops at ten hops",
			start: "https://example.com/r0.xml",
			locations: func() map[string]string {
				chain := make(map[string]string, 11)
				for i := 0; i <= 10; i++ {
					chain["/r"+strconv.Itoa(i)+".xml"] = "https://example.com/r" + strconv.Itoa(i+1) + ".xml"
				}

				return chain
			}(),
			want: "stopped after 10 redirects",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := New()
			production := parser.Client
			parser.Client = &http.Client{
				Transport:     &redirectTransport{locations: test.locations},
				CheckRedirect: production.CheckRedirect,
			}

			_, err := parser.ParseURLWithContext(test.start, context.Background())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("err = %v, want %q", err, test.want)
			}
		})
	}
}

// Outlets see the app's User-Agent on feed fetches, not Go's default. The
// swap is restored before the test ends; package tests run sequentially.
func TestNewSendsUserAgent(t *testing.T) {
	feedBody, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixture.rss"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	capture := &captureTransport{body: feedBody}
	previous := http.DefaultTransport
	http.DefaultTransport = capture

	t.Cleanup(func() { http.DefaultTransport = previous })

	if _, err := New().ParseURLWithContext("http://example.com/feed", context.Background()); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	if capture.gotUA != useragent.UserAgent {
		t.Errorf("User-Agent = %q, want %q", capture.gotUA, useragent.UserAgent)
	}
}
