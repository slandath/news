# AI Agent Guidelines

This file provides instructions for AI coding assistants working on the RSS Feed Reader project. The codebase is pure Go (Go 1.27); there is no Node, TypeScript, or package-manager tooling left anywhere in the repo.

## Build, Lint, and Test Commands

### Development

- `go run .` - Start the dev server on `http://localhost:3000`
- `go tool templ generate` - **Required after editing any `.templ` file.** The generated `*_templ.go` files are committed; CI fails if they are stale
- `go tool templ fmt .` - Format the templ sources
- `gofmt -l .` - List Go files with formatting problems (`gofmt -w .` to fix)

### Testing

- **Required for every code change:** run `go test ./... -race` and verify it passes before finishing work
- All external I/O is stubbed: `internal/app` tests use a stub `http.RoundTripper` serving `testdata/`, so the suite runs offline
- The real `gofeed` parser and real scraper run against the fixtures — do not mock them
- Hostile-payload expectations live in `internal/fixtures` and `testdata/`; `scrape` and `app` tests share them
- `TestConfigsAndFixturesPair` in `internal/scrape/configs_test.go` enforces that every config has a fixture and vice versa — if you add a scrape target, you must add both
- Coverage: `go test ./... -race -coverprofile=cover.out && go tool cover -func=cover.out`

### Lint

- `go tool golangci-lint run ./...` - Config in `.golangci.yml`: standard set plus `revive`, `gocritic`, `bodyclose`, `noctx`
- `golangci-lint` is pinned as a `tool` directive in `go.mod` so Dependabot bumps it. After a bump, **re-run the whole test suite** — new linter versions can surface findings
- `go vet ./...` - Always run it

Formatting and linting are enforced in CI (`.github/workflows/ci.yml`): the staleness gate, `gofmt -l .`, `go tool templ fmt -fail .`, `go vet`, `-race` tests, `golangci-lint`, and govulncheck all block a merge. There is no separate lint workflow.

### Vulnerabilities

- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` — CI runs it; it reports only call-graph-reachable vulnerabilities, so a hit means upgrade now

## Non-Negotiable Invariants

### XSS safety

- **Never use `templ.Raw`.** Every value interpolated with `${}` in a `.templ` file is HTML-escaped by templ; `templ.Raw` bypasses that
- Scraped text stays plain text: extraction functions return `[]string` / `string` derived from node `.Text()` (goquery's text-only extraction), never from outer HTML
- Templates receive pre-built hrefs and plain strings only

### Error surface (parity with the original TS app)

The HTTP behaviors are deliberately mixed; don't "unify" them:

| Condition                   | Status  | Body                                         |
| --------------------------- | ------- | -------------------------------------------- |
| missing `url` param         | 400     | plain text `No URL found`                    |
| non-http(s) protocol        | 400     | plain text `Invalid URL protocol`            |
| malformed URL (no host)     | 400     | plain text `Invalid URL`                     |
| unconfigured feed `url`     | 400     | plain text `Invalid feed URL`, no outbound request |
| empty feed list             | 400     | plain text `URL undefined`                   |
| fetch/parse feed failure    | 502     | full error page, "Bad Feed"                  |
| article fetch timeout       | **504** | full error page, "Request Timeout"           |
| article fetch other failure | 302     | `Location: <source URL>`                     |
| unconfigured article domain | 302     | `Location: <source URL>`                     |
| panic                       | 500     | full error page, "Internal Server Error"     |
| unknown path                | (none)  | no catch-all 404 handler; Go default applies |

Rules behind the table:

- 400s are plain-text bodies written directly by handlers — never route them through the error template
- 302s use the hand-written `redirect()` helper (empty body). **Do not use `http.Redirect`** — it writes its own HTML body
- Timeout detection: `var ne net.Error; errors.As(err, &ne) && ne.Timeout()` via `scrape.IsTimeout`, or the `Client.Timeout` mechanism. **Never match on error strings**; client timeouts are `*url.Error` with `Timeout() == true`, not `context.DeadlineExceeded`
- Every error path calls `slog.Error(...)` before responding

### Feed data

- `internal/feeds.Feed` entries are in **hardcoded display order**. Never sort feeds at runtime — ICU `localeCompare` and Go's byte-order `<` disagree on this data, and the order is asserted by tests
- A configured feed's title overrides the feed's own `<title>`
- An absent/empty `?url=` on `/feed` falls back to `feeds.All[0]` with status 200 (not a 400)
- Only **configured** feed URLs are fetchable on `/feed`: `url.Values` are client-controlled and a feed URL outside `feeds.All` gets a plain-text 400 `Invalid feed URL` with no outbound request. This is a deliberate deviation from the TS app (which fetched any URL); approved during the pre-merge review
- Missing `pubDate` renders as an empty cell; never render the literal `undefined`

### URLs and hosts

- Hostname is lowercased before the `scrape.Configs` lookup; config keys are stored without `www.`
- Hrefs are built with `url.Values.Encode()` — which sorts params, so links emit `feed=…&url=…` order. This differs from the old TS output (`url=…&feed=…`); when asserting on rendered HTML, compare **decoded query values**, not raw href strings
- `&` in rendered hrefs appears as `&amp;` (correct). Test with html-escaped expectations or entity-decode first

### Code conventions

- All errors are returned explicitly; wrap with `%w`; match sentinels with `errors.Is`. Only `main.go` and the recover middleware may terminate a request path non-explicitly
- No `panic` outside `init`/startup code
- `context.Context` is always the first parameter
- Don't shadow builtins (`max`, `min`, `len`, `append`) as identifiers
- Functions get explicit return types; interfaces are defined at the consumer (`internal/app.Deps` is the wiring point for parser, scraper, favicon, feeds)
- Inject your `*http.Client` (parser, scraper) so tests can stub transports — never construct clients inside handlers
- Feeds, configs, and page templates take plain Go data; keep `internal/*` decoupled — `app` composes, sub-packages never import `app`

## Key Files and Their Purposes

- `main.go` - Entrypoint: `PORT` (default `3000`), route registration, graceful shutdown, embedded favicon
- `generate.go` - `go generate ./...` drives `go tool templ generate`
- `internal/app/app.go` - Routes, handlers, request logging + recovery middleware, href builders
- `internal/app/errors.go` - Error surfaces (plain-text 400s, `redirect()`, `writeHTML` for 502/504/500 pages)
- `internal/feeds/feeds.go` - The hardcoded feed list in display order
- `internal/rss/` - Feed fetching/timeout wrapper around `gofeed.Parser`
- `internal/scrape/configs.go` - Per-site selectors: `Title` and `ArticleWrapper`
- `internal/templates/` - `.templ` sources and committed generated output for home, feed, article, error pages
- `internal/fixtures/fixtures.go` - Hostile-payload expectation table shared by tests
- `testdata/` - Fixture HTML and RSS consumed by tests
- `go-migration-plan.md` - Historical plan for the TS→Go port; §8 records verified traps (templ escaping, sort order, rune slicing, timeout detection) still worth reading

## Project Overview

**RSS Feed Reader** is a lightweight web server (stdlib `net/http`) that:

1. Fetches RSS/Atom feeds from a hardcoded list of URLs
2. Parses the XML with `gofeed`
3. Extracts article text from linked URLs using site-specific goquery selectors
4. Strips all markup from third-party content so templ's escaping is the only line of defense
5. Renders pages with committed templ output

Endpoints: `/`, `/feed?url=`, `/article?url=` (optional `&feed=`), `/health`, `/favicon.ico`.

## Before Committing

1. `go tool templ generate` after any `.templ` edit — and commit the generated files. CI rejects stale output
2. Run `go test ./... -race` and keep it green. If a change intentionally alters behavior, update tests in the same commit — never commit red
3. `gofmt -l .` and `go tool templ fmt -fail .` must report nothing
4. `go vet ./...` and `go tool golangci-lint run ./...` must be quiet
5. New scrape target ⇒ config entry + `testdata/<site>.html` fixture + `internal/fixtures` entry, all three together
