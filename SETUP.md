# Workbench setup guide

Workbench is a local project workspace and launcher. It runs on your computer and opens in your
browser. You do not need a hosting account, API key, GitHub connection, or npm installation to run
Workbench itself.

## 1. Requirements

- **macOS:** Python 3.10 or newer. The usual `lsof` and `ps` utilities must be available.
- **Linux:** Python 3.10+, `lsof`, and `ps`. Use the terminal launcher. The native Browse folder
  picker is macOS-only; paste a folder path on Linux.
- A modern desktop browser.
- Your individual projects need their own runtimes and dependencies, such as Node/npm or Python
  packages. Workbench detects missing runtimes but does not install project dependencies for you.

Windows is not currently supported. Node is needed only if you want to run the optional UI tests.

Check Python with:

```sh
python3 --version
```

If Python is missing or older than 3.10, install a current Python release before continuing.

## 2. Extract and start

1. Extract **Workbench-codebase.zip** into a normal writable folder.
2. Open the extracted **Workbench** folder.
3. On macOS, double-click **Open Workbench.command**. Alternatively, open a terminal in that folder:

```sh
python3 start.py
```

The launcher starts or reuses this installation, prints its local address, and opens your browser.
On a fresh machine, the default address is:

**http://127.0.0.1:8730/**

To start without opening a browser tab:

```sh
python3 start.py --no-browser
```

The first run creates a fresh `registry.json` containing Workbench itself. The archive contains
the application source and tests; your existing personal project catalog and launch history are
not included.

### Using the already-running workspace on this laptop

The updated development workspace is **http://127.0.0.1:8731/**. Its old page on port 8730 forwards
there when refreshed. You can use that instance directly; extracting the ZIP does not require
starting another copy.

## 3. Add your first project

1. Choose **Find projects**.
2. Browse to a project/workspace folder, or paste its absolute path.
3. Choose **Find projects** in the dialog.
4. Select **Review & add** on a suggestion.
5. Confirm its name, folder, launch command, local website URL, and expected port; save it.

Discovery recognises `package.json` dev/start/serve/preview scripts, `start.py` / `server.py`
launchers, and static `index.html` websites. It checks a bounded set of folders and skips hidden,
dependency, build, secret and symlinked trees. Existing registered projects are skipped.

You can also choose **Add manually**. A typical project might use:

```text
Name: My local app
Project folder: /absolute/path/to/my-local-app
Launch command: npm run dev
Local website URL: http://127.0.0.1:5173/
Expected ports: 5173
```

Use the address your project actually serves. Suggested framework ports may need correction.

## 4. Start, open, inspect, stop

- **Start:** review the exact command and working folder, then confirm. Leave **Open the website
  when it’s ready** checked if you want Workbench to navigate the new tab automatically.
- **Ready / Open:** a project-owned listener exists and the registered local website responded.
  This checks availability, not every function inside your app.
- **Details:** inspect the URL, folder, launch command, ownership, ports, and settings.
- **Launch logs:** read recent output when startup fails or takes too long.
- **Stop:** open Details or Diagnostics, select Stop, review the process, and confirm.

Workbench checks reservations and live listeners before launching and guards against duplicate
starts. It does not stop unknown processes automatically. A paused or archived catalog entry can
still have a running process; lifecycle and process state are separate.

Commands run from the project folder. Sequential steps using `&&` are supported:

```text
npm run build && npm start
```

Shell substitutions, pipes, output redirection, background operators, and inline environment
assignments are not supported. Put complex setup in a project script and launch that script.
Desktop apps and reference items use their recorded normal launcher instructions.

## 5. Arrange your workspace

- Star a card to add it to **Favourites**.
- Projects you open from Workbench appear under **Recently opened**.
- In Board view, drag a handle to reorder cards. Focus a handle and use arrow keys as an alternative.
- Switch to List view for a compact scan.
- Use the theme toggle for light or dark mode.

These preferences are saved in this browser and are specific to its local address. Shared project
records are saved on disk in `registry.json`.

## 6. Common problems

| Problem | What to do |
| --- | --- |
| Port already in use | Inspect Diagnostics. Reuse the correct existing instance, or choose a free port. Do not blindly kill its listener. |
| Another Workbench installation owns the port | Open that installation, or configure a different local port using the steps below. |
| Folder moved | Open Project settings and update the absolute project folder. |
| Runtime missing | Install the runtime named in the error, then restart Workbench from a terminal with that runtime on PATH. |
| Project dependency missing | Follow that project’s setup instructions, then retry Start. |
| Not ready / wrong address | Check the launch output and update the project URL and expected port to match. |
| Ownership unverified | Inspect the listener. Workbench will not assume that every process using a project’s expected port belongs to it. |
| Browser blocks the launch tab | Wait for Ready and use the project’s Open button. |
| Browse is unavailable | Paste the folder path into the discovery form. |
| The engine is unavailable | Run `python3 start.py --no-browser` and open the printed address. Check `.runtime/server.log` if startup fails. |

### Choose a different Workbench port

Run these commands from the extracted Workbench folder. Pick a port that is not already occupied or
reserved. This example uses 8732:

```sh
python3 start.py --help
python3 registry.py check-port 8732
```

If the registry does not exist yet, run the launcher once to initialise it. An occupied port may
prevent that initial launch, but the new local registry will still have been created.

After checking that 8732 is free:

```sh
python3 -c 'import json; from pathlib import Path; Path(".runtime").mkdir(exist_ok=True); Path(".runtime/local.json").write_text(json.dumps({"port":8732})+"\n")'
python3 start.py
```

The launcher updates its own catalog address when this setting changes. A running older instance
is not terminated. Reuse it or shut it down explicitly from its own terminal when appropriate.

## 7. Data, backup, and updates

- `registry.json`: your shared project records. Use **Export project registry** for a backup.
- `registry.previous.json`: the preceding registry snapshot, not an unlimited backup history.
- `.runtime/local.json`: optional local port setting.
- `.runtime/runs/`: most recent runner state for each launched project.
- `.runtime/logs/`: launch output, bounded to two log segments per run.
- `.runtime/server.log`: Workbench engine output.

After replacing v2 backend source files, reopen the launcher. It reloads this installation in place
when needed and leaves detached project runners running. To stop a project, use its explicit Stop
control. Closing the browser does not stop the engine or your projects.

## 8. Source code and verification

```text
server.py / engine.py    Local HTTP engine and reload configuration
inventory.py            Shared registry, process ownership, skill discovery
discovery.py            Read-only project suggestions
supervisor.py           Launch plans, runner state, readiness, diagnoses, logs
runner.py               Detached sequential process runner
start.py                Portable bootstrap / launcher
registry.py             Agent and terminal CLI
public/                 HTML, CSS, JavaScript, SVG assets
tests/                  Backend, integration, UI, and optional browser checks
package.py              Creates clean source or runtime ZIPs
```

Backend tests require Python and the local process utilities:

```sh
python3 -m unittest discover -s tests -v
```

For the optional JavaScript interaction checks, install a test-only jsdom copy (Node 20.19+):

```sh
npm install --prefix .runtime/qa --no-audit --no-fund jsdom
JSDOM_MODULE="$PWD/.runtime/qa/node_modules/jsdom" node tests/interface.cjs
node --check public/app.js
```

The browser check uses Playwright/Chrome separately. Automated Chrome capture is blocked in the
current development environment; screenshot-based desktop/mobile verification remains a manual
step. Functional and live endpoint checks are described in README.md.

To rebuild a clean archive:

```sh
python3 package.py
```

This produces `dist/Workbench-codebase.zip`. Use `python3 package.py --runtime-only` for the smaller
`dist/Workbench.zip`, or `--output /absolute/path/name.zip` to choose a destination. Both variants
exclude personal registry records, runtime data, credentials, and browser preferences.
