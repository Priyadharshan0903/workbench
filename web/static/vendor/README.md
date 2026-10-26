# htmx

Vendored htmx 2.0.11, from https://unpkg.com/htmx.org@2.0.11/dist/htmx.min.js.
The upstream license is in HTMX-LICENSE. Runtime assets are served locally and
embedded in the Go executable; no CDN or npm installation is required.

The application disables htmx eval, script processing and injected styles to
respect the existing Content Security Policy. Only same-origin requests are used.
