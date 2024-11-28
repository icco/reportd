package static

import (
	"embed"
)

//go:embed *.js *.html
var Files embed.FS
