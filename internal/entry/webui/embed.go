package webui

import (
	"embed"
	"io/fs"
)

// embeddedFiles contains generated production Web UI assets when they are
// present at build time. The tracked placeholder keeps ordinary Go-only builds
// valid without pretending that an empty asset set is a usable Web UI.
//
//go:embed all:static
var embeddedFiles embed.FS

func embeddedStaticFS() fs.FS {
	static, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		return nil
	}
	return static
}
