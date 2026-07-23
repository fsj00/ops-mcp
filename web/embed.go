// Package web embeds the ops-mcp browser UI (static assets).
package web

import "embed"

// FS contains the embedded static files served at GET /.
//
//go:embed index.html
var FS embed.FS
