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
import { siteConfigs } from './siteConfigs.js'

const app = new Hono()
const parser = new Parser()
const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)
const templateDir = join(__dirname, 'templates')

const getDomain = (url: string) => new URL(url).hostname.replace('www.', '')

const homeTemplate = Handlebars.compile(
  fs.readFileSync(join(templateDir, 'home.html'), 'utf-8'),
)

const feedTemplate = Handlebars.compile(
  fs.readFileSync(join(templateDir, 'feeds.html'), 'utf-8'),
)

const articleTemplate = Handlebars.compile(
  fs.readFileSync(join(templateDir, 'article.html'), 'utf-8'),
)

Handlebars.registerHelper('encodeURI', (str) => {
  return encodeURIComponent(str)
})

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
    return c.text('No URL found', 400)
  }
  try {
    const domain = getDomain(url)
    const config = siteConfigs[domain]

    if (!config) {
      return c.redirect(url)
    }

    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 5000)
    const response = await fetch(url, { signal: controller.signal })
    clearTimeout(timeout)
    const html = await response.text()
    const $ = cheerio.load(html)

    const article = config.articleWrapper($ as CheerioAPI)
    const title = $(config.title).text()
    const rendered = articleTemplate({ title, article, url })
    return c.html(rendered)
  }
  catch (error) {
    console.error(error)

    if (error instanceof Error && error.name === 'AbortError') {
      return c.html('<p>Request Timeout</p>')
    }

    return c.redirect(url)
  }
})

serve(app)
