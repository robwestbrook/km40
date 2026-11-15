package main

import "embed"

// The SvelteKit build is embedded so the dashboard ships as one binary.
//
//go:embed all:web/build
var webBuild embed.FS
