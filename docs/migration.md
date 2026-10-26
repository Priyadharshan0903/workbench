# Go migration

This branch provides a Go runtime and CLI, with embedded frontend assets and the
existing dashboard's JSON API. Its CLI uses Cobra, skill Markdown previews use Goldmark,
and the HTTP/MCP transports use the Go standard library. The UI keeps
its board, preferences and interaction model; launch logs are the first htmx view.
The dark dashboard uses neutral surfaces and locally bundled Inter 4.1. Workbench's own record
is shown in the engine header and Diagnostics; project totals exclude it. Older theme preferences
migrate to dark once, preserving favourites, recent opens and arrangement.

## Implemented

- Version-1 registry compatibility, shared OS file lock, optimistic revisions,
  atomic replacement and previous-snapshot backup. Concurrent Start requests also
  use the file lock, including requests from separate engine instances.
- Loopback HTTP engine, exact public asset allowlist, Host/Origin/fetch-metadata
  checks, per-engine CSRF token and bounded JSON writes. Health reports version 3
  and runtime `go`; Python v2 engines are never automatically replaced.
- Listener ownership by resolved folder, deepest-root matching and expected-port
  tie breaking. Inspection errors stay unknown. Stop tokens bind PID, birth time,
  working folder, endpoints and project association; Stop rechecks before SIGTERM.
- Reviewed launch plans, literal argv parsing, bounded sequential steps, runtime
  checks, reservation checks and duplicate prevention.
- Detached runners using the same executable's `runner` subcommand, sequential
  execution, inherited environment, bounded log segments, signal forwarding and
  atomic run state. Engine reattachment uses the recorded runner identity.
- Local HTTP readiness, limited concurrent probes, short cache, proxy bypass and
  no redirect following. HTTP response alone does not establish ownership.
- Bounded project discovery (including Go executable suggestions) and linked
  Codex/Claude skill discovery. Discovery never executes project configuration.
- CLI catalog operations, portable macOS launcher and source/runtime ZIPs from
  explicit allowlists, excluding personal catalog and runtime files.
- Locally bundled htmx 2.0.11 and escaped Go HTML log fragments. The JSON logs API
  remains available. htmx eval, response scripts, style injection and history
  snapshots are disabled to preserve the CSP and avoid caching launch output.

## Compatibility and rollout

Build with `make build`; launch with `./workbench start`. Keep the executable in
the installation folder, or pass the global `--root` option. Rebuild before
reopening after changing Go or embedded public assets. Start can reload the same
installation's Go engine after a rebuild; it leaves detached runners alive.

Existing JSON catalogs need no conversion. Python CLI invocations in a source
checkout forward to the binary. Legacy Python backend modules and their tests
remain as a reference and rollback implementation; they are excluded from Go
archives. The old engine can still serve public assets from disk: the log UI uses
JSON when the engine does not advertise HTML-fragment support.

Copy a catalog deliberately if moving installations. Browser preferences remain
specific to their original browser/port. Never copy logs, runner state or port
configuration into distribution archives. This migration does not switch the
shared dashboard registry to a new installation or start/stop existing services.

## Frontend work still available

Registered web projects with a command but missing website/port details now offer **Set up &
start**. Saving those settings opens the reviewed launch plan; it does not run the command.
The existing reservation, duplicate-start and process identity checks remain authoritative.

The project board, discovery/editor forms, service diagnostics and skills view
still use vanilla JavaScript and JSON. Migrating those views to Go templates and
htmx is a separate incremental step. Drag ordering, keyboard interactions,
browser preferences and opening a pending browser tab still need JavaScript.

## Verification limits

`make check` runs race-enabled Go tests, vet and JS syntax checks. UI interaction
tests cover the existing dashboard and real htmx swaps in jsdom. Temporary
fixtures exercise runner execution, duplicate prevention, reattachment, readiness,
logs and failures. Test fixture services are the only services tests stop.

The current hosted macOS environment blocks `ps`, including after an elevated
retry. On such hosts, runner lifecycle tests inject only fixture identity
responses, while using actual child processes, `lsof`, HTTP and run files. Tests
separately exercise changed identities and protected/unmatched stop rejection.
Production never substitutes an identity: unverifiable launches/stops are refused.
In-place Go engine reload was exercised against this installation on port 8732. Validate real
process birth checks and the native folder picker in a normal desktop terminal before using
this build as the daily launcher. Retire the Python reference modules and wrappers after
updating external automation and verifying these remaining desktop checks; the README records
the removal sequence.

Full-browser screenshot verification remains blocked: Chrome exits before rendering, including
on the elevated retry. Functional browser tests use jsdom and do not establish visual verification.
Native signed installers are outside this migration. Runtime archives are specific to the build
OS and architecture.

## Repository organization

The executable entry point is now `cmd/workbench`; backend responsibilities are separate
`internal` packages with package-local tests. Embedded assets moved to `web/static` and log
fragments to `web/templates`. UI checks moved to `tests/ui`. Setup, architecture, design and
migration docs live under `docs`. See [architecture.md](architecture.md) for dependency direction.

Legacy Python and its tests moved to `legacy/python`; forwarding entry points moved to
`scripts/compat`. The optional instruction installer is in `scripts` and now generates Go CLI
commands. These moves do not change the root executable, catalog format, data directory or
public HTTP paths. External callers of root-level Python files need the new compatibility
paths or the direct Go CLI.

## Legacy catalog import and agent access

The 8731 catalog was merged into the new installation using revision-checked CLI writes.
Existing records were preserved; the duplicate record for this Go installation was skipped.
The legacy engine was retained under `workbench-legacy`, paused in the new catalog, with its
8730/8731 reservations preserved. Its process was not stopped. Existing commands were kept
while missing URL/port metadata was filled from matching source records.

A full pre-import catalog backup and the import plan/result are retained under the ignored
`.runtime/backups/` and `.runtime/catalog-import/` folders. These personal records do not enter
distribution archives. Browser preferences and historical runner files remain specific to their
original installation; the import migrates durable project records.

`./workbench mcp` supplies stdio tools directly against the new catalog. The implementation and
CLI are tested for protocol-only stdout, version negotiation, read-only defaults, bounded frames,
port-inspection failures, log masking and stale-write rejection. A real MCP SDK client connected,
listed the tools/projects, read Branchroom and checked its port. No personal client configuration
was registered automatically. See [agents.md](agents.md) for Claude Code/Codex commands.

## CLI readability and Cobra migration

The CLI now builds a fresh Cobra command tree for each invocation. Catalog, engine, inventory
and integration commands live in separate `internal/app/commands_*.go` files. Existing command
names, JSON shapes and exit codes are retained; help is available for every command without
opening a catalog. Global `--root` also works after the subcommand. Invalid flags and argument
counts fail before command work begins. Cobra provides the standard `completion` command.

MCP framing, session dispatch, tool handlers and schemas are separate files with descriptive
names, keyed literals and spacing between logical steps. Responses are serialized once before
writing, with the same response bound and explicit short-write handling. Stdout remains protocol
only. Cobra is pinned in `go.mod`/`go.sum`; dependency licenses accompany both archive formats.

## Readable frontend and concrete data contracts

The dashboard script now has descriptive state names, multiline templates, focused card renderers,
named event handlers and grouped initialization. Form parsing, port parsing and local website
validation are separate functions. It remains a plain JavaScript asset embedded in the Go binary.
The existing workspace storage key/theme migration and reviewed launch/stop flow are preserved.

Known Go payloads now use concrete structs, including public runner summaries, launch results,
HTTP health/action responses, shared port checks, catalog acknowledgements and MCP tool schemas
and results. MCP reuses the encoded tool object for structured content. Generic serialization
and dispatch boundaries retain `any`; domain payloads do not use untyped maps. Verification
covers optional JSON fields, a successful exit code of zero and explicit schema restrictions.

## Skill workspace

Agent skills now open in a wider viewer with Markdown/raw views, supporting files, reference
navigation, clipboard actions, bundle download and reviewed duplication. Personal/project
instruction editing uses a draft, a before/after review and an explicit save. Backups and file
revisions preserve existing work; stale requests return HTTP 409. Plugin-managed and built-in
originals remain read-only, with a separate editable-copy flow.

The new `internal/skills` package handles bounded access and writes; `web/static/skills.js`
owns the browser draft and viewer. Public assets, archive allowlists and Goldmark license notices
are updated. API writes retain same-origin/CSRF checks; file targets and destinations are resolved
server-side. Tests use isolated homes/catalogs and never change installed skills. The fifth UI
suite covers rendering, copying, stale edits, draft retention, managed skills and selection races.
No skill is executed or automatically activated by these flows. Browser screenshot inspection
remains unavailable because Chrome exits before rendering in this host, including the elevated retry.
