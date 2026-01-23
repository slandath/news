import fs from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { serve } from '@hono/node-server'
import Handlebars from 'handlebars'
import { Hono } from 'hono'
import Parser from 'rss-parser'
import { feeds } from './feeds.js'

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

serve(app)
