package main

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedDist embed.FS

func getEmbeddedDistFS() (fs.FS, error) {
	return fs.Sub(embeddedDist, "dist")
}
