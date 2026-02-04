export function articleTemplate({ title, article, url }) {
    return `
  <!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="color-scheme" content="light dark" />
    <title>${title}</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css" />
  </head>
  <body>
    <main class="container">
      <header>
        <nav aria-label="breadcrumb">
          <ul>
            <li><a href="/">Home</a></li>
            <li><a href="/feed">Feed</a></li>
          </ul>
        </nav>
      </header>
      <h1>${title}</h1>
      <section>
      ${article}
        <a href="${url}" target="_blank">Source</a>
      </section>
    </main>
  </body>
</html>
  `;
}
//# sourceMappingURL=article.js.map