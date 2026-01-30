// Import types and libraries
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
import { homeTemplate } from './templates/home.js'

// Initialize Hono app and RSS parser
const app = new Hono()
const parser = new Parser()

// Get __dirname equivalent in ES modules
const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)
const templateDir = join(__dirname, 'templates')

// Helper to extract domain from URL (e.g., "apnews.com" from "www.apnews.com")
const getDomain = (url: URL): string => url.hostname.replace('www.', '')

const feedTemplate = Handlebars.compile(
  fs.readFileSync(join(templateDir, 'feeds.html'), 'utf-8'),
)

const articleTemplate = Handlebars.compile(
  fs.readFileSync(join(templateDir, 'article.html'), 'utf-8'),
)

// Register Handlebars helper for URL encoding
Handlebars.registerHelper('encodeURI', (str) => {
  return encodeURIComponent(str)
})

// GET / - Display home page with list of available feeds
app.get('/', (c) => {
  const feedsWithEncoded = feeds.map(f => ({
    ...f,
    encodedURL: encodeURIComponent(f.url),
  }))
  const html = homeTemplate({ feeds: feedsWithEncoded })
  return c.html(html)
})

// GET /feed - Fetch and display RSS feed items
app.get('/feed', async (c) => {
  const feedURL = c.req.query('url') || feeds[0]?.url
  if (!feedURL) {
    return c.text('URL undefined')
  }
  try {
    const feed = await parser.parseURL(feedURL)
    const html = feedTemplate({ feed, feeds })
    return c.html(html)
  }
  catch (error) {
    console.error(error)
    return c.text('Error fetching feed')
  }
})

// GET /article - Fetch article content and extract body/title using Cheerio
app.get('/article', async (c) => {
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
    const response = await fetch(parsedUrl.toString(), { signal: controller.signal })
    const html = await response.text()
    const $ = cheerio.load(html)

    // Extract article content and title using site-specific selectors
    const article = config.articleWrapper($ as CheerioAPI)
    const title = $(config.title).text()
    const rendered = articleTemplate({ title, article, url: parsedUrl.toString() })
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

// Start HTTP server on default port
serve(app)
