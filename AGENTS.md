# Workbench source

Read README.md and docs/setup.md first. Production uses Go standard library only, with embedded local htmx assets.
Before starting a server, inspect the local registry and live reservations with
`./workbench status --json` and `./workbench check-port PORT`.
Reuse a healthy instance of this installation. Do not stop unknown or duplicate
processes automatically. Use the CLI to update registry records atomically.
Project launch requires an explicit reviewed plan; stop requires a fresh process
identity check and human confirmation. Discovery never executes project code.
Never read secret files. Keep registry.json, browser preferences, launch logs and
runtime settings out of source archives. Tests use isolated temporary fixtures.

Build `./cmd/workbench` through Make; the root contains no Go package.
Read docs/architecture.md before changing package boundaries. Keep Go tests beside their
packages, public assets/templates in web/, and UI checks in tests/ui/. Legacy Python is
isolated in legacy/python/; scripts/compat/ contains transitional entry points.
When adding an embedded asset, update web/embed.go and internal/distribution/archive.go.
Agent access is documented in docs/agents.md. MCP defaults to read-only; optional catalog
writes require revisions. MCP must not bypass the reviewed dashboard launch/stop controls.
