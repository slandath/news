import type { FeedTemplateProps } from '../types.js'
import { html } from 'hono/html'

export function feedTemplate({ feed }: FeedTemplateProps) {
  return html`
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="color-scheme" content="light dark" />
    <title>${feed.title}</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css" />
  </head>
  <body>
    <main class="container">
      <header>
        <nav aria-label="breadcrumb">
          <ul>
            <li><a href="/">Home</a></li>
          </ul>
        </nav>
      </header>
      <h1>${feed.title}</h1>
      <section id="content">
            <ul>
      ${feed.items.map(article => html`
      <li>
      ${article.link
        ? html`
        <a href="/article?url=${encodeURIComponent(article.link)}">
        ${article.title}
        </a>
        `
        : article.title}
        <p>${article.pubDate}</p>
              </li>
        `)}
      </ul>
      </section>
    </main>
  </body>
</html>
`
}
