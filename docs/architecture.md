# Repository architecture

Workbench is one Go executable with a browser interface. Packages follow the jobs the app does:
catalog storage, process inspection, discovery, supervised launches and HTTP access. The command
entry point only connects the process arguments and standard streams to the application.

## Source layout

```text
cmd/
  workbench/main.go          Executable entry point
internal/
  app/                      Cobra commands, configuration and engine lifecycle
  catalog/                  Project validation, registry revisions and atomic transactions
  inventory/                TCP ownership inspection and workspace snapshots
  discovery/                Bounded project and agent-skill discovery
  launch/                   Reviewed plans, argv parsing, detached runners, readiness and logs
  httpapi/                  Loopback HTTP routes, CSRF/origin checks and verified stop controls
  mcp/                      Local stdio protocol and agent tools; no service start/stop tools
  distribution/             Clean source/runtime archive creation
  fsutil/                   Bounded reads, path boundaries and atomic file replacement
  testutil/                 Temporary catalogs and inspection fixtures; tests only
web/
  embed.go                  Embeds public assets and templates in the executable
  static/                   HTML, CSS, JavaScript, icons, fonts and vendored htmx
  templates/                Server-rendered HTML fragments
tests/
  ui/                       jsdom interaction tests and optional Playwright checks
scripts/
  test-ui.sh                Runs the UI suites without a production npm dependency
  compat/                   Transitional Python commands that forward to the Go executable
  install_agent_instructions.py
                            Optional helper; does not install instructions automatically
legacy/
  python/                   Previous backend retained for migration verification
    tests/                  Python-only regression and process fixtures
docs/
  setup.md                  Installation and everyday use
  architecture.md           Package boundaries and repository conventions
  agents.md                 Claude Code/Codex MCP setup and CLI integration
  migration.md              Rollout status and remaining desktop verification
  design.md                 Dashboard design and verification notes
third_party/
  licenses/                 Go dependency license notices for distribution
```

The root holds the README, agent instructions, Go module, Makefile and macOS launcher. The
generated `workbench` executable stays at the installation root so existing CLI commands,
catalog paths and engine reloads continue to work. It is ignored by Git, as are `dist/`,
`.runtime/`, catalogs, logs and browser/runtime preferences. Source organization does not move
the user's data.

## Dependency direction

```text
cmd/workbench → app
app          → httpapi, mcp, launch, inventory, catalog, discovery, distribution
httpapi      → launch, inventory, catalog, discovery, web
mcp          → launch, inventory, catalog, discovery
launch       → catalog, inventory, fsutil
inventory    → catalog, fsutil
discovery    → catalog, fsutil
catalog      → fsutil
distribution → fsutil
```

The lower packages never import `app` or `httpapi`. Catalogs do not inspect processes or make
HTTP requests. Inventory reports process evidence without launching anything. Discovery reads
bounded named files without executing project code. Launching requires a reviewed plan and
checks the current catalog and listener evidence. HTTP stop controls recheck process identity.

No `pkg/`, service/repository interfaces or dependency-injection framework is needed for this
single application. Introduce another package when a responsibility has a real independent
boundary, rather than creating a folder for every source file.

CLI command constructors are grouped by responsibility in `internal/app/commands_*.go`;
`cli.go` connects the root command, streams, installation path and exit status. Each call to
`Run` creates fresh flag state. Cobra owns parsing, help and completion; command handlers own
catalog operations. Validation and help do not initialize a registry or start an engine.
MCP separates transport (`server.go`), session dispatch (`protocol.go`), tool implementations
(`handlers.go`) and tool metadata (`tools.go`).

Prefer names such as `registry`, `project`, `snapshot`, `contents` and `err` over one-letter
domain variables. Separate logical steps with blank lines, use keyed multiline literals for
protocol objects, and extract functions at meaningful responsibility boundaries. Standard Go
conventions such as `err` and short loop indexes remain appropriate. Format Go files with gofmt.

Known data shapes use concrete structs: catalog registration acknowledgements, port checks,
public run summaries, launch/stop results, health responses and MCP schemas/results. Keep
`any` at JSON encoding/decoding and method-dispatch boundaries where the payload type actually
varies. Protocol tests may decode heterogeneous or deliberately invalid fixtures dynamically.
Do not replace `any` with an alias or `interface{}` just to hide its use.

`web/static/app.js` remains a plain browser script with no production npm build step. It keeps
state and preferences explicit, uses named card renderers and event handlers, separates form
parsing from website validation, and groups initialization/event bindings at the end. Templates
escape external values before inserting HTML. Keep this first-party source formatted and readable;
vendored dependencies retain their upstream distribution format. Run all four UI suites after
changes to rendering, bindings, preference migration or reviewed launch/stop flows.

## Tests and changes

Go tests live beside their package. Cross-package black-box tests use an external test package;
shared fixture construction stays in `internal/testutil` and is never imported by production
code. Runner integration tests create isolated services and temporary catalogs.

```sh
make check
JSDOM_MODULE=/path/to/jsdom make check-ui NODE=/path/to/node
python3 -m unittest discover -s legacy/python/tests -v
```

The Python suite is a migration baseline, not a dependency of the Go application or its build.
Compatibility commands moved to `scripts/compat`: prefer the direct Go CLI in new automation.
The optional instruction installer now generates references to the Go executable.

## Build and distribution

Build the command package, not the repository root:

```sh
go build -trimpath -o workbench ./cmd/workbench
make release
make package
```

`make run` builds and starts/reuses the local engine. `make serve` builds and runs it in the
foreground. Both pass an optional `PORT`; the background target also accepts `NO_BROWSER=1`.
The launcher owns port selection and instance reuse, keeping Make a thin interface.

`web/embed.go` owns the explicit embedded-file list. Public URL paths stay independent of the
source layout: `/app.js` is backed by `web/static/app.js`, for example. Go HTML fragments live
under `web/templates` and are parsed from the same embedded filesystem.

`internal/distribution` owns the archive allowlist. Source archives contain the Go command,
internal packages and tests, web assets/templates, docs, UI checks and their shell runner.
They include `go.mod`, `go.sum` and the Go dependency licenses. Source builds download the pinned
modules if they are not cached; runtime archives carry the compiled code and license notices.
Legacy Python, compatibility commands, personal records and runtime data are excluded.
Runtime archives contain the root executable, launcher, setup/migration docs and third-party
licenses. When adding an embedded asset, update both the embed list and archive allowlist.

An exported source archive is verified by building and testing it in a separate temporary
directory; a successful build in the working checkout alone does not establish portability.
