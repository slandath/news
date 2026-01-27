export interface Feed {
  title: string
  url: string
}

export const feeds: Feed[] = [
  { title: 'CNBC', url: 'https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114' },
  { title: 'Hacker News', url: 'https://news.ycombinator.com/rss' },
  { title: 'NBC News', url: 'https://feeds.nbcnews.com/nbcnews/public/news' },
  { title: 'ClickOnDetroit', url: 'https://www.clickondetroit.com/arc/outboundfeeds/rss/category/news/?outputType=xml' },
  { title: 'AP News', url: 'https://news.google.com/rss/search?q=site%3Aapnews.com&hl=en-US&gl=US&ceid=US%3Aen' },
  { title: '/Film', url: 'https://www.slashfilm.com/feed/' },
  { title: 'TMZ', url: 'https://www.tmz.com/rss.xml' },
  { title: 'PC Gamer', url: 'https://www.pcgamer.com/rss/' },
].sort((a, b) => a.title.localeCompare(b.title))
