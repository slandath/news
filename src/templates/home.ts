import type { Feed } from '../types.js'
import { html } from 'hono/html'

export function homeTemplate({ feeds }: { feeds: Feed[] }) {
  return html`
<!doctype html>
<html lang="en">
   <head>
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title>Tom's News</title>
      <link rel="stylesheet" href="/styles.css" />
   </head>
   <body>
  <div class="section">
      <header>
        <nav class="breadcrumb" aria-label="breadcrumbs">
          <ul>
            <li class="is-active"><a href="/">Home</a></li>
          </ul>
        </nav>
      </header>
      </div>
      <section class="hero is-small is-primary">
         <div class="hero-body">
            <p class="title">News Feeds</p>
         </div>
      </section>
      <section class="section">
         <ul>
            ${feeds.map(feed => html`
            <li class="py-1">
               <a href="/feed?url=${feed.encodedURL}">
               ${feed.title}
               </a>
            </li>
            `)}
         </ul>
      </section>
   </body>
</html>
`
}
