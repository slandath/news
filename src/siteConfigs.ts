import type { CheerioAPI } from 'cheerio'

export interface SiteConfig {
  article: string
  articleWrapper: ($: CheerioAPI) => string
  title: string
}

export const siteConfigs: Record<string, SiteConfig> = {
  'clickondetroit.com': {
    article: 'p.article-text',
    articleWrapper: ($: CheerioAPI) => $('p.article-text')
      .map((i: number, el: any) => `<p>${$(el).text()}</p>`)
      .get()
      .join(''),
    title: 'h1.headline',
  },
  'cnbc.com': {
    article: 'div.group p',
    articleWrapper: ($: CheerioAPI) => $('div.group p')
      .map((i: number, el: any) => `<p>${$(el).text()}</p>`)
      .get()
      .join(''),
    title: 'h1.ArticleHeader-headline',
  },
  'nbcnews.com': {
    article: 'p.body-graf',
    articleWrapper: ($: CheerioAPI) => $('p.body-graf')
      .map((i: number, el: any) => `<p>${$(el).text()}</p>`)
      .get()
      .join(''),
    title: 'h1.article-hero-headline__htag',
  },
  'pcgamer.com': {
    article: 'div#article-body p',
    articleWrapper: ($: CheerioAPI) => $('div#article-body p')
      .map((i: number, el: any) => `<p>${$(el).text()}</p>`)
      .get()
      .join(''),
    title: 'h1',
  },
  'slashfilm.com': {
    article: 'div.columns-holder p',
    articleWrapper: ($: CheerioAPI) => $('div.columns-holder p')
      .map((i: number, el: any) => `<p>${$(el).text()}</p>`)
      .get()
      .join(''),
    title: 'h1.title-gallery',
  },

}
