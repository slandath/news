package useragent_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/slandath/news/internal/useragent"
)

// stubTransport records the request it receives without touching the network.
type stubTransport struct {
	request *http.Request
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s.request = req

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("ok")),
		Request:    req,
	}, nil
}

func TestTransportSetsUserAgent(t *testing.T) {
	stub := &stubTransport{}
	client := &http.Client{Transport: useragent.Transport(stub)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/feed", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Keep-Me", "yes")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()

	if got, want := stub.request.Header.Get("User-Agent"), useragent.UserAgent; got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}

	if got, want := stub.request.Header.Get("X-Keep-Me"), "yes"; got != want {
		t.Errorf("X-Keep-Me = %q, want %q (caller headers must survive)", got, want)
	}

	if got := req.Header.Get("User-Agent"); got != "" {
		t.Errorf("caller request User-Agent = %q, want empty (must not mutate the caller)", got)
	}
}

func TestTransportDefaultsToDefaultTransport(t *testing.T) {
	if useragent.Transport(nil) == nil {
		t.Error("Transport(nil) is nil, want a usable RoundTripper")
	}
}
