// Package feeds holds the RSS sources the home page links to.
package feeds

// Feed is one source in the navigation list.
type Feed struct {
	Title   string
	URL     string
	Section string
}

// Sections group feeds on the home page.
const (
	SectionNews   = "News"
	SectionSports = "Sports"
)

// All is the feed list in the order the site displays it.
//
// The order is written out literally instead of being sorted at startup. The
// TypeScript version sorted with localeCompare (ICU collation), which puts
// "ClickOnDetroit" before "CNBC"; Go's byte-wise sort does the opposite. The
// literal order keeps the shipped UI identical. See go-migration-plan.md §8.2.
//
// All[0] is the default feed for /feed when no url parameter is supplied.
var All = []Feed{
	{Title: "/Film", URL: "https://www.slashfilm.com/feed/", Section: SectionNews},
	{Title: "AP News", URL: "https://news.google.com/rss/search?q=site%3Aapnews.com&hl=en-US&gl=US&ceid=US%3Aen", Section: SectionNews},
	{Title: "ClickOnDetroit", URL: "https://www.clickondetroit.com/arc/outboundfeeds/rss/category/news/?outputType=xml", Section: SectionNews},
	{Title: "CNBC", URL: "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100003114", Section: SectionNews},
	{Title: "Hacker News", URL: "https://news.ycombinator.com/rss", Section: SectionNews},
	{Title: "NBC News", URL: "https://feeds.nbcnews.com/nbcnews/public/news", Section: SectionNews},
	{Title: "PC Gamer", URL: "https://www.pcgamer.com/rss/", Section: SectionNews},
	{Title: "TMZ", URL: "https://www.tmz.com/rss.xml", Section: SectionNews},
	{Title: "NBA", URL: "https://www.espn.com/espn/rss/nba/news", Section: SectionSports},
	{Title: "NFL", URL: "https://www.espn.com/espn/rss/nfl/news", Section: SectionSports},
	{Title: "Tennis", URL: "https://feeds.bbci.co.uk/sport/tennis/rss.xml", Section: SectionSports},
	{Title: "Boxing", URL: "https://feeds.bbci.co.uk/sport/boxing/rss.xml", Section: SectionSports},
	{Title: "F1", URL: "https://feeds.bbci.co.uk/sport/formula1/rss.xml", Section: SectionSports},
}
