package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

// httpError is an error that carries the status and message to show the user.
// Handlers return it and the middleware renders it with the error template.
//
// Plain-text responses (the 400s) are written by the handlers themselves and
// return nil, matching the TypeScript app's c.text() calls.
type httpError struct {
	Status int
	Msg    string
}

func (e *httpError) Error() string { return e.Msg }

func writeText(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)

	if _, err := io.WriteString(w, msg); err != nil {
		slog.Error("write response", "err", err)
	}
}

// writeHTML renders component into a buffer first, so a render failure can
// still become a clean error response instead of a half-written page.
func writeHTML(ctx context.Context, w http.ResponseWriter, status int, component templ.Component) error {
	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		return fmt.Errorf("render: %w", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

// redirect sends a 302 with an empty body. http.Redirect would add a short
// HTML body with a link, which the TypeScript app never did.
func redirect(w http.ResponseWriter, location string) {
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusFound)
}
