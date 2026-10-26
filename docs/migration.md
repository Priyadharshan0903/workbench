# Go migration

This branch provides a Go runtime and CLI, with embedded frontend assets and the
existing dashboard's JSON API. It uses the Go standard library only. The UI keeps
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
the installation folder, or pass `--root` before the subcommand. Rebuild before
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
