package scrape

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/slandath/news/internal/fixtures"
	"github.com/slandath/news/internal/useragent"
)

// roundTripFunc lets a test stand in for the network, mirroring the TS suite's
// vi.stubGlobal('fetch', …) without mutating any global state.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func okResponse(req *http.Request, body io.Reader) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(body),
		Request:    req,
	}
}

func TestExtractReadsArticle(t *testing.T) {
	for domain, fx := range fixtures.Articles {
		t.Run(domain, func(t *testing.T) {
			cfg, ok := Configs[domain]
			if !ok {
				t.Fatalf("no config for %s", domain)
			}

			body, err := fixtures.Read(fx.File)
			if err != nil {
				t.Fatalf("read fixture %s: %v", fx.File, err)
			}
			var method string
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				method = req.Method
				if got, want := req.URL.Hostname(), "www."+domain; got != want {
					t.Errorf("hostname = %q, want %q", got, want)
				}
				if got, want := req.Header.Get("User-Agent"), useragent.UserAgent; got != want {
					t.Errorf("User-Agent = %q, want %q", got, want)
				}
				return okResponse(req, bytes.NewReader(body)), nil
			})}

			title, paragraphs, err := New(client).Extract(context.Background(), "https://www."+domain+"/story", cfg)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			if got, want := method, http.MethodGet; got != want {
				t.Errorf("method = %q, want %q", got, want)
			}

			if diff := cmp.Diff(fx.Title, title); diff != "" {
				t.Errorf("title mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(fx.Paragraphs, paragraphs); diff != "" {
				t.Errorf("paragraphs mismatch (-want +got):\n%s", diff)
			}

			for _, text := range append([]string{title}, paragraphs...) {
				if tagPattern.MatchString(text) {
					t.Errorf("markup survived extraction: %q", text)
				}
			}
		})
	}
}

// An origin error page is a fetch failure: the body is not parsed, so its
// markup can never render as if it were the article.
func TestExtractRejectsErrorStatus(t *testing.T) {
	body, err := fixtures.Read(fixtures.Articles["cnbc.com"].File)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		resp := okResponse(req, bytes.NewReader(body))
		resp.StatusCode = http.StatusNotFound

		return resp, nil
	})}

	title, paragraphs, err := New(client).Extract(context.Background(), "https://www.cnbc.com/story", Configs["cnbc.com"])
	if err == nil {
		t.Fatal("Extract accepted a 404 body, want an error")
	}

	if title != "" || len(paragraphs) != 0 {
		t.Errorf("Extract = (%q, %d paragraphs), want empty results on error status", title, len(paragraphs))
	}
}

func TestExtractRejectsMalformedURL(t *testing.T) {
	_, _, err := New(&http.Client{}).Extract(context.Background(), "://bad", Configs["cnbc.com"])
	if err == nil {
		t.Fatal("Extract accepted a URL with no scheme")
	}
}

func TestExtractWrapsReadError(t *testing.T) {
	sentinel := errors.New("connection reset")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return okResponse(req, errorReader{sentinel}), nil
	})}

	_, _, err := New(client).Extract(context.Background(), "https://www.cnbc.com/story", Configs["cnbc.com"])
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want it to wrap %v", err, sentinel)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestExtractWrapsTransportError(t *testing.T) {
	sentinel := errors.New("dial failed")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, sentinel
	})}

	_, _, err := New(client).Extract(context.Background(), "https://www.cnbc.com/story", Configs["cnbc.com"])
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want it to wrap %v", err, sentinel)
	}
}

// Proves the request carries the caller's context, which is what lets a
// client disconnect abort the upstream fetch.
func TestExtractPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	})}

	_, _, err := New(client).Extract(ctx, "https://www.cnbc.com/story", Configs["cnbc.com"])
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

type countingReader struct {
	reader io.Reader
	read   int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.read += n
	return n, err
}

func TestExtractCapsResponseBody(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), maxArticleBytes+4096)
	body := &countingReader{reader: bytes.NewReader(payload)}

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return okResponse(req, body), nil
	})}

	if _, _, err := New(client).Extract(context.Background(), "https://www.cnbc.com/story", Configs["cnbc.com"]); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got, want := body.read, maxArticleBytes; got != want {
		t.Errorf("read %d bytes, want the %d-byte cap", got, want)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unrelated", err: errors.New("connection refused"), want: false},
		{name: "dns failure", err: &net.DNSError{Err: "no such host", Name: "example.com"}, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "wrapped deadline exceeded", err: fmt.Errorf("read article: %w", context.DeadlineExceeded), want: true},
		{name: "client timeout", err: &url.Error{Op: "Get", URL: "https://example.com", Err: timeoutError{}}, want: true},
		{name: "wrapped client timeout", err: fmt.Errorf("fetch article: %w", &url.Error{Op: "Get", URL: "https://example.com", Err: timeoutError{}}), want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsTimeout(test.err); got != test.want {
				t.Errorf("IsTimeout(%v) = %t, want %t", test.err, got, test.want)
			}
		})
	}
}
