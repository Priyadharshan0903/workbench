#!/usr/bin/env python3
"""Create a portable source ZIP without personal projects, preferences, logs or credentials."""
from pathlib import Path
import zipfile
import argparse

ROOT = Path(__file__).resolve().parent
FILES = ['server.py', 'inventory.py', 'registry.py', 'start.py', 'discovery.py', 'supervisor.py',
         'runner.py', 'engine.py', 'Open Workbench.command', 'README.md', 'DESIGN.md', 'SETUP.md']


def package(output=None, source=True):
    output = Path(output) if output else ROOT / ('dist/Workbench-codebase.zip' if source else 'dist/Workbench.zip')
    output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as archive:
        for name in FILES:
            archive.write(ROOT / name, 'Workbench/' + name)
        for file in (ROOT / 'public').iterdir():
            if file.is_file() and file.suffix in {'.html', '.js', '.css', '.svg'}:
                archive.write(file, 'Workbench/public/' + file.name)
        if source:
            for name in ('package.py', 'install_agent_instructions.py'):
                archive.write(ROOT / name, 'Workbench/' + name)
            for file in (ROOT / 'tests').iterdir():
                if file.is_file() and file.suffix in {'.py', '.cjs'}:
                    archive.write(file, 'Workbench/tests/' + file.name)
        archive.writestr('Workbench/.gitignore', '.runtime/\ndist/\n.taste-skill/\n__pycache__/\n*.pyc\nregistry.json\nregistry.json.lock\nregistry.previous.json\n')
        archive.writestr('Workbench/AGENTS.md', '''# Workbench source

Read README.md and SETUP.md first. Production uses Python standard library only.
Before starting a server, inspect the local registry and live reservations with
`python3 registry.py status --json` and `python3 registry.py check-port PORT`.
Reuse a healthy instance of this installation. Do not stop unknown or duplicate
processes automatically. Use the CLI to update registry records atomically.
Project launch requires an explicit reviewed plan; stop requires a fresh process
identity check and human confirmation. Discovery never executes project code.
Never read secret files. Keep registry.json, browser preferences, launch logs and
runtime settings out of source archives. Tests use isolated temporary fixtures.
''')
    return output


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime-only', action='store_true')
    parser.add_argument('--output')
    args = parser.parse_args()
    print(package(args.output, source=not args.runtime_only))
