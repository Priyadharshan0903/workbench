# Workbench

Find your local projects, start them, and pick up where you left off.

Workbench is a personal local workspace for web projects, installed apps, references and agent
skills. It keeps the project catalog on disk, checks real local services, and provides an
explicit Start → Ready → Open workflow for registered web projects.

## Open

From a source checkout, build and start or reuse Workbench with one command:

```sh
make run
```

It opens the browser and returns to your terminal. It respects `.runtime/local.json`;
this installation now defaults to port **8732**. A fresh installation defaults to **8730**.
For an explicit port, use `make run PORT=8732`; add `NO_BROWSER=1` to skip opening a browser.

On macOS, **Open Workbench.command** and the compiled CLI are also available:

```sh
./workbench start
```

Workbench now runs as a Go executable with its dashboard assets embedded. Build from source with
Go 1.24+ (`make build`), or use an executable package for your OS and architecture. The executable
needs neither Python nor Node. macOS is the primary platform; process inspection uses `lsof` and
`ps`. Linux is supported when those utilities are installed. Windows is not supported.
A project's own dependencies and runtime still need to be installed.

The default address for a fresh installation is **http://127.0.0.1:8730/**. An optional
`.runtime/local.json` containing `{"port":8731}` selects another port. Check the shared local
catalog and actual listeners before choosing one; an older installation may reserve these ports.

The launcher reuses a healthy Go instance of this installation and refuses an occupied port or
another runtime. After rebuilding, reopening the launcher reloads a changed Go engine in place;
detached project runners keep their processes. It never stops an older Python installation to
make room. Corrupt catalogs are reported rather than reset. A new installation gets its own
empty catalog containing only Workbench.

The backend and CLI run in Go. The dashboard opens in dark mode with a compact project board,
command previews and a separate engine indicator. Launch logs use locally bundled htmx and Go
HTML templates; other dashboard interactions use the JSON API and vanilla JavaScript. Both the
font and htmx are served locally. See [migration notes](docs/migration.md) for the remaining migration work.

## Run the compiled version

From this source checkout:

```sh
make check
make release
./workbench start --port 8732
```

`make release` builds an optimized executable without Go's symbol/debug tables. There is no
development server, npm bundle or Python process involved. Open the address printed by Start;
with the example port it is **http://127.0.0.1:8732/**. Check reservations before selecting a port.
Port 8732 is the current development installation; use the same port to reuse it after a rebuild.

Start detaches the engine, opens the browser and returns to your terminal. For a foreground
process managed by your terminal or a service supervisor, use Serve:

```sh
./workbench serve --port 8732
```

Choose **Start or Serve** for a given port. Serve binds directly; it cannot replace an engine
already listening there. Ctrl+C stops a foreground engine; detached project runners continue.
There is currently no public CLI command for stopping a background engine or starting/stopping
individual projects. Project launch and stop use the dashboard's reviewed confirmation flows.

`--port` applies only to that invocation. To make a port the default, create
`.runtime/local.json` with the following content before using the double-click launcher or
`./workbench start` without a port:

```json
{"port":8732}
```

Rebuild after changing Go code, CSS, HTML, JavaScript or the bundled font; all public assets are
embedded. Run Start again on the same port to reload a changed build of this installation.
Closing or refreshing a browser tab alone does not load a newly built executable.

An executable package runs without Go, Node or Python installed. It still needs `lsof`, `ps`,
a writable installation/catalog folder and the runtimes required by your individual projects.
This release runs on loopback as a personal local tool; it is not a public multi-user web service.

## CLI reference

Run commands from the folder containing the executable. The global `--root` option goes
**before** the subcommand and selects the folder containing the catalog and `.runtime` data.

| Command | Purpose |
| --- | --- |
| `./workbench --help` | Show the command list. |
| `./workbench start [--port PORT] [--no-browser]` | Start or reuse this installation in the background; normally opens a browser. |
| `./workbench serve [--port PORT]` | Run the engine in the foreground; Ctrl+C stops the engine. |
| `./workbench list [--json]` | List durable project records, including stopped projects. |
| `./workbench status [--json]` | Inspect project ownership and actual TCP listeners. |
| `./workbench check-port PORT` | Check catalog reservations and live listeners before starting a service. |
| `./workbench context PROJECT_ID` | Print the project's path, launch command, owner and maintenance handoff. |
| `./workbench register --file FILE` | Add or replace a project from a JSON record; use `--file -` for stdin. |
| `./workbench lifecycle PROJECT_ID active` | Mark a catalog entry active. Also accepts `paused` or `archived`; does not stop services. |
| `./workbench skills [--provider codex\|claude] [--json]` | Discover installed agent skill files. |
| `./workbench package [--output FILE]` | Export a clean Go source ZIP. |
| `./workbench package --runtime-only [--output FILE]` | Export the executable for its build OS/architecture, setup files and licenses. |

Examples:

```sh
# Background engine without opening a browser.
./workbench start --port 8732 --no-browser

# Check reservations; inspect actual listeners and ownership.
./workbench check-port 3000
./workbench status --json

# Use a particular installation from any working folder.
/path/to/workbench --root /path/to/installation list --json

# Read a record before replacing it, then save a reviewed JSON file.
./workbench context my-project
./workbench register --file /path/to/project.json
./workbench lifecycle my-project paused

# Discover skills and create release packages.
./workbench skills --provider claude --json
./workbench package --output dist/Workbench-source.zip
./workbench package --runtime-only --output dist/Workbench-macos.zip
```

Successful commands exit 0; errors exit 1. `check-port` exits 2 if a port is reserved, occupied
or cannot be inspected. `status` reports process/listener evidence; the dashboard also checks
HTTP readiness and runner progress. The internal `runner` subcommand is used by the supervisor.

| Build command | Purpose |
| --- | --- |
| `make build` | Build the normal development executable. |
| `make run [PORT=8732] [NO_BROWSER=1]` | Build and start/reuse Workbench; opens the browser unless disabled. |
| `make serve [PORT=8732]` | Build and run the engine in the foreground on a free port; Ctrl+C stops it. |
| `make release` | Build the smaller release executable. |
| `make test` | Run Go tests with the race detector. |
| `make check` | Run race-enabled Go tests, vet and JavaScript syntax checks. |
| `make check-ui` | Run the jsdom UI suites using `NODE` and optional `JSDOM_MODULE`. |
| `make package` | Build the release executable and create both distribution ZIPs. |

## Repository layout

```text
cmd/workbench/          Go executable entry point
internal/
  app/                  CLI and engine lifecycle
  catalog/              Project records and atomic registry transactions
  inventory/            Process ownership and workspace snapshots
  discovery/            Read-only project and skill discovery
  launch/               Plans, runners, readiness and logs
  httpapi/              HTTP routes and local access controls
  distribution/         Source and runtime packaging
  fsutil/               Safe filesystem operations
  testutil/             Shared Go test fixtures
web/
  static/               Browser assets, fonts and vendored dependencies
  templates/            Server-rendered HTML fragments
tests/ui/               Frontend interaction and browser checks
scripts/                Developer tools and compatibility commands
legacy/python/          Previous Python implementation and its tests
docs/                   Setup, architecture, migration and design notes
```

Go tests live beside their packages. The root `workbench` binary, catalogs, `.runtime/` and
`dist/` are generated/local data and stay out of Git. See the [architecture guide](docs/architecture.md)
for package boundaries and the [setup guide](docs/setup.md) for installation.

Build the entry point with `go build -o workbench ./cmd/workbench`, or use the Make targets.
Existing `./workbench ...` commands and the double-click launcher continue to work.

## The everyday workflow

1. **Find projects.** Choose a folder with Browse, paste an absolute path, or select a suggested
   workspace root. Discovery recognises package.json dev/start/serve/preview scripts, Python
   launchers, Go executables and static index.html websites. Review a suggestion before adding it. A proposed
   framework port is a suggestion, not proof that a project uses it.
2. **Start a project.** Select Start, review the exact command and working folder, and confirm.
   If a registered web project has a launch command but no local URL or port, choose
   **Set up & start** on its card. Enter the website address and expected port once, then
   **Save & review start**. Saving opens the reviewed launch plan; only confirming Start
   executes the command. Output appears in Launch logs, followed by Open when readiness is verified.
   Sequential commands joined with `&&` are supported. Commands are passed as literal argument
   arrays; shell substitution, pipes, backgrounding, redirection and inline environment assignments
   are unsupported. Put advanced setup in a project script instead.
3. **Open when ready.** Workbench waits for the registered local URL to respond. The optional
   browser tab opened by your Start click navigates to the project once it is ready. Popup blocking
   is handled by keeping an Open action in the workspace.
4. **Resolve a problem.** Inspect the project for the recorded path, command, URL, owned listeners
   and settings. Launch logs show the last 160 lines from Workbench's most recent launch.
5. **Stop explicitly.** Inspect the target and confirm Stop service. Workbench verifies the PID's
   birth time, folder, listening addresses and project association again before SIGTERM. Unknown
   listeners and the current engine are protected. No force kill or automatic duplicate cleanup.

Missing runtimes, moved folders, stale launch plans, overlapping reservations, occupied ports and
nonzero exits have specific recovery messages. A duplicate Start does not create a second runner.
A launcher that detaches its server can finish while the project remains ready; actual services
and the HTTP response take precedence over the launcher's exit status.

## Your workspace

- **Board / list.** The board is a rearrangeable project workspace. Drag a card's handle to move it,
  or focus the handle and use arrow keys. Touch handles support moving a card to another card.
  The list is a compact alternative. Moving a card never changes its service state.
- **Favourites / recent.** Star projects for quick access. Projects opened from Workbench appear
  in Recently opened and in the resume strip. Stopped projects keep their place.
- **Dark / light.** Dark is the default. The theme toggle remembers your choice. Existing version-2
  preferences switch to the new dark theme once; favourites, recent opens and board order are kept.
- **Engine details.** Workbench itself appears in the header and Diagnostics. It does not count as
  one of your projects or occupy a project card. Click the engine indicator to inspect its record.
- **Diagnostics.** Inspect all project listeners or include unmatched processes. Port ownership is
  matched by resolved source folder; identical roots prefer matching expected ports. A process
  launched from an unrelated folder remains unverified even when its website responds.
- **Agent skills.** Codex, Claude, shared and project-local skills remain a secondary view. Discovery
  describes files on disk; it does not install or activate skills, or establish compatibility.

Favourites, recent opens, board order, layout and theme are stored in this browser under
`workbench.workspace.v2`. They are browser/port-specific. Shared project records live in
`registry.json`, independently of browser preferences. Browser storage failure does not discard
project records. API failures retain existing cards and report the stale connection.

## What status means

| State | Evidence / next action |
| --- | --- |
| Ready | A project-owned listener exists and its registered local URL returned HTTP 2xx or 3xx. Open it. |
| Running | A listener exists, but a local website readiness check is not applicable. |
| Starting | A recent Workbench launch is waiting for its local website. Inspect logs as needed. |
| Stopped | No associated listener exists. Start a registered web project to reopen it. |
| Launch failed | A Workbench launch step exited unsuccessfully. Read its logs and adjust settings. |
| Not ready | A service or launch exists but its website is unavailable or returned an error. |
| Ownership unverified | An unmatched listener occupies an expected port. Inspect it; ownership is not guessed. |
| Port conflict | Another registered project owns a listener on an expected port. |
| Unknown | Process inspection failed. It is not reported as stopped or healthy. |
| Installed / available | A desktop app, command-line tool, or reference exists. TCP listening is not required. |

Ready checks the registered page, not the full application's behaviour. Redirects are not followed
outside the local address. Health checks bypass proxy settings and use short timeouts. Paused and
archived are catalog lifecycle values, independent of whether a process is running.

## Discovery and local boundaries

Discovery is read-only, has a depth and folder-count limit, skips symlinks and dependency, build,
hidden, secret and agent settings trees, and reads only bounded named project files. It never runs
project code, imports suggestions automatically, or reads `.env` files. The macOS Browse action
opens a folder picker; other platforms can paste a path. A cancelled picker leaves the form usable.

The HTTP engine binds to loopback, checks Host/Origin and cross-site request metadata, uses a
per-run CSRF token for writes, and serves only named public assets. Launches use the current
server-owned project record and a reviewed plan fingerprint/revision, not client-supplied shell
commands. Only local website URLs are probed. Logs are capped into two segments per run, responses
are bounded, and common credential patterns are masked; masking cannot guarantee removal of every
possible secret an application prints. Project runners inherit the launcher's environment.

`.runtime/runs/` records launch progress; `.runtime/logs/` contains the output. Runs are detached,
so closing the browser or reloading the engine does not stop project services. Runner shutdown
forwards SIGTERM only to the child it created. Log history is retained locally.

## One catalog for humans and agents

The UI and CLI share the same registry, locking, validation, backup and atomic writes:

```sh
./workbench list --json
./workbench status --json
./workbench context branchroom
./workbench check-port 3000
./workbench skills --provider claude --json
./workbench register --file /path/to/project.json
./workbench lifecycle branchroom paused
```

CLI status inspects TCP ownership; the HTTP workspace adds readiness and runner state. A reservation
matters even when a project is stopped or archived. `check-port` exits 2 if a port is occupied,
reserved, or cannot be inspected. Registering an existing ID replaces that entry, so read it first.
UI saves use a revision to reject stale changes. `registry.previous.json` is the previous snapshot,
not an unlimited history; export durable backups from the sidebar.

Project fields: `id`, `name`, `description`, `kind` (web/desktop/cli/library), `path`, `url`, `ports`,
`lifecycle` (active/paused/archived), `owner`, `launch`, `maintenance`, `docs`, `updatedAt`.

The surrounding repository's AGENTS.md / CLAUDE.md instruct agents to check this catalog and live
ports. Global installation of those instructions remains a separate explicit action; discovering
an agent skill does not activate it. Native desktop app launch commands remain instructions to use
with the app's normal launcher; the supervised Start workflow applies to web projects.

## Share a clean copy

```sh
make build
./workbench package
```

`dist/Workbench-codebase.zip` includes the Go source, tests, CLI, launcher, frontend assets,
bundled htmx license and setup guide. Use `./workbench package --runtime-only` for
`dist/Workbench.zip`, containing the executable for the machine you built on and its setup
files. Build separately for each OS/architecture you want to distribute.

Both archives exclude personal project records, logs, runtime settings and credentials. The
runtime package needs no Go or Python installation. It is an unsigned portable application,
not a native installer.

## Verify from a source checkout

```sh
make check
JSDOM_MODULE=/absolute/path/to/jsdom node tests/ui/interface.cjs
JSDOM_MODULE=/absolute/path/to/jsdom node tests/ui/htmx.cjs
JSDOM_MODULE=/absolute/path/to/jsdom node tests/ui/workspace.cjs
```

Go tests use temporary catalogs and service fixtures. They exercise validation and concurrent
atomic writes, bounded discovery, linked skills, ownership matching, stale plans, identity-verified
stopping, readiness, duplicate launches, detached runner reattachment, log limits/redaction,
HTTP boundaries, embedded assets and clean archives. `make check` includes the race detector,
`go vet` and JavaScript syntax validation.

Production has no Go module or npm dependencies. htmx 2.0.11 is vendored with its upstream license;
jsdom is test-only and needs Node 20.19+, 22.12+ or 24+ (use a runtime compatible with the installed
jsdom version). The UI checks cover the dashboard, htmx log integration, first-run state, engine
separation, theme migration and preference restoration.

Legacy Python modules remain in this branch as a reference during migration;
`python3 -m unittest discover -s legacy/python/tests -v` checks the prior implementation. They are excluded
from Go distribution archives. `scripts/compat/start.py`, `scripts/compat/registry.py` and `scripts/compat/package.py` forward old command
invocations to Go.

The previous root-level Python paths have moved. Update any remaining external callers to
the Go CLI or the explicit `scripts/compat/` paths; see [compatibility commands](scripts/compat/README.md).

## Retiring the Python files

The compiled release already runs without Python. The `.py` files under `legacy/python`,
`scripts/compat` and the optional instruction installer are retained for the migration.
They can be removed once the Go rollout is verified; a full conversion of every frontend view
to htmx is a separate task.

Before removal, verify real process identity checks, project start/stop, engine reload and the
native folder picker in a normal desktop session. Update external automation and agent
instructions from `python3 registry.py ...` / `python3 start.py` to the Go CLI, and port or retire
the optional `scripts/install_agent_instructions.py` helper. Then remove the legacy modules, compatibility
wrappers and Python-only tests together, updating the setup/migration docs in the same change.

The optional Playwright browser check is still available. Automated full-browser screenshot
verification has not been established for this migration; functional tests do not establish
visual verification.
