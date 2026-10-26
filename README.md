# Workbench

Find your local projects, start them, and pick up where you left off.

Workbench is a personal local workspace for web projects, installed apps, references and agent
skills. It keeps the project catalog on disk, checks real local services, and provides an
explicit Start → Ready → Open workflow for registered web projects.

## Open

On macOS, double-click **Open Workbench.command**. From a terminal:

```sh
python3 start.py
```

Python 3.10+ is required. macOS is the primary platform; process inspection uses `lsof` and `ps`.
Linux can use the Python launcher when those utilities are installed. Windows is not supported.
A project's own dependencies and runtime still need to be installed.

The default address for a fresh installation is **http://127.0.0.1:8730/**. This workspace uses
**http://127.0.0.1:8731/**, selected by `.runtime/local.json`, because an older engine remains on
8730. Refreshing its old page forwards to the updated workspace. Port 8730 remains reserved;
do not automatically stop its listener. The launcher always prints the actual current address.

The launcher reuses a healthy instance and refuses an occupied port belonging to another app.
For a v2 engine, reopening the launcher after backend source changes requests an in-place engine
reload. Detached project runners keep their processes. It does not kill another process or install
a login item. Corrupt catalogs are reported rather than reset. A genuinely new installation gets
its own empty catalog containing only Workbench.

## The everyday workflow

1. **Find projects.** Choose a folder with Browse, paste an absolute path, or select a suggested
   workspace root. Discovery recognises package.json dev/start/serve/preview scripts, Python
   launchers, and static index.html websites. Review a suggestion before adding it. A proposed
   framework port is a suggestion, not proof that a project uses it.
2. **Start a project.** Select Start, review the exact command and working folder, and confirm.
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
- **Light / dark.** The theme toggle changes the workspace without changing project data.
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
python3 registry.py list --json
python3 registry.py status --json
python3 registry.py context branchroom
python3 registry.py check-port 3000
python3 registry.py skills --provider claude --json
python3 registry.py register --file /path/to/project.json
python3 registry.py lifecycle branchroom paused
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
python3 package.py
```

`dist/Workbench-codebase.zip` includes the full source, tests, CLI, launcher, public assets, and
SETUP.md. Use `python3 package.py --runtime-only` for the smaller `dist/Workbench.zip`.
It excludes your registry, local port override, logs, preferences, personal paths from project
records, credentials and runtime data. Extract it, install Python if needed, then open
**Open Workbench.command** on macOS or run **python3 start.py**. It creates a fresh local catalog.
The archive is a portable Python application, not a signed native installer.

## Verify from a source checkout

```sh
python3 -m unittest discover -s tests -v
node --check public/app.js
JSDOM_MODULE=/absolute/path/to/jsdom node tests/interface.cjs
```

Production has no npm dependency. jsdom is used only by the interaction checks. Backend checks cover
bounded discovery, command parsing, unchanged-plan/reservation guards, process identity, actual
runner startup/readiness/logging/duplicate prevention and cleanup, and HTTP boundaries. Interaction
checks cover board and keyboard arrangement, favourites, both layouts and themes, discovery/import,
launch preview, explicit start/stop, logs, filters, safe rendering and failed refresh recovery.

The optional `tests/browser.cjs` uses Playwright and a local Chrome installation. Browser inspection
in this Codex environment still exits before Chrome can render, even with the permitted retry.
UI interactions and live endpoints are verified; real desktop/tablet/phone screenshots remain
unverified here. The earlier stopped-card styling was user-checked before this broader redesign.
