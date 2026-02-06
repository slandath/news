import type { CheerioAPI } from 'cheerio'
import type { SiteConfig } from './types.js'

/*
SECURITY: Use .text() to extract plain text content only.
This strips all HTML tags (including malicious scripts) from scraped articles,
preventing XSS attacks when content is displayed in templates.
The template then safely reconstructs HTML using Hono's html`` tag,
which automatically escapes any remaining special characters.
*/

export const siteConfigs: Record<string, SiteConfig> = {
  'clickondetroit.com': {
    article: 'p.article-text',
    articleWrapper: ($: CheerioAPI) => $('p.article-text')
      .map((_, el) => $(el).text())
      .get(),
    title: 'h1.headline',
  },
  'cnbc.com': {
    article: 'div.group p',
    articleWrapper: ($: CheerioAPI) => $('div.group p')
      .map((_, el) => $(el).text())
      .get(),
    title: 'h1.ArticleHeader-headline',
  },
  'nbcnews.com': {
    article: 'p.body-graf',
    articleWrapper: ($: CheerioAPI) => $('p.body-graf')
      .map((_, el) => $(el).text())
      .get(),
    title: 'h1.article-hero-headline__htag',
  },
  'pcgamer.com': {
    article: 'div#article-body p',
    articleWrapper: ($: CheerioAPI) => $('div#article-body p')
      .map((_, el) => $(el).text())
      .get(),
    title: 'h1',
  },
  'slashfilm.com': {
    article: 'div.columns-holder p',
    articleWrapper: ($: CheerioAPI) => $('div.columns-holder p')
      .map((_, el) => $(el).text())
      .get(),
    title: 'h1.title-gallery',
  },

}
