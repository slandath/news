// Import types and libraries
import type { CheerioAPI } from 'cheerio'
import type { Context } from 'hono'
import process from 'node:process'
import { serve } from '@hono/node-server'
import { serveStatic } from '@hono/node-server/serve-static'
import * as cheerio from 'cheerio'
import { Hono } from 'hono'
import { HTTPException } from 'hono/http-exception'
import Parser from 'rss-parser'
import { feeds } from './feeds.js'
import { siteConfigs } from './siteConfigs.js'
import { articleTemplate } from './templates/article.js'
import { errorTemplate } from './templates/error.js'
import { feedTemplate } from './templates/feeds.js'
import { homeTemplate } from './templates/home.js'

// Initialize Hono app and RSS parser
const app = new Hono()
const parser = new Parser()

// Port
const PORT = Number(process.env.PORT) || 4000

// Favicon
app.use('/favicon.ico', serveStatic({ path: './favicon.ico' }))

// Helper to extract domain from URL (e.g., "apnews.com" from "www.apnews.com")
const getDomain = (url: URL): string => url.hostname.replace('www.', '')

// GET / - Display home page with list of available feeds
app.get('/', (c: Context) => {
  const feedsWithEncoded = feeds.map(f => ({
    ...f,
    encodedURL: encodeURIComponent(f.url),
  }))
  const html = homeTemplate({ feeds: feedsWithEncoded })
  return c.html(html)
})

// GET /feed - Fetch and display RSS feed items
app.get('/feed', async (c: Context) => {
  const feedURL = c.req.query('url') || feeds[0]?.url
  if (!feedURL) {
    return c.text('URL undefined')
  }
  try {
    const feed = await parser.parseURL(feedURL)
    const feedsWithEncoded = feeds.map(f => ({
      ...f,
      encodedURL: encodeURIComponent(f.url),
    }))
    const html = feedTemplate({ feed, feeds: feedsWithEncoded })
    return c.html(html)
  }
  catch (error) {
    console.error(error)
    throw new HTTPException(502, { message: 'Bad Feed' })
  }
})

// GET /article - Fetch article content and extract body/title using Cheerio
app.get('/article', async (c: Context) => {
  const urlParam = c.req.query('url')
  if (!urlParam) {
    return c.text('No URL found', 400)
  }
  let parsedUrl: URL
  try {
    parsedUrl = new URL(urlParam)
    if (!['http:', 'https:'].includes(parsedUrl.protocol)) {
      return c.text('Invalid URL protocol', 400)
    }
  }
  catch (error) {
    console.error(error)
    return c.text('Invalid URL', 400)
  }

  let timeout: NodeJS.Timeout | undefined
  try {
    const domain = getDomain(parsedUrl)
    const config = siteConfigs[domain]

    // If no config exists for this domain, redirect to source URL
    if (!config) {
      return c.redirect(parsedUrl.toString())
    }

    // Fetch article with 5-second timeout
    const controller = new AbortController()
    timeout = setTimeout(() => controller.abort(), 5000)
    const response = await fetch(parsedUrl.toString(), {
      signal: controller.signal,
    })
    const html = await response.text()
    const $ = cheerio.load(html)

    // Extract article content and title using site-specific selectors
    const article = config.articleWrapper($ as CheerioAPI)
    const title = $(config.title).text()
    const rendered = articleTemplate({
      title,
      article,
      url: parsedUrl.toString(),
    })
    return c.html(rendered)
  }
  catch (error) {
    console.error(error)

    // Return timeout error message if fetch took too long
    if (error instanceof Error && error.name === 'AbortError') {
      return c.html('<p>Request Timeout</p>')
    }

    // Fallback: redirect to source URL on any other error
    return c.redirect(parsedUrl.toString())
  }
  finally {
    clearTimeout(timeout)
  }
})

app.onError((error, c) => {
  if (error instanceof HTTPException) {
    console.error(error)
    return c.html(
      errorTemplate({ title: 'Error', message: error.message }),
      error.status,
    )
  }
  console.error(error)
  return c.text('Internal Server Error', 500)
})

// Start HTTP server on default port
serve({ fetch: app.fetch, port: PORT, hostname: '0.0.0.0' }, (info) => {
  console.warn(`Server listening on http://${info.address}:${info.port}`)
})
