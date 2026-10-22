import { describe, expect, it } from 'vitest'
import { siteConfigs } from '../src/siteConfigs.js'
import { applyFixture, articleFixturesByDomain, stripOnlyTags } from './fixtures.js'

/** cheerio's .text() strips tags but keeps their inner text — see stripOnlyTags in fixtures */

describe('siteConfigs', () => {
  it('has configs for exactly the five scraping sites', () => {
    expect(Object.keys(siteConfigs).sort()).toEqual([
      'clickondetroit.com',
      'cnbc.com',
      'nbcnews.com',
      'pcgamer.com',
      'slashfilm.com',
    ])
  })

  it('knows nothing about outlets without configs (they rely on redirect fallback)', () => {
    expect(siteConfigs['tmz.com']).toBeUndefined()
    expect(siteConfigs['news.ycombinator.com']).toBeUndefined()
    expect(siteConfigs['news.google.com']).toBeUndefined()
  })

  for (const [domain, fixture] of Object.entries(articleFixturesByDomain)) {
    describe(domain, () => {
      const fx = fixture

      it('extracts the title and paragraphs via its real selectors', () => {
        expect.hasAssertions() // guard for dynamic lookups above
        const config = siteConfigs[domain]
        expect(config, `${domain} has no config`).toBeDefined()
        const { article, title } = applyFixture(config!, fx)

        expect(title).toBe(stripOnlyTags(fx.title))
        expect(article).toEqual(fx.paragraphs.map(stripOnlyTags))
        expect(article.length).toBe(2)
      })

      it('strips all markup, so no tags or attributes survive extraction', () => {
        const config = siteConfigs[domain]!
        const { article, title } = applyFixture(config, fx)
        for (const text of [title, ...article]) {
          expect(text).not.toMatch(/<\/?[a-z][^>]*>/i)
          expect(text).not.toContain('onerror=')
        }
      })
    })
  }
})
