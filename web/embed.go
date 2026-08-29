package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns a filesystem containing the React app build.
func Dist() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
