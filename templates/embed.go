package templates

import "embed"

//go:embed *.tmpl robots.txt
var FS embed.FS
