export interface Feed {
  title: string
  url: string
}

export const feeds: Feed[] = [
  { title: 'CNBC', url: 'https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114' },
  { title: 'Hacker News', url: 'https://news.ycombinator.com/rss' },
  { title: 'NBC News', url: 'https://feeds.nbcnews.com/nbcnews/public/news' },
  { title: 'ClickOnDetroit', url: 'https://www.clickondetroit.com/arc/outboundfeeds/rss/category/news/?outputType=xml' },
].sort((a, b) => a.title.localeCompare(b.title))
