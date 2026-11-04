package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slandath/news/internal/app"
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
}

func TestAddFeedAddsValidFeed(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "user-feeds.json")
	newURL := "https://example.com/custom.xml"
	handler := newAddTestHandler(t, feedStub(t, newURL), storePath)

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
}

// An omitted section defaults to News instead of failing validation.
func TestAddFeedDefaultsSection(t *testing.T) {
	newURL := "https://example.com/nodefault.xml"
	handler := newAddTestHandler(t, feedStub(t, newURL), "")

	rec := postForm(t, handler, "/add", url.Values{"url": {newURL}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	home := do(t, handler, "/").Body.String()
	if !strings.Contains(home, `<h2 class="section-title">News</h2>`) {
		t.Error("added feed did not land in the News section")
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
}
