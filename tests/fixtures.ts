import type { CheerioAPI } from 'cheerio'
import type { ParserFeed, SiteConfig } from '../src/types.js'
import * as cheerio from 'cheerio'

/*
 Fixtures are hand-crafted per site using the real selectors from src/siteConfigs.ts.
 Every article fixture embeds hostile payloads (<script>, <img onerror>) so tests
 prove that text-only extraction strips them before anything is rendered.
 */

export interface ArticleFixture {
  /** HTML served by the mocked fetch */
  html: string
  /** Expected title extracted by the config's title selector */
  title: string
  /** Expected paragraphs extracted by the config's articleWrapper */
  paragraphs: string[]
}

/**
 * Removes tags but keeps inner text.
 * Implemented with cheerio itself so it is an exact mirror of the
 * production extraction (html5 parse semantics) instead of a regex
 * approximation — and therefore correct for any input.
 */
export function stripOnlyTags(text: string): string {
  return cheerio.load(text).text()
}

const scriptPayload = `<script>alert(1)</script>`
const imgPayload = `<img src="x.png" onerror="alert(1)">`

export const clickOnDetroitFixture: ArticleFixture = {
  html: `<!doctype html><html><head><title>Local</title></head><body>
    <h1 class="headline">${scriptPayload}Boy rescues cat</h1>
    ${imgPayload}
    <p class="article-text">The kitten is ${scriptPayload}doing fine.</p>
    <p class="article-text">Neighbors are relieved.</p>
  </body></html>`,
  paragraphs: [`The kitten is ${scriptPayload}doing fine.`, 'Neighbors are relieved.'],
  title: `${scriptPayload}Boy rescues cat`,
}

export const cnbcFixture: ArticleFixture = {
  html: `<!doctype html><html><head><title>Markets</title></head><body>
    <h1 class="ArticleHeader-headline">Stocks ${scriptPayload}rally on earnings</h1>
    <div class="group">
      ${imgPayload}
      <p>Futures point higher ${scriptPayload}at the open.</p>
      <p>Analysts expect a quiet week.</p>
    </div>
  </body></html>`,
  paragraphs: [`Futures point higher ${scriptPayload}at the open.`, 'Analysts expect a quiet week.'],
  title: `Stocks ${scriptPayload}rally on earnings`,
}

export const nbcNewsFixture: ArticleFixture = {
  html: `<!doctype html><html><head><title>US</title></head><body>
    <h1 class="article-hero-headline__htag">Storm ${scriptPayload}heads north</h1>
    ${imgPayload}
    <p class="body-graf">Residents hunkered down overnight.</p>
    <p class="body-graf">Power was restored by morning.</p>
  </body></html>`,
  paragraphs: ['Residents hunkered down overnight.', 'Power was restored by morning.'],
  title: `Storm ${scriptPayload}heads north`,
}

export const pcGamerFixture: ArticleFixture = {
  html: `<!doctype html><html><head><title>Games</title></head><body>
    <h1>Best rigs of ${scriptPayload}2026</h1>
    <div id="article-body">
      ${imgPayload}
      <p>The new cards are ${scriptPayload}remarkably cool.</p>
      <p>Prices may drop next quarter.</p>
    </div>
  </body></html>`,
  paragraphs: [`The new cards are ${scriptPayload}remarkably cool.`, 'Prices may drop next quarter.'],
  title: `Best rigs of ${scriptPayload}2026`,
}

export const slashfilmFixture: ArticleFixture = {
  html: `<!doctype html><html><head><title>Movies</title></head><body>
    <h1 class="title-gallery">Sequels ${scriptPayload}return this fall</h1>
    <div class="columns-holder">
      ${imgPayload}
      <p>Production wrapped ${scriptPayload}last week.</p>
      <p>The trailer drops Friday.</p>
    </div>
  </body></html>`,
  paragraphs: [`Production wrapped ${scriptPayload}last week.`, 'The trailer drops Friday.'],
  title: `Sequels ${scriptPayload}return this fall`,
}

/** Site domain -> fixture, matching the keys of src/siteConfigs.ts */
export const articleFixturesByDomain: Record<string, ArticleFixture> = {
  'clickondetroit.com': clickOnDetroitFixture,
  'cnbc.com': cnbcFixture,
  'nbcnews.com': nbcNewsFixture,
  'pcgamer.com': pcGamerFixture,
  'slashfilm.com': slashfilmFixture,
}

/**
 * Runs each config's real selectors against its fixture and returns
 * the extracted title/paragaphs, so tests exercise the actual siteConfigs
 * rather than hopeful copies of them.
 */
export function applyFixture(config: SiteConfig, fixture: ArticleFixture): { article: string[], title: string } {
  const $: CheerioAPI = cheerio.load(fixture.html)
  return {
    article: config.articleWrapper($),
    title: $(config.title).text(),
  }
}

/** RSS 2.0 XML with a hostile title, as a feed server would serve it */
export const rssFeedXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Fixture Feed &amp; Wire</title>
    <link>https://example.com</link>
    <description>A test feed</description>
    <item>
      <title>Item &lt;script&gt;one&lt;/script&gt;</title>
      <link>https://example.com/story-1</link>
      <description>First story &lt;b&gt;bold&lt;/b&gt;</description>
      <pubDate>Wed, 21 Oct 2026 10:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Item two &amp;&amp; more</title>
      <link>https://example.com/story-2</link>
      <pubDate>Wed, 21 Oct 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

/** What rss-parser hands us after parsing rssFeedXML */
export function parseFeedFixture(): ParserFeed {
  return {
    title: 'Fixture Feed & Wire',
    items: [
      {
        title: 'Item one',
        link: 'https://example.com/story-1',
        pubDate: 'Wed, 21 Oct 2026 10:00:00 GMT',
        content: 'First story bold',
      },
      {
        title: 'Item two && more',
        link: 'https://example.com/story-2',
        pubDate: 'Wed, 21 Oct 2026 11:00:00 GMT',
      },
    ],
  }
}
