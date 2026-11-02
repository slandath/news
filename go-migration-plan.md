# Plan: Port `news` (RSS reader) from TypeScript/Hono to Go

Status: **decisions locked**, v2 (2026-11-01)
Supersedes: v1 (draft)
Repo: `slandath/news` @ `main` (`4f67fd1`)
Deploy target: Railway project `news`, service `news`, custom domain `news.tomslanda.com` → container port **3000**

---

## 1. Goal & scope

Replace the Node/TypeScript implementation with an equivalent Go service that:

- serves the same routes with the same status codes, redirects, and escaped output,
- keeps the same scraping configs and the same text-only extraction guarantee (no raw HTML from third parties ever reaches the response),
- ships as a single static binary with templates embedded,
- keeps a test suite of at least equal coverage, ported test-for-test,
- deploys to Railway with zero build-config surprises.

**In scope:** `src/`, `tests/`, build/lint/test tooling, CI, devcontainer, README/AGENTS docs, Railway service settings.

**Out of scope:** issue #30 (caching), issue #19 (sports feeds), UI redesign. Re-target both at the Go codebase after cutover.

**Non-goal:** byte-identical HTML. Parity is defined by §7's behavior matrix, not raw bytes.

---

## 2. Locked decisions

| #    | Decision        | Resolution                                                                                                                                       |
| ---- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| 2.1  | Templating      | **`templ`** v0.3.1100. Generated `*_templ.go` **committed**; CI verifies staleness.                                                              |
| 2.2  | Router          | stdlib `net/http` + Go 1.22 `ServeMux` (`mux.HandleFunc("GET /feed", …)`). No router dependency.                                                 |
| 2.3  | Repo strategy   | Long-lived `go-rewrite` branch; TS tree deleted in a separate commit after a green deploy.                                                       |
| 2.4  | Port            | **3000**, matching `src/index.ts`, README, devcontainer, and the live Railway domain mapping. Fix `.env.example` + `AGENTS.md` (currently 4000). |
| 2.5  | Article timeout | **Fix to 504.** Update the ported test.                                                                                                          |
| 2.6  | Feed order      | Hardcode the slice in display order; **delete the runtime sort**. (§8.2)                                                                         |
| 2.7  | `pubDate`       | Keep `gofeed`'s raw `Item.Published` string for parity.                                                                                          |
| 2.8  | Module path     | `github.com/slandath/news`.                                                                                                                      |
| 2.9  | Go version      | **1.27.x** (latest stable 1.27.3). Floor is 1.26 — templ v0.3.1100's own `go.mod` requires it. **Local toolchain must be upgraded from 1.24.1.** |
| 2.10 | Feeds source    | **Hardcoded** in `internal/feeds/feeds.go` (parity). Rationale in §12.2.                                                                         |
| 2.11 | Dockerfile      | **None.** Railway's Railpack builds Go natively; see §12.                                                                                        |

---

## 3. Target layout

```
news/
├── go.mod                          # go 1.27.0, templ as runtime dep + tool
├── go.sum
├── main.go                         # PORT, http.Server, graceful shutdown
├── generate.go                     # //go:generate go tool templ generate
├── internal/
│   ├── app/
│   │   ├── app.go                  # routes, middleware (recover + error render), handlers
│   │   ├── app_test.go             # ports routes.test.ts
│   │   └── errors.go               # httpError{status, message}
│   ├── feeds/
│   │   ├── feeds.go                # pre-sorted feed list
│   │   └── feeds_test.go
│   ├── rss/
│   │   └── parser.go               # FeedParser interface + gofeed impl
│   ├── scrape/
│   │   ├── configs.go              # siteConfigs
│   │   ├── configs_test.go
│   │   └── scrape.go               # fetch + extract
│   └── templates/
│       ├── templates.go            # //go:embed, Render helper
│       ├── templates_test.go
│       ├── home.templ              + home_templ.go        (generated, committed)
│       ├── feed.templ              + feed_templ.go
│       ├── article.templ           + article_templ.go
│       └── error.templ             + error_templ.go
├── testdata/                       # article + RSS fixtures (ports tests/fixtures.ts)
├── favicon.ico                     # unchanged, //go:embed'd
└── .github/workflows/ci.yml        # replaces linting.yml
```

Notes:

- `testdata/` is a toolchain convention — the `go` command ignores it, which retires the `AGENTS.md` warning about colocating tests under `src/templates/`.
- `internal/` keeps packages unimportable from outside the module. Costs nothing, discourages coupling.
- One package per concern, not per file. Go favors fewer, larger packages; don't port the file-per-module shape literally.

---

## 4. Dependencies

Versions verified against `proxy.golang.org` and by actually building a scratch templ module on 2026-11-01.

| Module                           | Version   | Replaces             | Notes                                                                   |
| -------------------------------- | --------- | -------------------- | ----------------------------------------------------------------------- |
| `github.com/a-h/templ`           | v0.3.1100 | `hono/html`          | Runtime + codegen. **Requires Go ≥ 1.26.**                              |
| `github.com/mmcdole/gofeed`      | v1.5.0    | `rss-parser` 3.13    | RSS 0.9x/1.0/2.0, Atom, JSON Feed. Typed `*gofeed.Feed`.                |
| `github.com/PuerkitoBio/goquery` | v1.13.0   | `cheerio` 1.2        | Wraps `x/net/html` + `cascadia`. `Selection.Text()` ≡ `cheerio.text()`. |
| `golang.org/x/net`               | v0.62.0   | (transitive)         | HTML parser; pinned by goquery.                                         |
| `github.com/google/go-cmp`       | v0.7.0    | `expect().toEqual()` | `cmp.Diff` for slice/struct assertions.                                 |

Tooling, pinned via the Go 1.24+ `tool` directive so versions can't drift from `go.mod`:

```
go get -tool github.com/a-h/templ/cmd/templ@v0.3.1100
```

invoked as `go tool templ generate`. This solves templ's usual "generator version must match module version" hazard without a second install step in CI or Docker.

Not needed: router, test framework, assertion library, template engine, linter config, formatter, lockfile tool, watcher. That's 5 direct deps vs 5 runtime + 9 dev + 11 override pins today.

---

## 5. templ workflow (new section — this is the main cost of 2.1)

**Setup, verified working:**

```bash
go mod init github.com/slandath/news
go get github.com/a-h/templ@v0.3.1100
go get -tool github.com/a-h/templ/cmd/templ@v0.3.1100
```

`go get -tool` writes a `tool` directive into `go.mod`; `go tool templ generate` then works from a clean clone with no global install.

**Commit policy: commit the generated `*_templ.go` files.** Reasoning:

- Railway's Railpack runs `go build` and nothing else — it will not run `templ generate`. Committed generated code means the existing RAILPACK builder keeps working and **no Dockerfile is required** (§12).
- `go build ./...` and `go test ./...` work for any contributor (and for gopls) without a codegen step.
- The usual objection — stale generated files — is handled by a CI staleness gate rather than by hiding the code.

**CI staleness gate** (in `ci.yml`, after `go tool templ generate`):

```bash
test -z "$(git status --porcelain)" || (git status --porcelain && echo "generated templ files are stale" && exit 1)
```

**Dev loop:** `templ generate --watch` in one terminal + `go run .` in another, or just `templ generate && go run .`. Also add `templ fmt` to the CI format gate alongside `gofmt` — templ files are not gofmt-formatted.

**Things templ forbids** (design constraints to know before phase 5):

- No expressions inside `<script>` or `<style>` blocks — only constants. Your templates have no interpolated script/style, so this is a non-issue, but it's why templ can guarantee safety there.
- `on*` attributes only accept `templ.ComponentScript`, not strings.
- `style={expr}` is rejected; style attributes must be constant or use `css` blocks.
- `class={expr}` is sanitized; `templ.SafeClass` bypasses sanitization (still escaped).
- `href`/`src`/`action`/`formaction` values are sanitized by `templ.URL` unless wrapped in `templ.SafeURL`. Same protection class as `html/template`, different mechanism.
- `templ.Raw` renders unescaped HTML — **the exact equivalent of Hono's `raw()`. Ban it in this project** (put it in AGENTS.md next to the XSS section).

---

## 6. Phases

Each phase ends with green `go build ./... && go test ./... && go vet ./...` and its own commit on `go-rewrite`.

### Phase 0 — Baseline capture (½ day)

1. `git switch -c go-rewrite`.
2. `curl` each route against the live feeds, saving status + `Location` + normalized body to `/tmp/parity-baseline/`: `GET /`, `GET /feed`, `GET /feed?url=<each of 8>`, `GET /article?url=<one per configured site>`, the four 400 cases, the 302 cases, `GET /favicon.ico`.
3. Copy the hostile payloads from `tests/fixtures.ts` verbatim into `testdata/*.html`. This is the security regression corpus — do not retype from memory.

Exit: baseline recorded; fixtures in `testdata/`.

### Phase 1 — Scaffold (½ day)

1. **Upgrade local Go to 1.27.x** (`brew upgrade go`). Verify `go version`. Optionally set `GOTOOLCHAIN=local` so a version drift fails loudly instead of silently downloading.
2. `go mod init github.com/slandath/news`; `go 1.27.0`.
3. Add templ + tool directive (§5), write one placeholder `.templ`, confirm `go tool templ generate && go build` round-trips.
4. `main.go`: `PORT` env, default `3000`; `&http.Server{Addr: ":" + port}`; `signal.NotifyContext(os.Interrupt, syscall.SIGTERM)` + `srv.Shutdown(ctx)` with a drain timeout. Binding `:3000` listens on all interfaces — required by Railway; never bind `127.0.0.1`.
5. `internal/templates/templates.go`: `//go:embed favicon.ico *.templ` is not possible (embed takes compiled-in assets, not template sources) — embed `favicon.ico` only; templ components are Go functions, so there is nothing to parse at runtime at all. This is strictly simpler than `html/template`.

Exit: `go run .` serves 200 on `/`; `go build` from a clean clone produces a working binary runnable from any CWD.

### Phase 2 — Static data (½ day)

1. `internal/feeds/feeds.go`: `var All = []Feed{…}` in final display order, `Feed{Title, URL}`. Drop `encodedURL` — it existed only to pre-escape for the TS template.
2. `internal/scrape/configs.go`:

   ```go
   type SiteConfig struct {
       Article string
       Title   string
       ArticleWrapper func(*goquery.Document) []string
   }
   ```

   `ArticleWrapper` ports 1:1 — Go structs hold funcs natively.

3. Port the five configs verbatim. Selectors are CSS2/3 (`p.article-text`, `div.group p`, `div#article-body p`, `h1.headline`) — all supported by cascadia.
4. Port `feeds_test.go` + `configs_test.go`.

### Phase 3 — RSS layer (½ day)

```go
type Parser interface { ParseURL(url string) (*gofeed.Feed, error) }
```

`*gofeed.Parser` satisfies this as-is — no adapter, and no `vi.mock('rss-parser')` equivalent. Real impl should use `ParseURLWithContext` so the 502 path can't hang; keep the interface narrow. Delete the `ParserFeed`/`BaseFeed` optional-field interfaces — `gofeed.Feed` is already typed.

### Phase 4 — Scrape layer (1 day)

1. `internal/scrape/scrape.go`: `func (s *Scraper) Extract(ctx context.Context, rawURL string, cfg SiteConfig) (title string, paragraphs []string, err error)`.
2. `&http.Client{Timeout: 5 * time.Second}`, injectable. Covers connect + headers + body, which is what the current `AbortController` intends. See §8.4 for error detection.
3. `goquery.NewDocumentFromReader(bytes.NewReader(body))`, then `doc.Find(cfg.Article).Each(...)` collecting `Selection.Text()` — mirrors `.map((_, el) => $(el).text()).get()`.
4. Enforce the text-only invariant structurally: `ArticleWrapper` returns `[]string`, never `templ.Component`/`template.HTML`. There is no type-level path from scraped text to raw output.
5. `io.LimitReader` cap (10 MB) — the current `await response.text()` is an unbounded allocation on a hostile page.

### Phase 5 — templ components (1–1.5 days)

Structure mapping:

| TS                           | templ                                            |
| ---------------------------- | ------------------------------------------------ |
| `html\`…${x}…\``             | `{ x }`                                          |
| `${arr.map(v => html\`…\`)}` | `for _, v := range arr { … }`                    |
| `${cond ? html\`…\` : ''}`   | `if cond { … }`                                  |
| `truncate(title, 30)`        | Go helper called inside `{ }`, rune-based (§8.3) |
| `ArticleTemplateProps`       | `templ Article(props ArticleProps)`              |

Verified escaping behavior (rendered from a scratch module, templ v0.3.1100):

```
href={ "/article?url=" + url.QueryEscape(raw) }  →  /article?url=https%3A%2F%2Fexample.com%2Fa%3Fb%3D1%26c%3D2
href={ "https://example.com/a?b=1&c=2" }         →  https://example.com/a?b=1&amp;c=2
href={ "javascript:alert(1)" }                   →  about:invalid#TemplFailedSanitizationURL
{ raw }                                          →  https://example.com/a?b=1&amp;c=2
```

**No double-encoding.** The pre-encoded-interpolation pattern ports directly, unlike `html/template` (§8.1). Still prefer building the full URL in Go so the value is testable as one string.

Also: templ components are plain Go functions, so the ported `templates_test.go` renders into a `bytes.Buffer` via `Component.Render(ctx, &buf)` — no template parsing, no `Execute` error surface.

### Phase 6 — Handlers, routing, errors (1 day)

1. `internal/app/app.go`: `func New(deps) http.Handler` returning a `*http.ServeMux` with injected parser, `*http.Client`, config map, feed list. No package-level globals — the main testability win over `export const app`.
2. Routes: `GET /`, `GET /feed`, `GET /article`, `GET /favicon.ico`, **`GET /health`** (new — returns 200 `ok`, no dependencies touched; required for Railway zero-downtime deploys, §12).
3. `/` — build `[]FeedLink{Title, Href}` where `Href = "/feed?" + url.Values{"url": {f.URL}}.Encode()`. Note `Values.Encode()` sorts keys, so multi-param links come out `feed=…&url=…` where TS emitted `url=…&feed=…` (§8.5).
4. `/feed` — `url` param, fall back to `feeds.All[0].URL`; empty → 400 `URL undefined`; parse error → 502 via `error.templ`.
5. `/article` — 400 `No URL found` / 400 `Invalid URL protocol` / 400 `Invalid URL`; unknown domain → 302; fetch+extract; timeout → **504** (§8.4); other errors → 302 fallback.
6. Errors: `httpError{Status, Msg}` + `func(w, r) error` adapter + one middleware that renders `error.templ` and `recover()`s panics into a 500. `net/http` recovers panics per connection but logs and closes without a body — the middleware is what produces the 500 page.
7. `log/slog` with method, path, status, duration. Upgrade from bare `console.error`, not scope creep.

### Phase 7 — Test port (1 day)

| TS                          | Go                                     | Notes                                          |
| --------------------------- | -------------------------------------- | ---------------------------------------------- |
| `tests/feeds.test.ts`       | `internal/feeds/feeds_test.go`         | pure data                                      |
| `tests/siteConfigs.test.ts` | `internal/scrape/configs_test.go`      | table-driven over `testdata/`                  |
| `tests/templates.test.ts`   | `internal/templates/templates_test.go` | `Render` into `bytes.Buffer`                   |
| `tests/routes.test.ts`      | `internal/app/app_test.go`             | `httptest` + fake parser + stub `RoundTripper` |
| `tests/fixtures.ts`         | `testdata/` + `fixtures_test.go`       | hostile payload corpus, verbatim               |

Two fidelity upgrades (choices, not accidents):

1. **Serve real RSS XML** through the stub transport and let real `gofeed` parse `testdata/fixture.rss`. The TS suite mocks the parser, so `rss-parser` is never exercised; don't reproduce that blind spot.
2. **Keep `stripOnlyTags` goquery-based** — the TS helper uses cheerio so the test mirrors production parsing exactly. Same rationale, same implementation.

Add: `-race` in CI, and a pairing test asserting every `SiteConfig` key has a `testdata/` fixture and vice versa.

### Phase 8 — Tooling & CI (½ day)

`.github/workflows/ci.yml` replacing `linting.yml`:

```yaml
on: [push, pull_request, workflow_dispatch]
permissions: {contents: read}
jobs:
  build:
    strategy: {matrix: {go: [1.27.x]}}
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with: {go-version: '${{ matrix.go }}', cache: true}
      - run: go tool templ generate
      - run: test -z "$(git status --porcelain)" || (git status --porcelain && exit 1) # staleness
      - run: test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
      - run: go tool templ fmt -fail .
      - run: go vet ./...
      - run: go test ./... -race -coverprofile=cover.out
      - uses: golangci/golangci-lint-action@v9
      - uses: golang/govulncheck-action@v1
```

- `golangci-lint`: defaults + `errcheck`, `revive`, `gocritic`, `bodyclose`, `noctx`. Skip `gocyclo`/`funlen` — they'd fight the handlers.
- `govulncheck` is the direct answer to the minimatch conversation: call-graph reachability instead of "this version is on a list". Expect zero noise.
- Dependabot: `gomod` + `github-actions`, weekly, grouped minor/patch.

### Phase 9 — Docs (½ day)

1. **README** — rewrite. It's currently stale three ways: claims "Handlebars templates" (never true), badges Pico CSS 2.1.1 while templates load Bulma 1.0.4, says port 3000 while `.env.example` says 4000.
2. **AGENTS.md** — full rewrite. Encode the invariants that matter:
   - never `templ.Raw`; scraped text stays `string`,
   - new scrape targets require a `testdata/` fixture with hostile payloads,
   - `go test ./... -race` before every commit,
   - run `go tool templ generate` after editing `.templ`, commit the output,
   - explicit error returns; `errors.Is` for sentinels; no `panic` outside init.
3. `.env.example` — `PORT=3000` only.
4. `.devcontainer/devcontainer.json` — `mcr.microsoft.com/devcontainers/go:1.27`, `golang.go` extension, `postCreateCommand: go mod download`, `forwardPorts: [3000]`.

### Phase 10 — Cutover (½ day)

1. Side-by-side diff against the phase-0 baseline for every §7 case.
2. Push `go-rewrite`; let Railway build it on a preview environment or a temporary service first if you want a dry run.
3. On `main`: merge, confirm Railpack detects Go (build log should show `go build`), confirm `/health` returns 200.
4. Set the Railway healthcheck path (§12.3) — this is the only service-config change.
5. After a green deploy: separate commit deleting `src/`, `tests/`, `package.json`, `pnpm-lock.yaml`, `tsconfig.json`, `eslint.config.mjs`, `vitest.config.ts`.
6. Re-target #30 and #19 at the Go codebase.

---

## 7. Behavior parity matrix

| Request                                | Status  | Body / headers                                            |
| -------------------------------------- | ------- | --------------------------------------------------------- |
| `GET /`                                | 200     | lists all 8 feeds, links `/feed?url=<encoded>`            |
| `GET /feed`                            | 200     | defaults to `feeds.All[0]` (`/Film`)                      |
| `GET /feed?url=<valid>`                | 200     | items with title, `pubDate`, `/article?feed=…&url=…` link (`url.Values.Encode()` order) |
| `GET /feed?url=<unreachable>`          | 502     | `error.templ`, "Bad Feed"                                 |
| `GET /feed?url=` (empty)               | 200     | falls back to `feeds.All[0]` — see note below             |
| `GET /article`                         | 400     | `No URL found`, no outbound request                       |
| `GET /article?url=ftp://…`             | 400     | `Invalid URL protocol`                                    |
| `GET /article?url=http://`             | 400     | `Invalid URL`                                             |
| `GET /article?url=<configured site>`   | 200     | title + paragraphs, markup stripped                       |
| `GET /article?url=<unconfigured site>` | 302     | `Location` = source URL                                   |
| `GET /article?url=<slow host>`         | **504** | `Request Timeout` (changed from 200, per 2.5)             |
| `GET /article?url=<fetch error>`       | 302     | `Location` = source URL                                   |
| `GET /favicon.ico`                     | 200     | `image/x-icon`                                            |
| `GET /health`                          | 200     | `ok` (new, per §12.3)                                     |
| Any panic                              | 500     | `Internal Server Error` (currently: connection closed)    |
| Any response                           | —       | no unescaped third-party markup, ever                     |

---

## 8. Gotchas (verified, not theoretical)

### 8.1 Escaping: templ ≠ `html/template`, and the difference matters

With **`html/template`**, interpolating a pre-encoded value after `?` double-encodes (verified: `url=https%253A%252F…`), because the engine applies `urlquery` escaping in query position.

With **templ**, the same pattern renders correctly single-encoded (verified: `url=https%3A%2F%2Fexample.com%2Fa%3Fb%3D1%26c%3D2`). templ applies `templ.URL` scheme sanitization + `templ.EscapeString` attribute escaping, not contextual query escaping.

Consequences:

- The TS pattern `href="/article?url=${encodeURIComponent(link)}"` ports **directly** to `href={ "/article?url=" + url.QueryEscape(link) }`.
- `&` in URLs still renders as `&amp;` — correct HTML, but test assertions must expect `&amp;` or compare decoded values.
- Unsafe schemes are neutralized to `about:invalid#TemplFailedSanitizationURL`.
- If you ever swap templ for `html/template`, revisit this section — the trap is real, just not yours right now.

### 8.2 Feed sort order changes if you use `sort.Strings`

`localeCompare` is ICU collation; Go's `<` is byte order. They disagree on your data:

```
JS localeCompare : /Film, AP News, ClickOnDetroit, CNBC, Hacker News, NBC News, PC Gamer, TMZ
Go sort.Strings  : /Film, AP News, CNBC, ClickOnDetroit, Hacker News, NBC News, PC Gamer, TMZ
```

(`ClickOnDetroit` vs `CNBC` — ICU gives letters a case-insensitive primary weight.)

Resolved by 2.6: hardcode the display order, delete the runtime sort, assert the order in a test. `feeds.All[0]` is `/Film` in both orders, so the `/feed` default is unaffected.

### 8.3 `truncate` slices UTF-16 code units; Go must slice runes

```ts
text.slice(0, 30) // UTF-16 code units — can split a surrogate pair
```

`string(s)[:30]` slices **bytes** and emits invalid UTF-8 for any non-ASCII headline. Use `[]rune(s)[:n]`. Identical output for ASCII; correct output for everything else.

### 8.4 Timeout detection: `Client.Timeout` errors are not `context.DeadlineExceeded`

TS checks `error.name === 'AbortError'`. Go equivalents differ by mechanism:

- `http.Client{Timeout: …}` → `*url.Error` with `Timeout() == true`:

  ```go
  var ne net.Error
  if errors.As(err, &ne) && ne.Timeout() { /* 504 */ }
  ```

- `context.WithTimeout` + `NewRequestWithContext` → `errors.Is(err, context.DeadlineExceeded)`.

Pick `Client.Timeout` on an injected client and write the test with a `RoundTripper` returning a `net.Error` whose `Timeout()` is true. Never assert on error strings.

### 8.5 Test substitutions

- `vi.stubGlobal('fetch', mock)` → inject `*http.Client` with a `roundTripFunc` transport returning fixtures keyed by `req.URL.Hostname()`. Same shape, no global mutation.
- `vi.mock('rss-parser')` → fake `rss.Parser`, or (preferred, phase 7) serve real XML through the stub transport.
- `app.request('/path')` → `httptest.NewRequest` + `httptest.NewRecorder` + `app.ServeHTTP`.
- `expect(x).toEqual(y)` → `cmp.Diff` for structs/slices, `==` for scalars, `strings.Contains` for HTML.
- **`url.Values.Encode()` sorts keys**, so links emit `feed=…&url=…` where TS emitted `url=…&feed=…`. Assert decoded query values, not raw href strings.

### 8.6 Small things that would otherwise get lost

- **Favicon.** `serveStatic({ path: './favicon.ico' })` is CWD-relative — works only when launched from the repo root. `//go:embed` removes the failure mode.
- **Missing `pubDate`.** TS renders the literal string `undefined`; Go renders `""`. Improvement, but check the no-date fixture.
- **`gofeed` parses `<description>` into HTML.** TS's `ParserFeed.content` is never rendered — don't port it. If descriptions are ever displayed, they must go through the text-only path.
- **templ `-fail` flag.** `templ fmt -fail .` exits non-zero on unformatted files; use it in CI so formatting is a gate, not a rewrite.

---

## 9. Effort

| Phase                         | Estimate    |
| ----------------------------- | ----------- |
| 0 Baseline                    | 0.5 d       |
| 1 Scaffold (incl. Go upgrade) | 0.5 d       |
| 2 Static data                 | 0.5 d       |
| 3 RSS layer                   | 0.5 d       |
| 4 Scrape layer                | 1 d         |
| 5 templ components            | 1.5 d       |
| 6 Handlers/routing/errors     | 1 d         |
| 7 Test port                   | 1 d         |
| 8 Tooling & CI                | 0.5 d       |
| 9 Docs                        | 0.5 d       |
| 10 Cutover                    | 0.5 d       |
| **Total**                     | **~8 days** |

Phase 5 carries the templ learning curve; phase 7 is where mocks get replaced by real parsing. The 439 lines of `src/` become ~600–800 lines of Go plus ~200 lines of generated code.

---

## 10. Risks

| Risk                                                       | Likelihood | Mitigation                                                                     |
| ---------------------------------------------------------- | ---------- | ------------------------------------------------------------------------------ |
| Stale generated `*_templ.go` committed                     | medium     | CI staleness gate (§5); regenerate in phase 5 review                           |
| Go version floor surprises (templ needs ≥1.26)             | medium     | pin `go 1.27.0`, `GOTOOLCHAIN=local` locally, `1.27.x` in CI                   |
| Railpack Go version resolution                             | low        | it reads `go.mod`; fallback `RAILPACK_GO_VERSION=1.27`                         |
| Template output drifts (escaping, `&amp;`, whitespace)     | medium     | phase-0 baseline + §8.1/8.3 + ported hostile-payload assertions                |
| goquery `.Text()` differs from cheerio on messy real pages | low        | fixtures are the corpus; real-site breakage shows up in the parity diff        |
| Port stalls with two implementations live                  | medium     | branch strategy (2.3) + phase exit criteria; TS tree stays green until cutover |
| Scope creep into #30 caching                               | medium     | explicitly out of scope (§1)                                                   |
| Zero-downtime deploy not verified                          | low        | `/health` + healthcheck path (§12.3) before the merge                          |

---

## 11. Resolved questions

All five from v1 are answered and folded in: templ (2.1), port 3000 (2.4), 504 on timeout (2.5), Railway deploy (§12), hardcoded feeds (2.10).

Remaining assumption to confirm during phase 0: whether anything external links to `news.tomslanda.com` paths that would change — the route set is unchanged, so only `/health` is additive.

---

## 12. Railway deployment

Current live state (read from the Railway API, 2026-11-01):

| Setting           | Value                                                                                                                |
| ----------------- | -------------------------------------------------------------------------------------------------------------------- |
| Project / service | `news` / `news` (`52550c78-bf9f-47b1-b68a-517af90f8d01`)                                                             |
| Source            | `slandath/news`, branch `main`, check suites on                                                                      |
| Builder           | **RAILPACK**, build env V3                                                                                           |
| Start command     | none set (builder default)                                                                                           |
| Healthcheck path  | **none set**                                                                                                         |
| Port              | `PORT` service variable present; both `news-production-5751.up.railway.app` and `news.tomslanda.com` target **3000** |
| Replicas / region | 1 / `us-east4-eqdc4a`                                                                                                |
| Sleep             | `sleepApplication: true`                                                                                             |

### 12.1 Why no Dockerfile

Railpack has a first-class Go provider: detects `go.mod` in the root, builds a **static binary with `CGO_ENABLED=0` and `-ldflags="-w -s"`, names it `out`**, and caches `~/.cache/go-build`. It picks the Go version from `go.mod` (falling back to `RAILPACK_GO_VERSION`, default 1.23 — which is why the `go 1.27.0` directive matters; without it you'd silently build with 1.23 and fail on templ).

It builds the root package when Go files are present there, so `main.go` at the root is the happy path. Because generated `*_templ.go` files are committed (§5), Railpack needs no codegen step.

Add a Dockerfile only if you later want distroless/pinned-base reproducibility; committing one would switch the service to the DOCKERFILE builder automatically.

### 12.2 Why feeds stay hardcoded

Three reasons: (a) parity is the whole point of the port — a config loader is a new failure mode during cutover; (b) issue #19 will restructure the feed list anyway (adding five sports feeds with categories), so the shape should be decided there, not twice; (c) a `[]Feed` literal in Go is already easy to edit and diff. When #19 lands, moving to an embedded `feeds.json` is ~20 lines plus a `//go:embed` — no architectural cost to deferring.

### 12.3 Post-cutover service config (one change)

Set the healthcheck path to `/health` (dashboard → service → Settings → Healthcheck Path, or `update-service` with `healthcheckPath`). Railway checks it against the injected `PORT`; a failing check marks the deployment failed and the previous one keeps serving, which is what makes deploys zero-downtime. The check times out after 300s by default.

`/health` must not touch the network or any dependency — it answers "is this process able to serve", nothing more.

Also worth noting: with `sleepApplication: true`, cold starts matter. This is where the pivot pays off visibly — a Go binary starts in single-digit milliseconds versus Node's module-graph boot plus the `dist/` tree.

### 12.4 Deploy checklist

1. Railpack build log shows Go detection and `go build` (not `npm`).
2. `/health` returns 200 on the public domain.
3. Healthcheck path set and a redeploy succeeds with no traffic drop.
4. Logs show `slog` JSON with method/path/status/duration.
5. Each §7 route spot-checked against the baseline.
