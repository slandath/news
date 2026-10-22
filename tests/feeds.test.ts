import { describe, expect, it } from 'vitest'
import { feeds } from '../src/feeds.js'

describe('feeds', () => {
  it('has the eight expected feeds', () => {
    expect(feeds).toHaveLength(8)
  })

  it('is sorted alphabetically by title (locale order, as src/feeds.ts does)', () => {
    const sorted = [...feeds].sort((a, b) => a.title.localeCompare(b.title))
    expect(sorted).toEqual(feeds)
  })

  it('uses https URLs only', () => {
    expect(feeds.every(feed => feed.url.startsWith('https://'))).toBe(true)
  })

  it('has unique URLs and titles', () => {
    expect(new Set(feeds.map(f => f.url)).size).toBe(feeds.length)
    expect(new Set(feeds.map(f => f.title)).size).toBe(feeds.length)
  })
})
