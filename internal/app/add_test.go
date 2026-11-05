package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/slandath/news/internal/app"
	"github.com/slandath/news/internal/cache"
	"github.com/slandath/news/internal/feeds"
	"github.com/slandath/news/internal/fixtures"
	"github.com/slandath/news/internal/rss"
	"github.com/slandath/news/internal/scrape"
)

var errOffline = errors.New("offline")

// newAddTestHandler mirrors newTestHandler with a feed store path, so add
// tests prove persistence without touching other tests' expectations.
func newAddTestHandler(t *testing.T, transport http.RoundTripper, storePath string) http.Handler {
	t.Helper()

	parser := rss.New()
	parser.Client = &http.Client{Transport: transport}

	return app.New(app.Deps{
		Feeds:         feeds.All,
		Configs:       scrape.Configs,
		Parser:        parser,
		Scraper:       scrape.New(&http.Client{Transport: transport}),
		Favicon:       []byte("icon"),
		FeedCache:     cache.New[*gofeed.Feed](app.FeedCacheTTL),
		FeedStorePath: storePath,
	})
}

func postForm(t *testing.T, handler http.Handler, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

// feedStub serves the shared RSS fixture for one extra URL on top of the
// standard corpus, so a brand-new feed URL validates and parses.
func feedStub(t *testing.T, newURL string) *stubTransport {
	t.Helper()

	transport := newStubTransport(t)
	rssBody, err := fixtures.Read(fixtures.RSS)
	if err != nil {
		t.Fatalf("read %s: %v", fixtures.RSS, err)
	}

	previous := transport.handle
	transport.handle = func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == newURL {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(rssBody)),
				Request:    req,
			}, nil
		}

		return previous(req)
	}

	return transport
}

func TestAddFormRenders(t *testing.T) {
	handler := newAddTestHandler(t, newStubTransport(t), "")

	rec := do(t, handler, "/add")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	for _, want := range []string{`<form`, `name="url"`, `name="section"`, `>News</option>`, `>Sports</option>`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("form is missing %q", want)
		}
	}

	// The add link lives in the header on every other page, never here:
	// the header label already names this page. Match the element, not
	// the substring: the inlined stylesheet mentions nav-add on all pages.
	if strings.Contains(rec.Body.String(), `<li class="pure-menu-item nav-add">`) {
		t.Error("add page renders its own nav link")
	}
}

func TestAddFeedAddsValidFeed(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "user-feeds.json")
	newURL := "https://example.com/custom.xml"
	stub := feedStub(t, newURL)
	handler := newAddTestHandler(t, stub, storePath)

	rec := postForm(t, handler, "/add", url.Values{
		"url":     {newURL},
		"section": {"Sports"},
	})

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	if got, want := rec.Header().Get("Location"), feedHref(newURL); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	// The shared RSS fixture titles itself "Fixture Feed & Wire".
	home := do(t, handler, "/")
	if !strings.Contains(home.Body.String(), "Fixture Feed &amp; Wire") {
		t.Error("home page is missing the added feed")
	}

	if !strings.Contains(home.Body.String(), `<h2 class="section-title">Sports</h2>`) {
		t.Error("home page is missing the Sports section")
	}

	stored, err := feeds.LoadUserFeeds(storePath)
	if err != nil {
		t.Fatalf("load stored feeds: %v", err)
	}

	if len(stored) != 1 || stored[0].URL != newURL || stored[0].Section != feeds.SectionSports {
		t.Errorf("stored feeds = %+v, want the one added feed", stored)
	}

	// Adding the same URL again reports the duplicate; nothing is saved twice.
	dup := postForm(t, handler, "/add", url.Values{
		"url":     {newURL},
		"section": {"Sports"},
	})
	if dup.Code != http.StatusBadRequest {
		t.Fatalf("duplicate status = %d, want %d", dup.Code, http.StatusBadRequest)
	}

	if !strings.Contains(dup.Body.String(), "already listed") {
		t.Error("duplicate response is missing the explanation")
	}

	stored, err = feeds.LoadUserFeeds(storePath)
	if err != nil {
		t.Fatalf("reload stored feeds: %v", err)
	}

	if len(stored) != 1 {
		t.Errorf("stored %d feeds after duplicate, want 1", len(stored))
	}

	// The add seeds the feed cache: with the origin down, the redirect
	// target still renders from the just-fetched feed.
	stub.handle = func(*http.Request) (*http.Response, error) { return nil, errOffline }

	seeded := do(t, handler, "/feed?url="+url.QueryEscape(newURL))
	if seeded.Code != http.StatusOK {
		t.Fatalf("cached feed status = %d, want %d", seeded.Code, http.StatusOK)
	}
}

// An omitted section defaults to News instead of failing validation. The
// heading alone would not prove it: the built-in News feeds always render
// one, so the stored section is asserted too.
func TestAddFeedDefaultsSection(t *testing.T) {
	newURL := "https://example.com/nodefault.xml"
	storePath := filepath.Join(t.TempDir(), "user-feeds.json")
	handler := newAddTestHandler(t, feedStub(t, newURL), storePath)

	rec := postForm(t, handler, "/add", url.Values{"url": {newURL}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	home := do(t, handler, "/").Body.String()
	if !strings.Contains(home, `<h2 class="section-title">News</h2>`) {
		t.Error("added feed did not land in the News section")
	}

	stored, err := feeds.LoadUserFeeds(storePath)
	if err != nil {
		t.Fatalf("load stored feeds: %v", err)
	}

	if len(stored) != 1 || stored[0].Section != feeds.SectionNews {
		t.Errorf("stored feeds = %+v, want one feed in the News section", stored)
	}
}

// Scheme and host compare case-insensitively, so a differently cased
// spelling of a listed URL is still a duplicate.
func TestAddFeedRejectsCaseVariantDuplicate(t *testing.T) {
	newURL := "https://example.com/case.xml"
	handler := newAddTestHandler(t, feedStub(t, newURL), "")

	if rec := postForm(t, handler, "/add", url.Values{"url": {newURL}, "section": {"News"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("first add status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	dup := postForm(t, handler, "/add", url.Values{"url": {"https://EXAMPLE.com/case.xml"}, "section": {"News"}})
	if dup.Code != http.StatusBadRequest {
		t.Fatalf("duplicate status = %d, want %d", dup.Code, http.StatusBadRequest)
	}

	if !strings.Contains(dup.Body.String(), "already listed") {
		t.Error("duplicate response is missing the explanation")
	}
}

// A default port and an empty path are the same feed as the bare host and
// "/", so resubmits under either spelling are duplicates.
func TestAddFeedRejectsNormalizedDuplicates(t *testing.T) {
	tests := []struct {
		name  string
		first string
		dup   string
	}{
		{name: "default https port", first: "https://example.com/port.xml", dup: "https://example.com:443/port.xml"},
		{name: "empty path", first: "https://example.com/", dup: "https://example.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newAddTestHandler(t, feedStub(t, test.first), "")

			if rec := postForm(t, handler, "/add", url.Values{"url": {test.first}, "section": {"News"}}); rec.Code != http.StatusSeeOther {
				t.Fatalf("first add status = %d, want %d", rec.Code, http.StatusSeeOther)
			}

			dup := postForm(t, handler, "/add", url.Values{"url": {test.dup}, "section": {"News"}})
			if dup.Code != http.StatusBadRequest {
				t.Fatalf("duplicate status = %d, want %d", dup.Code, http.StatusBadRequest)
			}

			if !strings.Contains(dup.Body.String(), "already listed") {
				t.Error("duplicate response is missing the explanation")
			}
		})
	}
}

// A cross-site page can auto-submit the form; browsers label such
// requests, and they are rejected before any validation runs.
func TestAddFeedRejectsCrossSiteRequest(t *testing.T) {
	newURL := "https://example.com/xsite.xml"
	handler := newAddTestHandler(t, feedStub(t, newURL), "")

	form := url.Values{"url": {newURL}, "section": {"News"}}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if !strings.Contains(rec.Body.String(), "come from this site") {
		t.Error("response is missing the cross-site explanation")
	}

	home := do(t, handler, "/").Body.String()
	if strings.Contains(home, "example.com/xsite.xml") {
		t.Error("cross-site submit leaked onto the home page")
	}
}

// The open form cannot grow the feed list without bound: past the maximum,
// adds are rejected and the stored file is untouched.
func TestAddFeedRejectsOverLimit(t *testing.T) {
	rssBody, err := fixtures.Read(fixtures.RSS)
	if err != nil {
		t.Fatalf("read %s: %v", fixtures.RSS, err)
	}

	transport := &stubTransport{handle: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(rssBody)),
			Request:    req,
		}, nil
	}}
	storePath := filepath.Join(t.TempDir(), "user-feeds.json")
	handler := newAddTestHandler(t, transport, storePath)

	fill := func(i int) *httptest.ResponseRecorder {
		return postForm(t, handler, "/add", url.Values{
			"url":     {fmt.Sprintf("https://example.com/feed-%d.xml", i)},
			"section": {"News"},
		})
	}

	for i := 0; i < app.MaxUserFeeds; i++ {
		if rec := fill(i); rec.Code != http.StatusSeeOther {
			t.Fatalf("add %d status = %d, want %d", i, rec.Code, http.StatusSeeOther)
		}
	}

	over := fill(app.MaxUserFeeds)
	if over.Code != http.StatusBadRequest {
		t.Fatalf("over-limit status = %d, want %d", over.Code, http.StatusBadRequest)
	}

	if !strings.Contains(over.Body.String(), "maximum number of feeds") {
		t.Error("over-limit response is missing the explanation")
	}

	stored, err := feeds.LoadUserFeeds(storePath)
	if err != nil {
		t.Fatalf("load stored feeds: %v", err)
	}

	if len(stored) != app.MaxUserFeeds {
		t.Errorf("stored %d feeds, want %d", len(stored), app.MaxUserFeeds)
	}
}

func TestAddFeedRejectsBadInput(t *testing.T) {
	rssBody, err := fixtures.Read(fixtures.RSS)
	if err != nil {
		t.Fatalf("read %s: %v", fixtures.RSS, err)
	}

	transport := &stubTransport{handle: func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://example.com/good.xml", "https://example.com/notitle.xml", "https://example.com/garbage.xml", "https://example.com/blanktitle.xml":
			body := rssBody
			if strings.Contains(req.URL.String(), "notitle") {
				body = []byte(`<?xml version="1.0"?><rss version="2.0"><channel><link>https://example.com/</link></channel></rss>`)
			}
			if strings.Contains(req.URL.String(), "garbage") {
				body = []byte("not xml at all")
			}
			if strings.Contains(req.URL.String(), "blanktitle") {
				body = []byte(`<?xml version="1.0"?><rss version="2.0"><channel><title><![CDATA[   ]]></title><link>https://example.com/</link></channel></rss>`)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Request:    req,
			}, nil
		default:
			return nil, errOffline
		}
	}}
	handler := newAddTestHandler(t, transport, filepath.Join(t.TempDir(), "user-feeds.json"))

	tests := []struct {
		name    string
		form    url.Values
		message string
	}{
		{name: "empty url", form: url.Values{"url": {""}}, message: "Enter a feed URL"},
		{name: "http url", form: url.Values{"url": {"http://example.com/feed"}}, message: "https://"},
		{name: "malformed url", form: url.Values{"url": {"http://"}}, message: "https://"},
		{name: "bad section", form: url.Values{"url": {"https://example.com/good.xml"}, "section": {"News; DROP"}}, message: "News or Sports"},
		{name: "unreachable", form: url.Values{"url": {"https://example.com/offline.xml"}, "section": {"News"}}, message: "Could not fetch"},
		{name: "unparsable", form: url.Values{"url": {"https://example.com/garbage.xml"}, "section": {"News"}}, message: "Check the URL"},
		{name: "no title", form: url.Values{"url": {"https://example.com/notitle.xml"}, "section": {"News"}}, message: "no title"},
		{name: "blank title", form: url.Values{"url": {"https://example.com/blanktitle.xml"}, "section": {"News"}}, message: "no title"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := postForm(t, handler, "/add", test.form)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}

			if !strings.Contains(rec.Body.String(), test.message) {
				t.Errorf("response is missing %q", test.message)
			}

			if !strings.Contains(rec.Body.String(), "<form") {
				t.Error("response does not re-render the form")
			}
		})
	}

	home := do(t, handler, "/")
	if strings.Contains(home.Body.String(), "example.com") {
		t.Error("rejected feeds leaked onto the home page")
	}
}

func TestHomeMergesRepeatedSections(t *testing.T) {
	transport := newStubTransport(t)
	parser := rss.New()
	parser.Client = &http.Client{Transport: transport}

	handler := app.New(app.Deps{
		Feeds: []feeds.Feed{
			{Title: "News One", URL: feeds.All[0].URL, Section: feeds.SectionNews},
			{Title: "Sports One", URL: feeds.All[8].URL, Section: feeds.SectionSports},
			{Title: "News Two", URL: feeds.All[1].URL, Section: feeds.SectionNews},
		},
		Configs: scrape.Configs,
		Parser:  parser,
		Scraper: scrape.New(&http.Client{Transport: transport}),
		Favicon: []byte("icon"),
	})

	body := do(t, handler, "/").Body.String()

	if got, want := strings.Count(body, `<h2 class="section-title">News</h2>`), 1; got != want {
		t.Errorf("rendered %d News headings, want %d (sections must merge)", got, want)
	}

	if got, want := strings.Count(body, `<h2 class="section-title">Sports</h2>`), 1; got != want {
		t.Errorf("rendered %d Sports headings, want %d", got, want)
	}

	// The heading counts alone would pass if a feed card went missing, so
	// both News cards are asserted directly.
	for _, title := range []string{"News One", "News Two", "Sports One"} {
		if !strings.Contains(body, title) {
			t.Errorf("home page is missing the %q feed card", title)
		}
	}
}
