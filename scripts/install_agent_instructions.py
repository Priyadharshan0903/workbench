#!/usr/bin/env python3
"""Preview or install a small Workbench reference in Codex and Claude user instructions."""
import argparse
import datetime as dt
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[1]
START = '<!-- workbench-inventory:start -->'
END = '<!-- workbench-inventory:end -->'


def block():
    return f'''{START}
## Shared local tool inventory (Workbench)

Before creating a tool or starting a local server, consult `{ROOT / 'registry.json'}`.
Inspect actual listeners with `"{ROOT / 'workbench'}" status --json` and
check reservations with `"{ROOT / 'workbench'}" check-port PORT`.
Reuse healthy instances. Keep the shared catalog current when creating or changing tools:
`"{ROOT / 'workbench'}" register --file /path/to/project.json`.
Record purpose, source path, URL, expected ports, owner, launch command, docs, lifecycle,
and maintenance notes. Read `{ROOT / 'README.md'}` for schema and scope.
Human dashboard: http://127.0.0.1:8730/ ; start/reuse with
`"{ROOT / 'workbench'}" start --no-browser` when the user needs it.
Codex and Claude skills are discoverable using `"{ROOT / 'workbench'}" skills --json`.
Discovery does not activate skills. Do not kill duplicate or unknown services automatically.
{END}'''


def updated(text):
    if START in text or END in text:
        if text.count(START) != 1 or text.count(END) != 1 or text.index(START) > text.index(END):
            raise ValueError('Existing Workbench block is malformed; leaving the file unchanged.')
        before, rest = text.split(START, 1)
        _, after = rest.split(END, 1)
        return before + block() + after
    return text.rstrip() + ('\n\n' if text.strip() else '') + block() + '\n'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    targets = [Path.home() / '.codex/AGENTS.md', Path.home() / '.claude/CLAUDE.md']
    # Prepare both updates before making any change. Refuse symbolic instruction files.
    changes = []
    for path in targets:
        if path.is_symlink():
            raise ValueError(f'Instruction file is a symlink: {path}')
        current = path.read_text() if path.exists() else ''
        changes.append((path, current, updated(current)))
    if not args.apply:
        for path, _, _ in changes:
            print(f'Add/update managed Workbench block in {path}')
        print('\nExisting instructions will be preserved. Exact block:\n\n' + block())
        return
    stamp = dt.datetime.now().strftime('%Y%m%d-%H%M%S-%f')
    for path, current, content in changes:
        if current == content:
            print(f'Already current: {path}')
            continue
        path.parent.mkdir(parents=True, exist_ok=True)
        if path.exists():
            shutil.copy2(path, path.with_name(path.name + '.before-workbench-' + stamp))
        path.write_text(content)
        print(f'Updated: {path}')


if __name__ == '__main__':
    main()
