// Package fixtures exposes the hostile-payload corpus in testdata/ to the
// packages whose tests exercise it. It is imported only by tests.
//
// The expected values are literals, not values computed from the fixtures, so
// a selector that starts pulling markup in fails instead of agreeing with
// whatever the wrapper happens to return.
package fixtures

import (
	"os"
	"path/filepath"
)

// Article is one scraping fixture.
type Article struct {
	// File is the fixture's name inside testdata/.
	File string
	// Title is the text the config's title selector must produce.
	Title string
	// Paragraphs is the text its article wrapper must produce.
	Paragraphs []string
}

// Articles maps a hostname (without "www.") to its fixture.
var Articles = map[string]Article{
	"bbc.co.uk": {
		File:       "bbc.html",
		Title:      "Ocon alert(1)leaves Haas",
		Paragraphs: []string{"The team confirmed the split alert(1)on Tuesday.", "A reserve driver steps in this weekend."},
	},
	"clickondetroit.com": {
		File:       "clickondetroit.html",
		Title:      "alert(1)Boy rescues cat",
		Paragraphs: []string{"The kitten is alert(1)doing fine.", "Neighbors are relieved."},
	},
	"cnbc.com": {
		File:       "cnbc.html",
		Title:      "Stocks alert(1)rally on earnings",
		Paragraphs: []string{"Futures point higher alert(1)at the open.", "Analysts expect a quiet week."},
	},
	"espn.com": {
		File:       "espn.html",
		Title:      "Lakers alert(1)take the West",
		Paragraphs: []string{"The stars closed it out alert(1)late.", "The rematch is Friday."},
	},
	"nbcnews.com": {
		File:       "nbcnews.html",
		Title:      "Storm alert(1)heads north",
		Paragraphs: []string{"Residents hunkered down overnight.", "Power was restored by morning."},
	},
	"pcgamer.com": {
		File:       "pcgamer.html",
		Title:      "Best rigs of alert(1)2026",
		Paragraphs: []string{"The new cards are alert(1)remarkably cool.", "Prices may drop next quarter."},
	},
	"slashfilm.com": {
		File:       "slashfilm.html",
		Title:      "Sequels alert(1)return this fall",
		Paragraphs: []string{"Production wrapped alert(1)last week.", "The trailer drops Friday."},
	},
}

// RSS is the RSS 2.0 fixture, whose first item title contains a script tag.
const RSS = "fixture.rss"

// Read returns the contents of a file in the repository's testdata directory.
//
// The path is relative to the caller's working directory, which for every
// importer is internal/<pkg> — hence the two levels up.
func Read(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join("..", "..", "testdata", name))
}
