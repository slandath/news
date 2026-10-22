import type { ArticleFixture } from './fixtures.js'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { app } from '../src/app.js'
import { feeds } from '../src/feeds.js'
import { articleFixturesByDomain, parseFeedFixture, stripOnlyTags } from './fixtures.js'

const mocks = vi.hoisted(() => ({
  parseURL: vi.fn<() => Promise<unknown>>(),
}))

vi.mock('rss-parser', () => {
  return {
    default: class {
      parseURL = mocks.parseURL
    },
  }
})

/** Shared fetch mock; default implementation serves article fixtures by domain. */
const fetchMock = vi.fn(async (input: RequestInfo | URL): Promise<Response> => {
  const href = typeof input === 'string' ? input : String(input)
  const fixture = Object.entries(articleFixturesByDomain)
    .find(([domain]) => href.includes(domain))?.[1] as ArticleFixture | undefined
  if (!fixture)
    throw new Error(`No article fixture for ${href}`)
  return new Response(fixture.html, { headers: { 'content-type': 'text/html' } })
})

function articleURL(url: string, feed?: string): string {
  const params = new URLSearchParams({ url })
  if (feed)
    params.set('feed', feed)
  return `/article?${params.toString()}`
}

beforeEach(() => {
  mocks.parseURL.mockReset()
  vi.spyOn(console, 'error').mockImplementation(() => {})
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('gET /', () => {
  it('returns the home page listing every feed', async () => {
    const res = await app.request('/')
    expect(res.status).toBe(200)
    const text = await res.text()
    expect(text).toContain('News Feeds')
    for (const feed of feeds)
      expect(text).toContain(`<a href="/feed?url=${encodeURIComponent(feed.url)}">`)
  })
})

describe('gET /feed', () => {
  it('fetches the requested feed and lists its items', async () => {
    mocks.parseURL.mockResolvedValue(parseFeedFixture())
    const res = await app.request(`/feed?url=${encodeURIComponent(feeds[0]!.url)}`)
    expect(res.status).toBe(200)
    expect(mocks.parseURL).toHaveBeenCalledWith(feeds[0]!.url)
    const text = await res.text()
    expect(text).toContain('Item one')
    expect(text).toContain('Item two')
    expect(text).toContain('/article?url=')
  })

  it('falls back to the first configured feed when no url query is given', async () => {
    mocks.parseURL.mockResolvedValue(parseFeedFixture())
    const res = await app.request('/feed')
    expect(res.status).toBe(200)
    expect(mocks.parseURL).toHaveBeenCalledWith(feeds[0]!.url)
  })

  it('returns 502 with the error template when the parser throws', async () => {
    mocks.parseURL.mockRejectedValue(new Error('feed unavailable'))
    const res = await app.request(`/feed?url=${encodeURIComponent(feeds[0]!.url)}`)
    expect(res.status).toBe(502)
    const text = await res.text()
    expect(text).toContain('Bad Feed')
  })
})

describe('gET /article', () => {
  describe.each(Object.entries(articleFixturesByDomain))('%s', (domain, fixture) => {
    it('renders title and article paragraphs with markup stripped', async () => {
      const fx = fixture as ArticleFixture
      const res = await app.request(articleURL(`https://www.${domain}/story`))
      expect(res.status).toBe(200)
      expect(fetchMock).toHaveBeenCalledWith(`https://www.${domain}/story`, expect.anything())

      const text = await res.text()
      expect(text).toContain(stripOnlyTags(fx.title))
      for (const para of fx.paragraphs) {
        expect(text).toContain(stripOnlyTags(para))
      }
      expect(text).not.toContain('<script>')
      expect(text).not.toContain('onerror=')
      expect(text).not.toContain('<img')
    })
  })

  it('links back to the originating feed when one is supplied', async () => {
    const fmFeed = feeds.find(f => f.title === 'CNBC')!.url
    const res = await app.request(articleURL('https://www.cnbc.com/story', fmFeed))
    expect(res.status).toBe(200)
    const text = await res.text()
    expect(text).toContain(`/feed?url=${encodeURIComponent(fmFeed)}`)
    expect(text).toContain('CNBC')
  })

  it('returns 400 when url is missing', async () => {
    const res = await app.request('/article')
    expect(res.status).toBe(400)
    expect(await res.text()).toBe('No URL found')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('returns 400 for non-http protocols', async () => {
    const res = await app.request(articleURL('ftp://example.com/story'))
    expect(res.status).toBe(400)
    expect(await res.text()).toBe('Invalid URL protocol')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('returns 400 for a malformed URL', async () => {
    const res = await app.request(articleURL('http://'))
    expect(res.status).toBe(400)
    expect(await res.text()).toBe('Invalid URL')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('redirects to the source for domains without a site config', async () => {
    const res = await app.request(articleURL('https://www.tmz.com/story'))
    expect(res.status).toBe(302)
    expect(res.headers.get('location')).toBe('https://www.tmz.com/story')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('shows the timeout page when fetch aborts', async () => {
    const abortError = new Error('The operation was aborted')
    abortError.name = 'AbortError'
    fetchMock.mockRejectedValue(abortError)
    const res = await app.request(articleURL('https://www.pcgamer.com/story'))
    expect(res.status).toBe(200)
    expect(await res.text()).toBe('<p>Request Timeout</p>')
  })

  it('redirects to the source on any other fetch error', async () => {
    fetchMock.mockRejectedValue(new Error('offline'))
    const res = await app.request(articleURL('https://www.nbcnews.com/story'))
    expect(res.status).toBe(302)
    expect(res.headers.get('location')).toBe('https://www.nbcnews.com/story')
  })
})
