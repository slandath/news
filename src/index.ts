import { serve } from '@hono/node-server'
import { Hono } from 'hono'
import * as nunjucks from 'nunjucks'
import Parser from 'rss-parser'

const app = new Hono()
const parser = new Parser()

nunjucks.configure('src/templates', { autoescape: true })

app.get('/', async (c) => {
  try {
    const feed = await parser.parseURL(
      'https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114',
    )
    const html = nunjucks.render('feeds.html', { feed })
    return c.html(html)
  }
  catch (error) {
    console.error(error)
    return c.text('Error fetching feed')
  }
})

serve(app)
