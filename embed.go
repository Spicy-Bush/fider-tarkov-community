package main

import "embed"

//go:embed migrations views locale all:dist misc etc favicon.png robots.txt robots-dev.txt ssr.js
var EmbeddedFS embed.FS
