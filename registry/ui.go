package registry

import (
	"embed"
	"io/fs"
)

//go:embed all:ui/dist
var uiBundle embed.FS

func UI() fs.FS {
	assets, err := fs.Sub(uiBundle, "ui/dist")
	if err != nil {
		return uiBundle
	}
	return assets
}

func UIBuilt() bool {
	_, err := fs.Stat(UI(), "index.html")
	return err == nil
}
