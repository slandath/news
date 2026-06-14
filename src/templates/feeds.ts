import type { HtmlEscapedString } from 'hono/utils/html'
import type { FeedTemplateProps } from '../types.js'
import { html } from 'hono/html'

export function feedTemplate({ feed, feedURL }: FeedTemplateProps): HtmlEscapedString | Promise<HtmlEscapedString> {
  return html`
<!doctype html>
<html lang="en">
   <head>
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title>${feed.title}</title>
      <link
         rel="stylesheet"
         href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css"
         >
   </head>
   <body>
      <div class="section">
         <header>
            <nav class="breadcrumb" aria-label="breadcrumbs">
               <ul>
                  <li><a href="/">Home</a></li>
                  <li class="is-active"><a href="#" class="truncate" aria-current="page">${feed.title}</a></li>
               </ul>
            </nav>
         </header>
      </div>
      <section class="hero is-small is-primary">
         <div class="hero-body">
            <p class="title">${feed.title}</p>
         </div>
      </section>
      <main class="section">
         <section id="content">
            <ul>
               ${feed.items.map(article => html`
               <li class="py-3">
                  ${article.link
                    ? html`
                  <a href="/article?url=${encodeURIComponent(article.link)}&feed=${encodeURIComponent(feedURL)}">
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
