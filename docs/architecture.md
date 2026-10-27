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
  skills/                   Discovered file access, Markdown previews, reviewed edits and copies
  launch/                   Reviewed plans, argv parsing, detached runners, readiness and logs
  httpapi/                  Loopback HTTP routes, CSRF/origin checks and verified stop controls
  mcp/                      Local stdio protocol and agent tools; no service start/stop tools
  distribution/             Clean source/runtime archive creation
  fsutil/                   Bounded reads, path boundaries and atomic file replacement
  testutil/                 Temporary catalogs and inspection fixtures; tests only
web/
  embed.go                  Embeds public assets and templates in the executable
  assets.go                 Explicit public asset manifest shared with source packaging
  static/
    app.js                  Browser entry point and refresh coordination
    skills.js               Skill workspace module entry point
    js/shared/              DOM, API, preferences, appearance and icon helpers
    js/projects/            Board, cards, details, editor and arrangement
    js/skills/              Catalog, controller, state, views and preview filtering
    js/workspace/           Navigation, events, discovery, services, launches and logs
    css/                    Tokens, shared styles and feature styles
    styles.css              Ordered CSS imports; preserves the existing cascade
    fonts/, vendor/         Bundled Inter and htmx
  templates/                Server-rendered HTML fragments
scripts/
  check-frontend.cjs        Syntax checks for first-party browser modules
  compat/                   Transitional Python commands that forward to the Go executable
  install_agent_instructions.py
                            Optional helper; does not install instructions automatically
docs/
  setup.md                  Installation and everyday use
  architecture.md           Package boundaries and repository conventions
  agents.md                 Claude Code/Codex MCP setup and CLI integration
  migration.md              Rollout status and remaining desktop verification
  design.md                 Dashboard design and verification notes
third_party/
  licenses/                 Go dependency license notices for distribution
```

The root holds the README, agent instructions, Go module and Makefile. The
generated `workbench` executable stays at the installation root so existing CLI commands,
catalog paths and engine reloads continue to work. It is ignored by Git, as are `dist/`,
`.runtime/`, catalogs, logs and browser/runtime preferences. Source organization does not move
the user's data.

## Dependency direction

```text
cmd/workbench → app
app          → httpapi, mcp, launch, inventory, catalog, discovery, distribution
httpapi      → launch, inventory, catalog, discovery, skills, web
mcp          → launch, inventory, catalog, discovery
launch       → catalog, inventory, fsutil
inventory    → catalog, fsutil
discovery    → catalog, fsutil
skills       → discovery, catalog, fsutil
catalog      → fsutil
distribution → fsutil, web
```

The lower packages never import `app` or `httpapi`. Catalogs do not inspect processes or make
HTTP requests. Inventory reports process evidence without launching anything. Discovery reads
bounded named files without executing project code. Launching requires a reviewed plan and
checks the current catalog and listener evidence. HTTP stop controls recheck process identity.

`httpapi/stop.go` prepares signed stop-review tokens and handles confirmed stop requests.
Registered web listeners use the existing project ownership rules. On macOS, unregistered
listeners can also be reviewed when they have a specific working folder and `ps` confirms a
non-root UID matching Workbench's user. Their token binds the UID, PID, process name, birth time,
working folder, endpoints and project association. The stop endpoint captures fresh listeners,
rechecks eligibility and identity, and sends SIGTERM only to the reviewed PID. It does not
register the process, signal process groups, force-kill or bypass the dashboard confirmation.
MCP remains unable to stop processes.

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

`web/static/app.js` is the native ES module entry point. It constructs feature controllers,
connects refresh/review callbacks and initializes the page. Feature modules do not import the
entry point; their dependencies are explicit imports or constructor arguments. There are no
window-global application APIs, framework dependencies or production npm build steps.

Shared DOM escaping, clipboard, icons, API access and preferences live in `js/shared`.
Project rendering/editing/arrangement live in `js/projects`, while workspace navigation,
discovery and process interactions live in `js/workspace`. CSS is grouped by responsibility
under `css`; `styles.css` imports those files in their original cascade order. Templates escape
external values before inserting HTML. Keep first-party source formatted and readable.

Native dialogs keep their header and close button outside the scrollable `.dialog-content`.
While any dialog is open, the page root and body disable scrolling; the content scroll area
contains overscroll. Shared dialog events dismiss a backdrop click only when the pointer press
also started outside the panel. Close buttons and backdrop clicks use the same cancellable
path as Escape, preserving the skill workspace's unsaved-draft guard and native focus return.

`web/static/skills.js` re-exports the workspace factory. `js/skills` separates its controller,
state, rendering and preview filtering. The dashboard opens the viewer
only when the engine advertises `skillFiles`. File APIs resolve discovered IDs on the server;
relative files must appear in the bounded bundle inventory. Go 1.24 `os.Root` operations contain
bundle reads and new copy files. Skill content is data and is never executed by these APIs.

Goldmark renders previews with raw HTML/unsafe links disabled. The browser further restricts
preview elements and removes images before attaching the preview. Personal/project instruction
saves verify a content revision, coordinate with a file lock, keep a private runtime backup and
atomically replace the file after a final identity/revision check. Managed originals are read-only.
Duplication reserves a new destination folder exclusively and writes SKILL.md last; it does not
merge with an existing skill. Copies preserve supporting files and executable bits, exclude
listed private/link/dependency entries and keep external references unchanged. Copy write failures
report the new incomplete folder rather than deleting unknown files. No new MCP write tools are added.

## Tests and changes

Go tests live beside their package. Cross-package black-box tests use an external test package;
shared fixture construction stays in `internal/testutil` and is never imported by production
code. Runner integration tests create isolated services and temporary catalogs.

```sh
make check
```

The legacy Python backend and standalone UI test harness have been retired. Browser interaction
and visual QA are manual. Optional compatibility commands remain in `scripts/compat`: prefer the
direct Go CLI in new automation. The instruction installer generates Go CLI references.

## Build and distribution

Build the command package, not the repository root:

```sh
go build -trimpath -o workbench ./cmd/workbench
make release
make package
```

`make run` builds and starts/reuses the local engine. `make serve` builds and runs it in the
foreground. Both pass an optional `PORT`; the background target also accepts `NO_BROWSER=1`.
The Start command owns port selection and instance reuse, keeping Make a thin interface.

`web/embed.go` owns the explicit embedded-file list. `web/assets.go` owns the public URL-to-file
manifest consumed by both HTTP serving and source packaging. Public URL paths stay independent of the
source layout: `/app.js` is backed by `web/static/app.js`, for example. Go HTML fragments live
under `web/templates` and are parsed from the same embedded filesystem.

`internal/distribution` owns the archive allowlist. Source archives contain the Go command,
internal packages and tests, web assets/templates, docs and the JavaScript syntax checker.
They include `go.mod`, `go.sum` and the Go dependency licenses. Source builds download the pinned
modules if they are not cached; runtime archives carry the compiled code and license notices.
Legacy Python, compatibility commands, personal records and runtime data are excluded.
Runtime archives contain the root executable, setup/migration docs and third-party
licenses. When adding a browser asset, update both the embed list and the public manifest.
Non-public distribution documents/licenses remain in the packaging allowlist.

`make check` checks syntax for every browser module through `scripts/check-frontend.cjs`.
There is no standalone browser test harness. Manually check projects, discovery, reviewed
launch/stop, logs, themes and skill view/edit/copy flows after frontend changes.

An exported source archive is verified by building and testing it in a separate temporary
directory; a successful build in the working checkout alone does not establish portability.
