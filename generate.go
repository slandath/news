package main

// Regenerate the templ components after editing any *.templ file.
// The generated *_templ.go files are committed; CI fails if they are stale.
//
//go:generate go tool templ generate
