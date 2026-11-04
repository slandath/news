// Package scrape fetches article pages and extracts their text.
package scrape

import "github.com/PuerkitoBio/goquery"

// SiteConfig describes how to pull a title and body out of one outlet's
// article page.
//
// ArticleWrapper returns plain strings, never markup. That is the structural
// half of the text-only guarantee: there is no type-level path from scraped
// HTML to a raw template write. The wrapper owns its selector so it cannot
// drift from what it extracts; Article records the same selector for callers
// that want to know whether a page has any body text at all.
type SiteConfig struct {
	Article        string
	Title          string
	ArticleWrapper func(*goquery.Document) []string
}

// Configs maps a hostname (with any leading "www." removed) to its scraping
// config. Hosts missing from this map are not scraped; the article handler
// redirects them to the source URL instead.
var Configs = map[string]SiteConfig{
	"bbc.co.uk": {
		Article: "article p",
		Title:   "h1#main-heading",
		ArticleWrapper: func(doc *goquery.Document) []string {
			return texts(doc, "article p")
		},
	},
	"clickondetroit.com": {
		Article: "p.article-text",
		Title:   "h1.headline",
		ArticleWrapper: func(doc *goquery.Document) []string {
			return texts(doc, "p.article-text")
		},
	},
	"cnbc.com": {
		Article: "div.group p",
		Title:   "h1.ArticleHeader-headline",
		ArticleWrapper: func(doc *goquery.Document) []string {
			return texts(doc, "div.group p")
		},
	},
	"espn.com": {
		Article: "div.article-body p",
		Title:   "h1",
		ArticleWrapper: func(doc *goquery.Document) []string {
			return texts(doc, "div.article-body p")
		},
	},
	"nbcnews.com": {
		Article: "p.body-graf",
		Title:   "h1.article-hero-headline__htag",
		ArticleWrapper: func(doc *goquery.Document) []string {
			return texts(doc, "p.body-graf")
		},
	},
	"pcgamer.com": {
		Article: "div#article-body p",
		Title:   "h1",
		ArticleWrapper: func(doc *goquery.Document) []string {
			return texts(doc, "div#article-body p")
		},
	},
	"slashfilm.com": {
		Article: "div.columns-holder p",
		Title:   "h1.title-gallery",
		ArticleWrapper: func(doc *goquery.Document) []string {
			return texts(doc, "div.columns-holder p")
		},
	},
}

// texts returns the text content of every element matching selector, with all
// markup dropped. It is the exact equivalent of cheerio's
// `$(selector).map((_, el) => $(el).text()).get()`.
//
// SECURITY: never replace this with a helper that preserves inner HTML. Text
// extraction is what keeps hostile markup out of the response.
func texts(doc *goquery.Document, selector string) []string {
	selection := doc.Find(selector)
	out := make([]string, 0, selection.Length())
	selection.Each(func(_ int, s *goquery.Selection) {
		out = append(out, s.Text())
	})
	return out
}
