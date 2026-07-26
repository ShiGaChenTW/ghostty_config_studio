package main

// version is injected at build time with
// `-ldflags "-X main.version=<tag>"`, which the Homebrew formula supplies
// from the version it is packaging. It was a hand-edited constant until
// v0.1.9 shipped still claiming to be 0.1.8 — the formula's own test
// compares this string against the packaged version, so the two can never
// be allowed to drift again. "dev" is what a plain `go build` reports.
var version = "dev"
