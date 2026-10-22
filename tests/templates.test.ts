import type { ArticleTemplateProps, ErrorTemplateProps, Feed, FeedTemplateProps, ParserFeed } from '../src/types.js'
import { describe, expect, it } from 'vitest'
import { articleTemplate } from '../src/templates/article.js'
import { errorTemplate } from '../src/templates/error.js'
import { feedTemplate } from '../src/templates/feeds.js'
import { homeTemplate } from '../src/templates/home.js'

const feedData: Feed[] = [
  { title: 'CNBC & Markets', url: 'https://example.com/cnbc/rss', encodedURL: 'https%3A%2F%2Fexample.com%2Fcnbc%2Frss' },
  { title: 'Hacker News <script>alert(1)</script>', url: 'https://news.ycombinator.com/rss', encodedURL: 'https%3A%2F%2Fnews.ycombinator.com%2Frss' },
]

const parserFeed: ParserFeed = {
  title: 'Fixture Feed',
  items: [
    { title: 'First <script>alert(1)</script> item', link: 'https://example.com/story-1', pubDate: 'Wed, 21 Oct 2026 10:00:00 GMT' },
    { title: 'Link-less item', pubDate: 'Wed, 21 Oct 2026 11:00:00 GMT' },
  ],
}

describe('articleTemplate', () => {
  const props: ArticleTemplateProps = {
    title: 'Headline <script>alert(1)</script>',
    article: [
      'Paragraph with <script>alert(1)</script> inline',
      'Plain paragraph',
    ],
    url: 'https://example.com/story',
    feedTitle: 'CNBC',
    feedEncodedURL: feedData[0]!.encodedURL,
  }

  it('renders a full document with the title and paragraphs', async () => {
    const html = (await articleTemplate(props)).toString()
    expect(html).toContain('<!doctype html>')
    expect(html).toContain('<p class="py-2">Paragraph with &lt;script&gt;alert(1)&lt;/script&gt; inline</p>')
    expect(html).toContain('Plain paragraph')
    expect(html).toContain('href="https://example.com/story"')
  })

  it('escapes the title instead of emitting raw script tags', async () => {
    const html = (await articleTemplate(props)).toString()
    expect(html).toContain('Headline &lt;script&gt;alert(1)&lt;/script&gt;')
    expect(html).not.toContain('<script>alert(1)</script>')
  })

  it('truncates long breadcrumb and hero titles', async () => {
    const longTitle = `${'VeryLongWord'.repeat(6)} <script>alert(1)</script>`
    const html = (await articleTemplate({ ...props, title: longTitle })).toString()
    // breadcrumb truncates at 30 chars, hero at 60
    expect(html).toContain(`${longTitle.slice(0, 30)}...</a>`)
    expect(html).toContain(`${longTitle.slice(0, 60)}...</p>`)
  })

  it('omits the feed breadcrumb when no feed is given', async () => {
    const html = (await articleTemplate({ ...props, feedTitle: undefined, feedEncodedURL: undefined })).toString()
    expect(html).not.toContain('/feed?url=')
  })
})

describe('feedTemplate', () => {
  const props: FeedTemplateProps = {
    feed: parserFeed,
    feeds: feedData,
    feedURL: 'https://example.com/cnbc/rss',
  }

  it('renders feed title and linked items', async () => {
    const html = (await feedTemplate(props)).toString()
    expect(html).toContain('Fixture Feed')
    const expected = encodeURIComponent('https://example.com/story-1')
    expect(html).toContain(`/article?url=${expected}`)
  })

  it('renders link-less items as plain text without an anchor', async () => {
    const html = (await feedTemplate(props)).toString()
    expect(html).toContain('Link-less item')
    const anchors = (html.match(/<a href="\/article\?url=/g) ?? []).length
    expect(anchors).toBe(1)
  })

  it('escapes hostile item titles', async () => {
    const html = (await feedTemplate(props)).toString()
    expect(html).toContain('First &lt;script&gt;alert(1)&lt;/script&gt; item')
    expect(html).not.toContain('<script>alert(1)</script>')
  })
})

describe('homeTemplate', () => {
  it('renders every feed with an encoded /feed link', async () => {
    const html = (await homeTemplate({ feeds: feedData })).toString()
    expect(html).toContain('News Feeds')
    for (const feed of feedData)
      expect(html).toContain(`/feed?url=${feed.encodedURL}`)
    expect(html).toContain('CNBC &amp; Markets')
  })

  it('escapes hostile feed titles', async () => {
    const html = (await homeTemplate({ feeds: feedData })).toString()
    expect(html).toContain('Hacker News &lt;script&gt;alert(1)&lt;/script&gt;')
    expect(html).not.toContain('<script>alert(1)</script>')
  })
})

describe('errorTemplate', () => {
  const props: ErrorTemplateProps = { title: 'Failure', message: 'Bad things <script>alert(1)</script> happened' }

  it('renders title and escaped message', async () => {
    const html = (await errorTemplate(props)).toString()
    expect(html).toContain('<h1>Failure</h1>')
    expect(html).toContain('Bad things &lt;script&gt;alert(1)&lt;/script&gt; happened')
    expect(html).not.toContain('<script>alert(1)</script>')
  })
})
