import type { HtmlEscapedString } from 'hono/utils/html'
import type { ErrorTemplateProps } from '../types.js'
import { html } from 'hono/html'

export function errorTemplate({ title, message }: ErrorTemplateProps): HtmlEscapedString | Promise<HtmlEscapedString> {
  return html`
  <!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="color-scheme" content="light dark" />
    <title>Error</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css" />
  </head>
  <body>
    <main class="container" role="main">
      <header role="banner">
        <nav aria-label="breadcrumb" role="navigation">
          <ul>
            <li><a href="/">Home</a></li>
          </ul>
        </nav>
      </header>
      <section>
        <h1>${title}</h1>
        <p>${message}</p>
      </section>
    </main>
  </body>
</html>
  `
}
