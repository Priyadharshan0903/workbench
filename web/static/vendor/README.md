# htmx

Vendored htmx 2.0.11, from https://unpkg.com/htmx.org@2.0.11/dist/htmx.min.js.
The upstream license is in HTMX-LICENSE. Runtime assets are served locally and
embedded in the Go executable; no CDN or npm installation is required.

The application disables htmx eval, script processing and injected styles to
respect the existing Content Security Policy. Only same-origin requests are used.

# SortableJS

Vendored SortableJS 1.15.7, from
https://github.com/SortableJS/Sortable/blob/1.15.7/modular/sortable.esm.js
(commit 031649b8116565e02419e8aa2d252d7d8c82b9da).
The MIT license is in third_party/licenses/sortablejs.txt. This native ES module
provides card dragging, animated reordering and auto-scroll. It is embedded and
served locally, with no frontend build or runtime CDN dependency.
