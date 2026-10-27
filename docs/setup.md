# Workbench setup guide

Workbench runs locally and opens in your browser. It needs no account, API key or hosting service.

## Requirements

- macOS or Linux with `lsof` and `ps` available. Windows is not supported.
- A modern browser.
- Your projects still need their own runtimes and dependencies.
- To build source: Go 1.24+ and network access for the first Go module download (or a populated
  module cache). A prebuilt executable needs only the OS utilities listed above and your projects' runtimes.

## Install through npm

Once a release is published, use Node 22.14+ and npm:

```sh
npx @priyadharshan933/workbench@latest
# Or:
npm install -g @priyadharshan933/workbench
workbench start
```

With no arguments, the npm launcher starts Workbench and opens the dashboard. Use
`workbench start --no-browser` in a headless session. Optional dependencies must be enabled so
npm installs the native runtime for macOS/Linux ARM64 or x64. No Go installation is required.
Linux browser opening uses `xdg-open`; port/process inspection requires `lsof` and `ps`.

The installed catalog and executable live in `~/Library/Application Support/Workbench` on
macOS, or `$XDG_DATA_HOME/workbench` (default `~/.local/share/workbench`) on Linux. Set
`WORKBENCH_HOME` to choose another persistent directory. `--root` still overrides the catalog
directory for an invocation. npm installs use a separate catalog from source checkouts;
records are not copied automatically. Updates preserve the installed catalog and logs.

Use `workbench --version` to check the installed release. Native downloads and publishing
instructions are documented in [releasing.md](releasing.md).

## Build and open

From a source checkout:

```sh
make run
```

This builds and starts or reuses Workbench in the background and opens your browser.
Use `make run PORT=8732 NO_BROWSER=1` to select a port and skip the browser. Without `PORT`,
the Start command uses `.runtime/local.json`, or 8730 on a fresh installation. For a foreground
engine on a free port, use `make serve PORT=8732`; Ctrl+C stops that engine.

From an executable ZIP, extract into a writable folder and run `./workbench start` from that
folder. Use `./workbench start --no-browser` to print the address without
opening a browser. `./workbench serve` runs the engine in the foreground.

The first launch creates `registry.json` with Workbench's own record. Existing catalogs use
the same version-1 JSON format and are validated, never reset. Browser preferences use the
same `workbench.workspace.v2` key and remain specific to the browser and port.
The redesigned dashboard defaults to dark. Earlier saved themes migrate to dark once; later
theme selections are remembered. Workbench's engine is shown in the header and Diagnostics,
separately from your project cards.

## Choose a port and reuse an instance

A fresh installation defaults to **http://127.0.0.1:8730/**. Inspect the shared catalog and live
listeners first. Once this installation has a catalog, its own CLI can inspect reservations:

```sh
./workbench status --json
./workbench check-port 8732
```

`check-port` exits 2 for reservations, listeners or inspection failures. To configure another
port, create `.runtime/local.json` containing `{"port":8732}` after verifying availability.
`./workbench start --port 8732` is also available. No existing service is stopped automatically.

Start reuses only a healthy Go v3 engine from this installation. An incompatible service or
another installation on the port causes an actionable error; choose another free port or use
that existing workspace. Rebuild the binary and rerun Start to reload an older Go build from
this installation. Detached project runners continue across engine reloads.

The `--root` option can go before or after the subcommand and selects the installation/catalog directory:

```sh
/path/to/workbench --root /path/to/workspace list --json
```

## Everyday use

Choose **Find projects**, select a folder, then review each suggestion before adding it.
Discovery recognizes package scripts, Python launchers, Go executables and static websites.
Go suggestions require review of the executable command and website port. Discovery never
executes project code or automatically registers suggestions.

Select **Start**, review the command and folder, and confirm. Commands run as literal arguments;
`&&` separates up to six steps. Pipes, redirects, substitutions and inline environment assignments
need a project script. Python projects still need an installed Python interpreter.

If the project has a command but is missing its website URL or expected port, its card offers
**Set up & start**. Fill those details once and choose **Save & review start**. Workbench saves
the configuration and shows the launch plan for confirmation. It does not execute on save.

Workbench opens a website when its project-owned listener and local HTTP response indicate
readiness. Use **Details**, **Diagnostics** and **Launch logs** to investigate failures.
Stopping requires an explicit confirmation and a fresh process identity check. Registered web
projects can be stopped even when they were started outside Workbench. On macOS, open
**Diagnostics → All listeners** to also stop unregistered listeners owned by your user account
with a known working folder. **Track project** remains a separate action. Review the PID, folder
and every listed port, then confirm **Stop service**. Workbench checks the owner again and sends
SIGTERM to that PID only; it does not force-kill a process or its process group. Refresh to verify
the listener exited. Root-owned/other-user unregistered processes and the current engine are
protected. Closing the browser does not stop projects.

Favourites, recent opens, board order, layout and theme are stored in your browser. Project
records are on disk and shared by the dashboard and CLI. **Export project registry** makes a
portable catalog backup; `registry.previous.json` contains only the preceding write.

## Skill files

Select **Agent skills** and open a skill to preview its instructions, read raw Markdown, follow
bundled references, and copy instructions or paths. **Copy & export** downloads a filtered ZIP
or creates a reviewed copy in a listed personal/project destination. The source is preserved.

For writable personal or project instructions, choose **Edit instructions**, then **Review
changes** and **Save changes**. Newer disk edits are rejected; your draft stays available to copy.
Backups are kept in `.runtime/skill-backups/`. Plugin-managed originals offer **Make editable
copy**. Supporting files are viewable/copyable; the editor changes `SKILL.md` only.

Bundle limits are 400 files, 16 MB total, 4 MB per supporting file and 10 directory levels.
Instruction editing and text preview are limited to 500 KB. Exclusions appear before copying;
external references are preserved and activation remains an agent-side decision.

## CLI

For the compiled release workflow, all command options and exit codes, see the
[README CLI reference](../README.md#cli-reference). `make release` builds the smaller release
executable; `make package` builds it and creates both source and executable archives.
Claude Code and Codex can use a local stdio MCP connection; see [agents.md](agents.md).

```sh
./workbench list --json
./workbench status --json
./workbench check-port 3000
./workbench context my-project
./workbench register --file /path/to/project.json
./workbench lifecycle my-project paused
./workbench skills --provider claude --json
```

## Runtime data and troubleshooting

- `registry.json`: durable project records; preserve it across updates.
- `registry.previous.json`: previous catalog snapshot.
- `.runtime/local.json`: optional engine port.
- `.runtime/server.log`: engine launch/reload errors.
- `.runtime/runs/`: detached runner state; `.runtime/logs/`: bounded project output.
- `.runtime/skill-backups/`: previous instruction files from explicit skill saves.
- `.runtime/skill-locks/`: coordination for concurrent skill edits by this installation.

An occupied port never triggers automatic cleanup. Missing runtimes or dependencies need to be
installed for the affected project. Update moved paths, incorrect URLs and ports in project
settings. If process inspection is unavailable, retry from a normal desktop terminal.

The Go executable embeds its frontend. Rebuild after changing Go or public assets. Keep the
executable named `workbench` beside its writable catalog directory, or use `--root` explicitly.

## Verify and distribute

```sh
make check
make package
```

Node is used only for JavaScript syntax checks in `make check`; no frontend build is required.
Go tests use isolated temporary fixtures; they do not stop catalog services. Browser interaction
and visual QA are manual.

`dist/Workbench-codebase.zip` contains Go source and tests. `dist/Workbench.zip` contains the
current platform executable, setup files and htmx/Inter licenses. Neither contains your
catalog, runtime settings, launch logs or credentials. Build each platform separately, for example:

```sh
GOOS=darwin GOARCH=arm64 go build -trimpath -o dist/darwin-arm64/workbench ./cmd/workbench
GOOS=darwin GOARCH=amd64 go build -trimpath -o dist/darwin-amd64/workbench ./cmd/workbench
GOOS=linux GOARCH=amd64 go build -trimpath -o dist/linux-amd64/workbench ./cmd/workbench
```

Executable packages are unsigned. Preserve executable permissions when transferring them.

## Source organization

The executable is built from `./cmd/workbench`. Use Make from the repository root; running
`go build .` at the root is no longer applicable. Go package tests sit under `internal` and
static sources under `web/static`. See [architecture.md](architecture.md).
