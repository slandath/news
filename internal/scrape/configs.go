// Package scrape fetches article pages and extracts their text.
package scrape

import "github.com/PuerkitoBio/goquery"

// SiteConfig describes how to pull a title and body out of one outlet's
// article page. Each selector appears exactly once: Article is read with
// texts(), which returns plain strings, never markup. That is the
// structural half of the text-only guarantee: there is no type-level path
// from scraped HTML to a raw template write, and no second copy of the
// selector to drift.
type SiteConfig struct {
	Article string
	Title   string
}

// Configs maps a hostname (with any leading "www." removed) to its scraping
// config. Hosts missing from this map are not scraped; the article handler
// redirects them to the source URL instead.
var Configs = map[string]SiteConfig{
	"bbc.co.uk": {
		Article: "article p",
		Title:   "h1#main-heading"},
	"clickondetroit.com": {
		Article: "p.article-text",
		Title:   "h1.headline"},
	"cnbc.com": {
		Article: "div.group p",
		Title:   "h1.ArticleHeader-headline"},
	"espn.com": {
		Article: "div.article-body p",
		Title:   "h1"},
	"nbcnews.com": {
		Article: "p.body-graf",
		Title:   "h1.article-hero-headline__htag"},
	"pcgamer.com": {
		Article: "div#article-body p",
		Title:   "h1"},
	"slashfilm.com": {
		Article: "div.columns-holder p",
		Title:   "h1.title-gallery"},
}

// texts returns the text content of every element matching selector, with all
// markup dropped.
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
