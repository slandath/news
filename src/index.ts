import { serve } from '@hono/node-server'
import { Hono } from 'hono'
import * as nunjucks from 'nunjucks'
import Parser from 'rss-parser'

nunjucks.configure('src/templates', { autoescape: true })

const app = new Hono()
const parser = new Parser()
const feeds = [
  { title: 'The Verge', url: 'https://www.theverge.com/rss/index.xml' },
  { title: 'CNBC', url: 'https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114' },
  { title: 'Hacker News', url: 'https://news.ycombinator.com/rss' },
  { title: 'NBC News', url: 'https://feeds.nbcnews.com/nbcnews/public/news' },
].sort((a, b) => a.title.localeCompare(b.title))

app.get('/', (c) => {
  const html = nunjucks.render('home.html', { feeds })
  return c.html(html)
})

app.get('/feed', async (c) => {
  const feedURL = c.req.query('url') || feeds[0].url
  try {
    const feed = await parser.parseURL(
      feedURL,
    )
    const html = nunjucks.render('feeds.html', { feed, feeds })
    return c.html(html)
  }
  catch (error) {
    console.error(error)
    return c.text('Error fetching feed')
  }
})

serve(app)
