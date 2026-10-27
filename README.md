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

With a compiled executable, start Workbench directly:

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

The Start command reuses a healthy Go instance of this installation and refuses an occupied port or
another runtime. After rebuilding, running Start again reloads a changed Go engine in place;
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
`.runtime/local.json` with the following content before using `./workbench start` without a port:

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

Run commands from the folder containing the executable. The CLI uses Cobra for commands,
help and argument validation. The global `--root` option can go before or after the subcommand
and selects the folder containing the catalog and `.runtime` data. Use `COMMAND --help` for
command-specific options; `./workbench completion --help` describes shell completion setup.

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
| `./workbench register --file FILE --revision N` | Save only if the catalog still has revision N. |
| `./workbench lifecycle PROJECT_ID active` | Mark a catalog entry active. Also accepts `paused` or `archived`; does not stop services. |
| `./workbench skills [--provider codex\|claude] [--json]` | Discover installed agent skill files. |
| `./workbench package [--output FILE]` | Export a clean Go source ZIP. |
| `./workbench package --runtime-only [--output FILE]` | Export the executable for its build OS/architecture, setup files and licenses. |
| `./workbench mcp [--allow-writes]` | Serve local MCP tools over stdio; read-only by default. |

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
  static/               Browser entry points and assets
    js/shared/          DOM, API, preferences, appearance and icon helpers
    js/projects/        Project board, details, editing and arrangement
    js/skills/          Skill catalog, editor state, views and actions
    js/workspace/       Navigation, discovery, services, launches and logs
    css/                Theme tokens, shared controls and feature styles
  templates/            Server-rendered HTML fragments
scripts/                Developer tools and compatibility commands
docs/                   Setup, architecture, migration and design notes
```

Go tests live beside their packages. The root `workbench` binary, catalogs, `.runtime/` and
`dist/` are generated/local data and stay out of Git. See the [architecture guide](docs/architecture.md)
for package boundaries and the [setup guide](docs/setup.md) for installation.

Build the entry point with `go build -o workbench ./cmd/workbench`, or use the Make targets.
Start from source with `make run`, or use `./workbench start` after building.

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
   birth time, process name, folder, listening addresses and project association again before
   SIGTERM. Registered web services can be stopped even if started from a terminal. On macOS,
   **Diagnostics → All listeners** also offers Stop for unregistered listeners owned by your
   user account with a known working folder. Their owner is checked again before stopping.
   The current engine, root-owned processes and other users' unregistered processes are protected.
   Stop affects every port served by that PID. No force kill or automatic duplicate cleanup.

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
- **Dialogs.** Click outside a panel, press Escape, or use its close button to dismiss it. The
  header stays visible while its contents scroll, and the workspace behind it stays locked.
  Unsaved skill drafts require an explicit keep/discard choice before closing.
- **Engine details.** Workbench itself appears in the header and Diagnostics. It does not count as
  one of your projects or occupy a project card. Click the engine indicator to inspect its record.
- **Diagnostics.** Inspect all project listeners or include unmatched processes. Port ownership is
  matched by resolved source folder; identical roots prefer matching expected ports. A process
  launched from an unrelated folder remains unverified even when its website responds. On macOS,
  eligible unregistered listeners have separate **Track project** and **Stop…** actions; stopping
  does not create a project record. Stop sends SIGTERM; a process may ignore it or be restarted
  by its supervisor, so refresh to confirm the listener exited.
- **Agent skills.** Codex, Claude, shared and project-local skills remain a secondary view. Discovery
  describes files on disk; it does not install or activate skills, or establish compatibility.

Favourites, recent opens, board order, layout and theme are stored in this browser under
`workbench.workspace.v2`. They are browser/port-specific. Shared project records live in
`registry.json`, independently of browser preferences. Browser storage failure does not discard
project records. API failures retain existing cards and report the stale connection.

## Read, copy and edit agent skills

Open **Agent skills**, then select a skill to read its Markdown preview or raw `SKILL.md`.
The file selector opens supporting text files and bundled references. Copy instructions or the
file path to the clipboard; **Copy & export** also offers a complete filtered ZIP and duplication
into a listed personal or project skill folder. Review the destination and folder name before
creating a copy. Existing folders are never merged or replaced.

Personal, shared and project instruction files can be edited through **Edit instructions →
Review changes → Save changes**. Drafts survive failed saves; closing with unsaved changes asks
whether to keep or discard the draft. A newer file revision causes a conflict instead of an
overwrite. Each successful save keeps the previous version under `.runtime/skill-backups/`.

Plugin-managed and built-in originals remain read-only. **Make editable copy** includes files
inside the skill folder and opens the new copy for editing. External references stay unchanged.
Copies exclude hidden/configuration files, credential files, dependencies and symbolic links;
the workspace lists exclusions and disables bundle copying when size/traversal limits are reached.
Raw HTML and unsafe URLs are disabled in Markdown previews, and previews do not load remote images.
These actions do not run a skill or automatically configure its activation in an agent.

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

Claude Code and Codex can connect through `./workbench mcp`, or use the CLI directly.
See the [agent integration guide](docs/agents.md) for client setup commands, tools, optional
revision-checked registration and how process ports are identified. MCP does not launch or
stop services; those actions retain the dashboard's reviewed confirmation flow.

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

`dist/Workbench-codebase.zip` includes the Go source, tests, CLI, frontend assets,
bundled htmx license and setup guide. Use `./workbench package --runtime-only` for
`dist/Workbench.zip`, containing the executable for the machine you built on and its setup
files. Build separately for each OS/architecture you want to distribute.

Both archives exclude personal project records, logs, runtime settings and credentials. The
runtime package needs no Go or Python installation. It is an unsigned portable application,
not a native installer.

## Verify from a source checkout

```sh
make check
```

Go tests use temporary catalogs and service fixtures. They exercise validation and concurrent
atomic writes, bounded discovery, linked skills, ownership matching, stale plans, identity-verified
stopping, readiness, duplicate launches, detached runner reattachment, log limits/redaction,
HTTP boundaries, embedded assets and clean archives. `make check` includes the race detector,
`go vet` and JavaScript syntax validation.

Browser JavaScript uses native ES modules; `app.js` connects the feature controllers and
initializes the page. CSS imports preserve the original cascade order. No frontend framework,
npm build or development server is required. Browsers load the modules directly. Frontend
interaction and visual checks are manual; the previous UI test harness has been removed.

The Go CLI uses Cobra and skill previews use Goldmark, with versions pinned in `go.mod` and
checksums in `go.sum`. The first
source build needs access to the Go module proxy or a populated module cache. Executable
packages still run without Go, Python or Node installed. There is no production npm dependency.
Go dependency licenses are bundled under `third_party/licenses` in both distribution formats.
htmx 2.0.11 is vendored with its upstream license.

## Retired migration code

The previous Python backend and its tests have been removed, along with the standalone UI test
harness and its runner. Go package tests and JavaScript syntax validation remain in `make check`.
Manual frontend QA should cover projects, discovery, launch/stop review, logs, themes, and skill
view/edit/copy flows after browser changes.

Optional Python forwarding commands remain under `scripts/compat`, and the agent instruction
installer remains under `scripts`; neither is required by the Go application or included in
executable archives. New automation should invoke `./workbench` directly; see
[compatibility commands](scripts/compat/README.md). Earlier implementation history remains in Git.
