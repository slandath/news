# AI Agent Guidelines

This file provides instructions for AI coding assistants working on the RSS Feed Reader project.

## Build, Lint, and Test Commands

### Development

- `pnpm dev` - Start the development server with hot reload (watches `src/index.ts`)
- `pnpm build` - Compile TypeScript and copy template files to `dist/`
- `pnpm start` - Run the compiled production build from `dist/index.js`
- `pnpm lint` - Run ESLint on all files
- `pnpm lint:fix` - Auto-fix ESLint issues

### Testing

- Currently no test suite is configured
- To add tests in the future, update the `"test"` script in `package.json` (currently just exits with error)

### Type Checking

- Run `npx tsc --noEmit` to check for TypeScript errors without compiling
- This is useful for quick validation during development

## Code Style Guidelines

### Imports

- Use ES module syntax: `import`, `import type`, `export`
- Separate type imports from value imports using `import type`
- **Example:**
  ```typescript
  import type { CheerioAPI } from 'cheerio'
  import { Hono } from 'hono'
  ```
- Import extensions: Always include `.js` extensions in import paths for ES modules
  ```typescript
  import { feeds } from './feeds.js'
  import { siteConfigs } from './siteConfigs.js'
  ```

### Formatting & Style

- Uses `@antfu/eslint-config` for strict linting
- Prettier is configured for automatic formatting of CSS, HTML, and Markdown
- ESLint enforces consistent code style across TypeScript files
- **Run `pnpm lint:fix`** to auto-format before committing

### Type System

- All functions must have explicit return types
- Use strict TypeScript settings (enabled in `tsconfig.json`):
  - `strict: true` - Enables all strict type checking
  - `strictNullChecks: true` - No implicit null/undefined
  - `noImplicitAny: true` - No implicit `any` types
  - `noUncheckedIndexedAccess: true` - Type-safe array/object access
- Define interfaces for component props and data structures
- **Type imports** must use `import type` syntax to avoid circular dependencies

### Naming Conventions

- **Functions**: camelCase (e.g., `getDomain`, `parseURL`)
- **Constants**: camelCase (e.g., `parser`, `app`, `feeds`)
- **Types/Interfaces**: PascalCase (e.g., `SiteConfig`, `ArticleTemplateProps`)
- **Template functions**: camelCase with "Template" suffix (e.g., `homeTemplate`, `feedTemplate`)
- **URL/domain handling**: Use descriptive names like `feedURL`, `parsedUrl`, `domain`

### Error Handling

- Use try-catch blocks for async operations (RSS parsing, fetch requests)
- Always log errors to `console.error()` before returning error responses
- Provide graceful fallbacks:
  - For missing RSS feed URLs: return a 400 error with descriptive message
  - For invalid URLs: validate protocol (`http:` or `https:`)
  - For network timeouts: use `AbortController` with 5-second timeout, catch `AbortError`
  - For missing site configs: redirect to original source URL instead of crashing
- Return appropriate HTTP status codes (400 for client errors, 500 for server errors)
- **Example:**
  ```typescript
  try {
    // operation
  }
  catch (error) {
    console.error(error)
    if (error instanceof Error && error.name === 'AbortError') {
      return c.html('<p>Request Timeout</p>')
    }
    return c.redirect(url)
  }
  ```

### Pattern: Template Functions

- All template files export a function that takes a props object and returns HTML string
- Props interfaces are defined in `src/types.ts`
- Templates use curly braces for variable interpolation
- For arrays of content (like article paragraphs), map over the array and use Hono's `html` tag
- **Example:**

  ```typescript
  // types.ts
  export interface ArticleTemplateProps {
    title: string
    article: string[]
    url: string
  }

  // templates/article.ts
  export function articleTemplate({
    title,
    article,
    url,
  }: ArticleTemplateProps): string {
    return html`
      <h1>${title}</h1>
      ${article.map((para: string) => html`<p>${para}</p>`)}
      <a href="${url}">Source</a>
    `
  }
  ```

### Cheerio DOM Parsing

- Use Cheerio (`$`) for DOM manipulation and selection
- Type Cheerio as `CheerioAPI` from the `cheerio` package
- Site-specific CSS selectors are stored in `siteConfigs` with matching `title` and `article` selectors
- **SECURITY:** Use `.text()` to extract plain text content only, stripping all HTML tags (including malicious scripts)
- Return an array of text strings that templates can safely wrap in HTML
- **Example:**
  ```typescript
  function articleWrapper($: CheerioAPI) {
    return $('p.body-text')
      .map((_, el) => $(el).text())
      .get()
  }
  ```

### Security (XSS Prevention)

- **Never use `raw()`** from Hono's html module - it bypasses all escaping
- Always use Hono's `html` tagged template literal which automatically escapes variables
- Site configs extract text-only content using Cheerio's `.text()` method
- Templates safely construct HTML by mapping over text arrays and wrapping in elements
- **Example:**

  ```typescript
  // siteConfigs.ts - extract text only
  articleWrapper: ($: CheerioAPI) =>
    $('p.article-text')
      .map((_, el) => $(el).text())
      .get()

  // article.ts - safely construct HTML
  article.map((para: string) => html`<p>${para}</p>`)
  ```

### Hono Routes

- All route handlers take a `Context` parameter (`c: Context`)
- Query parameters are accessed via `c.req.query('paramName')`
- Return responses using:
  - `c.html(htmlString)` for HTML responses
  - `c.text(message, statusCode?)` for plain text responses
  - `c.redirect(url)` for redirects
- Always validate query parameters before using them

## Key Files and Their Purposes

- `src/index.ts` - Main application file with Hono routes and request handlers
- `src/types.ts` - TypeScript interfaces for props and data structures
- `src/feeds.ts` - Hardcoded list of RSS feed URLs (sorted alphabetically)
- `src/siteConfigs.ts` - DOM selectors for extracting article content from different news sites
- `src/templates/` - HTML template functions for rendering pages
- `tsconfig.json` - TypeScript compiler configuration with strict settings
- `eslint.config.mjs` - ESLint configuration using `@antfu/eslint-config`
- `.devcontainer/` - Development container configuration for isolated dev environment

## Project Overview

**RSS Feed Reader** is a lightweight web server (built with Hono) that:

1. Fetches RSS feeds from configured URLs
2. Parses RSS XML and displays feed items
3. Extracts article content from linked URLs using site-specific CSS selectors
4. Sanitizes all third-party content to prevent XSS vulnerabilities
5. Renders content using server-side templates with automatic HTML escaping

The application runs on port 4000 and provides:

- `GET /` - Home page with available feeds
- `GET /feed?url=<RSS_URL>` - Display feed items
- `GET /article?url=<ARTICLE_URL>` - Display full article content

## Node & Package Manager

- **Node version**: 24.x (specified in `package.json`)
- **Package manager**: pnpm 10.26.0 (required)
- **Module system**: ES modules (`"type": "module"` in `package.json`)

## Before Committing

1. Run `pnpm lint:fix` to ensure code style compliance
2. Run `pnpm build` to verify TypeScript compilation succeeds
3. Check that no new type errors are introduced
4. If adding routes or types, update relevant files and interfaces
