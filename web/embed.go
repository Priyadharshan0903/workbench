package web

import (
	"embed"
	"io/fs"
)

//go:embed static/index.html static/app.js static/skills.js static/styles.css static/board-grid.svg static/favicon.svg static/vendor/htmx.min.js static/fonts/InterVariable.woff2 templates/logs.html
var Files embed.FS

var Static = staticFiles()

func staticFiles() fs.FS {
	files, err := fs.Sub(Files, "static")
	if err != nil {
		panic(err)
	}
	return files
}
