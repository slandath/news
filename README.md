# RSS Feed Reader

A lightweight web server that fetches RSS feeds, parses the XML, and renders
readable feed index and article pages.

## Features

- Parse RSS and Atom feeds from a configured list of outlets
- Server-side rendered templates via [templ](https://github.com/a-h/templ)
- Extract article text from configured outlets with all markup stripped
- Redirect unconfigured outlets to their source URL

## Tech Stack

[![Go](https://img.shields.io/badge/Go-1.27.3-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![templ](https://img.shields.io/badge/templ-0.3.1100-blue?style=for-the-badge)](https://github.com/a-h/templ)
[![gofeed](https://img.shields.io/badge/gofeed-1.5.0-black?style=for-the-badge)](https://github.com/mmcdole/gofeed)
[![goquery](https://img.shields.io/badge/goquery-1.13.0-CD853F?style=for-the-badge)](https://github.com/PuerkitoBio/goquery)
[![Bulma](https://img.shields.io/badge/Bulma-1.0.4-yellow?style=for-the-badge&logo=css&logoColor=white)](https://bulma.io)
[![golangci-lint](https://img.shields.io/badge/golangci--lint-2.15.0-4b3a8a?style=for-the-badge)](https://golangci-lint.run)

## Getting Started

Requires [Go 1.27.3+](https://go.dev/dl/).

```bash
go run .
```

Visit `http://localhost:3000` to browse RSS feeds.

### Environment Variables

Available environment variables:

- `PORT` - Server port (default: `3000`), see `.env.example`

Railway, Heroku, and Render set `PORT` for you; the server picks it up automatically.

## Endpoints

- `GET /` - Home page listing all feeds
- `GET /feed?url=<RSS_URL>` - Items for a feed (empty or missing `url` falls back to the first configured feed; only configured feed URLs are fetchable, anything else gets a 400)
- `GET /article?url=<ARTICLE_URL>` - Extracted text for an article; unconfigured outlets redirect to the source URL
- `GET /health` - Health check for the deploy platform
- `GET /favicon.ico` - Favicon, embedded in the binary

## Caching

Parsed feeds are cached in memory for 5 minutes and scraped articles for
1 hour, keyed by URL. Successful `/feed` and `/article` responses also send
`Cache-Control` so browsers reuse them, with `max-age` derived from the
entry's remaining lifetime (never more than the TTL). Non-2xx origin bodies
still render but are neither cached nor advertised as cacheable. The article
cache is capped at 1000 entries, evicting expired then soonest-expiring
first, since article URLs are caller-controlled; feed URLs come from the
closed configured list. Failures are never cached.

## Development

```bash
go run .                 # start the dev server on :3000
go tool templ generate   # regenerate *_templ.go after editing .templ files
gofmt -l .               # list files with formatting problems
go tool templ fmt .      # format the templ sources
go vet ./...
go test ./... -race
go tool golangci-lint run ./...
```

The generated `*_templ.go` files are committed so the production image never
needs the templ CLI; CI fails if you edit a `.templ` file without regenerating.

`golangci-lint` is pinned via a `tool` directive in `go.mod`, so
[Dependabot](https://docs.github.com/en/code-security/dependabot) proposes
bumps like any other module. After a bump, re-run the whole suite — new
linter versions can surface findings.

## Project Structure

- `main.go` - Server entrypoint: reads `PORT`, wires routes, graceful shutdown
- `generate.go` - `go generate ./...` drives the templ CLI
- `internal/app` - Routes, handlers, logging, panic recovery, error rendering
- `internal/cache` - Generic in-memory TTL cache backing feeds and articles
- `internal/rss` - Feed fetching and parsing ([gofeed](https://github.com/mmcdole/gofeed))
- `internal/scrape` - Article extraction ([goquery](https://github.com/PuerkitoBio/goquery)) and per-site selectors
- `internal/feeds` - The hardcoded feed list in display order
- `internal/templates` - Page templates (`.templ` sources plus committed output)
- `internal/fixtures` - Shared hostile-payload expectations for tests
- `testdata/` - Fixture HTML and RSS used by tests
- `.github/workflows/ci.yml` - Staleness, formatting, vet, race tests, lint, vulnerability scan

## Deployment

Deploys to [Railway](https://railway.com) with
[Railpack](https://railpack.com), which builds the Go binary automatically.
Set the healthcheck path to `/health` on the service.
