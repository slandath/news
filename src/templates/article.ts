import type { HtmlEscapedString } from 'hono/utils/html'
import type { ArticleTemplateProps } from '../types.js'
import { html } from 'hono/html'

export function articleTemplate({
  title,
  article,
  url,
}: ArticleTemplateProps): HtmlEscapedString | Promise<HtmlEscapedString> {
  return html`
    <!doctype html>
    <html lang="en">
      <head>
        <meta charset="UTF-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
        <meta name="color-scheme" content="light dark" />
        <title>${title}</title>
        <link
          rel="stylesheet"
          href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css"
        />
      </head>
      <body>
        <main class="container" role="main">
          <header role="banner">
            <nav aria-label="breadcrumb" role="navigation">
              <ul>
                <li><a href="/">Home</a></li>
                <li><a href="/feed">Feed</a></li>
              </ul>
            </nav>
          </header>
          <h1>${title}</h1>
          <section>
            ${article.map((para: string) => html`<p>${para}</p>`)}
            <a href="${url}" target="_blank">Source</a>
          </section>
        </main>
      </body>
    </html>
  `
}
