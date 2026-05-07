// Package templates embeds reportd's HTML templates and static assets so
// they ship in a single binary.
package templates

import "embed"

// FS holds reportd's HTML templates (*.tmpl) and static assets
// (robots.txt) for use with render.EmbedFileSystem.
//
//go:embed *.tmpl robots.txt
var FS embed.FS
