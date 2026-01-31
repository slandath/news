import type { CheerioAPI } from 'cheerio'

export interface ArticleTemplateProps {
  title: string
  article: string
  url: string
}

export interface BaseFeed {
  title: string
  url: string
}

export interface ParserFeed {
  title?: string
  items: Array<{
    title?: string
    link?: string
    pubDate?: string
    content?: string
  }>
}

export interface Feed extends BaseFeed {
  encodedURL: string
}

export interface FeedTemplateProps {
  feed: ParserFeed
  feeds: Feed[]
}

export interface SiteConfig {
  article: string
  articleWrapper: ($: CheerioAPI) => string
  title: string
}
