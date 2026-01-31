import type { Feed } from '../types.js'

export function homeTemplate({ feeds }: { feeds: Feed[] }) {
  return `
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
    <main class="container">
      <header>
        <nav aria-label="breadcrumb">
          <ul>
            <li></li>
          </ul>
        </nav>
      </header>
      <h1 class="h1">News Feeds</h1>
      <ul>
      ${feeds.map(feed => `
      <li>
        <a href="/feed?url=${feed.encodedURL}">
        ${feed.title}
        </a>
      </li>
        `).join('')}
      </ul>
    </main>
  </body>
</html>
`
}
