package web

import (
	"embed"
	"io/fs"
)

//go:embed static/app.js
//go:embed static/board-grid.svg
//go:embed static/css/base.css
//go:embed static/css/controls.css
//go:embed static/css/diagnostics.css
//go:embed static/css/dialogs.css
//go:embed static/css/layout.css
//go:embed static/css/projects.css
//go:embed static/css/responsive.css
//go:embed static/css/skills.css
//go:embed static/css/tokens.css
//go:embed static/favicon.svg
//go:embed static/fonts/InterVariable.woff2
//go:embed static/index.html
//go:embed static/js/projects/arrangement.js
//go:embed static/js/projects/board.js
//go:embed static/js/projects/cards.js
//go:embed static/js/projects/details.js
//go:embed static/js/projects/editor.js
//go:embed static/js/projects/model.js
//go:embed static/js/projects/removal.js
//go:embed static/js/projects/summary.js
//go:embed static/js/shared/api.js
//go:embed static/js/shared/appearance.js
//go:embed static/js/shared/dialogs.js
//go:embed static/js/shared/dom.js
//go:embed static/js/shared/icons.js
//go:embed static/js/shared/project.js
//go:embed static/js/shared/state.js
//go:embed static/js/skills/catalog.js
//go:embed static/js/skills/controller.js
//go:embed static/js/skills/preview.js
//go:embed static/js/skills/state.js
//go:embed static/js/skills/views.js
//go:embed static/js/workspace/discovery.js
//go:embed static/js/workspace/events.js
//go:embed static/js/workspace/launch.js
//go:embed static/js/workspace/logs.js
//go:embed static/js/workspace/navigation.js
//go:embed static/js/workspace/registry.js
//go:embed static/js/workspace/services.js
//go:embed static/skills.js
//go:embed static/styles.css
//go:embed static/vendor/htmx.min.js
//go:embed static/vendor/sortable.esm.js
//go:embed templates/logs.html
var Files embed.FS

var Static = staticFiles()

func staticFiles() fs.FS {
	files, err := fs.Sub(Files, "static")
	if err != nil {
		panic(err)
	}
	return files
}
