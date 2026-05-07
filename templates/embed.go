// Package templates embeds reportd's HTML templates and static assets.
package templates

import "embed"

// FS holds the embedded *.tmpl and robots.txt files.
//
//go:embed *.tmpl robots.txt
var FS embed.FS
