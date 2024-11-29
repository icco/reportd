package static

import (
	"embed"
)

//go:embed *
var files embed.FS

// Get returns the file contents of a file in the static directory.
func Get(name string) ([]byte, error) {
	return files.ReadFile(name)
}
