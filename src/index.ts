import type { CheerioAPI } from 'cheerio'
import fs from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { serve } from '@hono/node-server'
import * as cheerio from 'cheerio'
import Handlebars from 'handlebars'
import { Hono } from 'hono'
import Parser from 'rss-parser'
import { feeds } from './feeds.js'

interface SiteConfig {
  article: string
  articleWrapper: ($: CheerioAPI) => string
  title: string
}

const app = new Hono()
const parser = new Parser()
const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)
const templateDir = join(__dirname, 'templates')

const homeTemplate = Handlebars.compile(
  fs.readFileSync(join(templateDir, 'home.html'), 'utf-8'),
)

const feedTemplate = Handlebars.compile(
  fs.readFileSync(join(templateDir, 'feeds.html'), 'utf-8'),
)

const articleTemplate = Handlebars.compile(
  fs.readFileSync(join(templateDir, 'article.html'), 'utf-8'),
)

const siteConfigs: Record<string, SiteConfig> = {
  'nbcnews.com': {
    article: 'p.body-graf',
    articleWrapper: ($: CheerioAPI) => $('p.body-graf').map((i, el) => `<p>${$(el).html()}</p>`).get().join(''),
    title: 'h1.article-hero-headline__htag',
  },
  'cnbc.com': {
    article: 'div.group p',
    articleWrapper: ($: CheerioAPI) => $('div.group p')
      .map((i: number, el: any) => `<p>${$(el).html()}</p>`)
      .get()
      .join(''),
    title: 'h1.ArticleHeader-headline',
  },
  'clickondetroit.com': {
    article: 'p.article-text',
    articleWrapper: ($: CheerioAPI) => $('p.article-text')
      .map((i: number, el: any) => `<p>${$(el).html()}</p>`)
      .get()
      .join(''),
    title: 'h1.headline',
  },
}

app.get('/', (c) => {
  const feedsWithEncoded = feeds.map(f => ({
    ...f,
    encodedURL: encodeURIComponent(f.url),
  }))
  const html = homeTemplate({ feeds: feedsWithEncoded })
  return c.html(html)
})

app.get('/feed', async (c) => {
  const feedURL = c.req.query('url') || feeds[0].url
  try {
    const feed = await parser.parseURL(
      feedURL,
    )
    const feedSource = new URL(feedURL).hostname.replace('www.', '')

    feed.items.forEach((item) => {
      item.isHackerNews = feedSource === 'news.ycombinator.com'
    })
    const html = feedTemplate({ feed, feeds })
    return c.html(html)
  }
  catch (error) {
    console.error(error)
    return c.text('Error fetching feed')
  }
})

app.get('/article', async (c) => {
  const url = c.req.query('url')
  if (!url) {
    return c.text('URL required', 400)
  }
  try {
    const urlObj = new URL(url)
    const domain = urlObj.hostname.replace('www.', '')
    const config = siteConfigs[domain]

    if (!config) {
      return c.text('Site not configured', 400)
    }

    const response = await fetch(url)
    const html = await response.text()
    const $ = cheerio.load(html)

    const article = config.articleWrapper($ as CheerioAPI)
    const title = $(config.title).text()
    const rendered = articleTemplate({ title, article, url })
    return c.html(rendered)
  }
  catch (error) {
    console.error(error)
    return c.text('Error fetching article')
  }
})

serve(app)
