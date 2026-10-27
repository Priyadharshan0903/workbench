package web

import "maps"

// publicAssets is the complete HTTP/source-export allowlist for browser assets.
var publicAssets = map[string]string{
	"/app.js":                     "app.js",
	"/board-grid.svg":             "board-grid.svg",
	"/css/base.css":               "css/base.css",
	"/css/controls.css":           "css/controls.css",
	"/css/diagnostics.css":        "css/diagnostics.css",
	"/css/dialogs.css":            "css/dialogs.css",
	"/css/layout.css":             "css/layout.css",
	"/css/projects.css":           "css/projects.css",
	"/css/responsive.css":         "css/responsive.css",
	"/css/skills.css":             "css/skills.css",
	"/css/tokens.css":             "css/tokens.css",
	"/favicon.svg":                "favicon.svg",
	"/fonts/InterVariable.woff2":  "fonts/InterVariable.woff2",
	"/":                           "index.html",
	"/js/projects/arrangement.js": "js/projects/arrangement.js",
	"/js/projects/board.js":       "js/projects/board.js",
	"/js/projects/cards.js":       "js/projects/cards.js",
	"/js/projects/details.js":     "js/projects/details.js",
	"/js/projects/editor.js":      "js/projects/editor.js",
	"/js/projects/model.js":       "js/projects/model.js",
	"/js/projects/removal.js":     "js/projects/removal.js",
	"/js/projects/summary.js":     "js/projects/summary.js",
	"/js/shared/api.js":           "js/shared/api.js",
	"/js/shared/appearance.js":    "js/shared/appearance.js",
	"/js/shared/dialogs.js":       "js/shared/dialogs.js",
	"/js/shared/dom.js":           "js/shared/dom.js",
	"/js/shared/icons.js":         "js/shared/icons.js",
	"/js/shared/project.js":       "js/shared/project.js",
	"/js/shared/state.js":         "js/shared/state.js",
	"/js/skills/catalog.js":       "js/skills/catalog.js",
	"/js/skills/controller.js":    "js/skills/controller.js",
	"/js/skills/preview.js":       "js/skills/preview.js",
	"/js/skills/state.js":         "js/skills/state.js",
	"/js/skills/views.js":         "js/skills/views.js",
	"/js/workspace/discovery.js":  "js/workspace/discovery.js",
	"/js/workspace/events.js":     "js/workspace/events.js",
	"/js/workspace/launch.js":     "js/workspace/launch.js",
	"/js/workspace/logs.js":       "js/workspace/logs.js",
	"/js/workspace/navigation.js": "js/workspace/navigation.js",
	"/js/workspace/registry.js":   "js/workspace/registry.js",
	"/js/workspace/services.js":   "js/workspace/services.js",
	"/skills.js":                  "skills.js",
	"/styles.css":                 "styles.css",
	"/vendor/htmx.min.js":         "vendor/htmx.min.js",
	"/vendor/sortable.esm.js":     "vendor/sortable.esm.js",
}

func Asset(urlPath string) (string, bool) {
	filename, found := publicAssets[urlPath]
	return filename, found
}

// PublicAssets returns a copy so callers cannot change the serving allowlist.
func PublicAssets() map[string]string { return maps.Clone(publicAssets) }
