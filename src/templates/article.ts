import type { HtmlEscapedString } from 'hono/utils/html'
import type { ArticleTemplateProps } from '../types.js'
import { html } from 'hono/html'

export function articleTemplate({
  title,
  article,
  url,
  feedTitle,
  feedEncodedURL,
}: ArticleTemplateProps): HtmlEscapedString | Promise<HtmlEscapedString> {
  return html`
    <!doctype html>
    <html lang="en">
      <head>
        <meta charset="UTF-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
        <title>${title}</title>
      <link rel="stylesheet" href="/styles.css" />
      </head>
      <body>
        <div class="section">
         <header>
            <nav class="breadcrumb" aria-label="breadcrumbs">
               <ul>
                  <li><a href="/">Home</a></li>
                  ${feedEncodedURL ? html`<li><a href="/feed?url=${feedEncodedURL}">${feedTitle}</a></li>` : ''}
                  <li class="is-active"><a href="#" aria-current="page">${title}</a></li>
               </ul>
            </nav>
         </header>
      </div>
      <main>
      <section class="hero is-small is-primary">
         <div class="hero-body">
            <p class="title">${title}</p>
         </div>
      </section>
          <section class="section">
            ${article.map((para: string) => html`<p class="py-1">${para}</p>`)}
            <div class="pt-4">
            <a href="${url}" class="is-size-5" target="_blank">Source</a>
            </div>
          </section>
        </main>
      </body>
    </html>
  `
}
