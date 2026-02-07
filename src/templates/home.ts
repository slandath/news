import type { Feed } from '../types.js'
import { html } from 'hono/html'

export function homeTemplate({ feeds }: { feeds: Feed[] }) {
  return html`
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="color-scheme" content="light dark" />
    <title>Tom's News</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css" />
  </head>
  <body>
    <header role="banner">
      <nav aria-label="breadcrumb" role="navigation">
        <ul>
          <li></li>
        </ul>
      </nav>
    </header>
    <main class="container" role="main">
      <h1 class="h1">News Feeds</h1>
      <ul>
      ${feeds.map(feed => html`
      <li>
        <a href="/feed?url=${feed.encodedURL}">
        ${feed.title}
        </a>
      </li>
        `)}
      </ul>
    </main>
  </body>
</html>
`
}
