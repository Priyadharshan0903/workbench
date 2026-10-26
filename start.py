#!/usr/bin/env python3
"""Start one Workbench instance or reuse it. No login item is installed."""
import argparse
import json
from pathlib import Path
import socket
import subprocess
import sys
import time
from urllib.request import build_opener, ProxyHandler
import webbrowser
import engine

ROOT = Path(__file__).resolve().parent
PORT = engine.port()
URL = f'http://127.0.0.1:{PORT}'


def initialize():
    import inventory
    if not inventory.REGISTRY.exists():
        import getpass
        project = {'id': 'workbench', 'name': 'Workbench', 'description': 'Your local project workspace and launcher.',
                   'kind': 'web', 'path': str(ROOT), 'url': URL + '/', 'ports': [PORT],
                   'lifecycle': 'active', 'owner': getpass.getuser(), 'launch': 'python3 start.py',
                   'maintenance': 'Start reuses the existing instance. Project launch logs are in .runtime/logs.',
                   'docs': 'README.md', 'updatedAt': inventory.now()}
        inventory.atomic_write(inventory.REGISTRY, json.dumps({'version': 1, 'revision': 1, 'projects': [project]}, indent=2) + '\n')
    data = inventory.read_registry()
    own = next((p for p in data['projects'] if p['id'] == 'workbench' and p.get('path') == str(ROOT)), None)
    if own and own['url'].rstrip('/') != URL:
        inventory.save_project(dict(own, url=URL + '/', ports=[PORT]), data['revision'])


def running():
    try:
        with build_opener(ProxyHandler({})).open(URL + '/api/health', timeout=1) as response:
            return json.load(response).get('app') == 'workbench'
    except (OSError, ValueError):
        return False


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--no-browser', action='store_true')
    args = parser.parse_args()
    initialize()
    if running():
        from urllib.request import Request
        opener = build_opener(ProxyHandler({}))
        with opener.open(URL + '/api/health', timeout=2) as response:
            health = json.load(response)
        if health.get('version') != 2:
            raise SystemExit('An older Workbench is already using this port. Use its existing workspace or choose another port in .runtime/local.json. No process was stopped.')
        if health.get('sourceRoot') and Path(health['sourceRoot']).resolve() != ROOT:
            raise SystemExit('Another Workbench installation is already using this port. Open that workspace, or choose another port in .runtime/local.json. No process was stopped.')
        if health.get('version') == 2 and health.get('sourceStamp') != engine.source_stamp():
            with opener.open(URL + '/api/state', timeout=20) as response:
                token = json.load(response)['csrfToken']
            request = Request(URL + '/api/reload', data=b'{}', headers={
                'Content-Type': 'application/json', 'X-Workbench-Token': token}, method='POST')
            with opener.open(request, timeout=5) as response:
                json.load(response)
            for _ in range(60):
                time.sleep(.1)
                try:
                    with opener.open(URL + '/api/health', timeout=1) as response:
                        if json.load(response).get('sourceStamp') == engine.source_stamp():
                            break
                except (OSError, ValueError):
                    pass
            else:
                raise SystemExit('The engine could not reload. See .runtime/server.log for details.')
    if not running():
        with socket.socket() as probe:
            if probe.connect_ex(('127.0.0.1', PORT)) == 0:
                raise SystemExit(f'Port {PORT} belongs to another service. Inspect it before starting Workbench.')
        runtime = ROOT / '.runtime'
        runtime.mkdir(exist_ok=True)
        with (runtime / 'server.log').open('a') as log:
            process = subprocess.Popen([sys.executable, str(ROOT / 'server.py')], cwd=ROOT,
                                       stdin=subprocess.DEVNULL, stdout=log, stderr=log,
                                       start_new_session=True)
        (runtime / 'server.pid').write_text(str(process.pid))
        for _ in range(50):
            if running():
                break
            if process.poll() is not None:
                raise SystemExit('Workbench could not start. See home-dashboard/.runtime/server.log.')
            time.sleep(.1)
        else:
            raise SystemExit('Workbench did not become ready. See home-dashboard/.runtime/server.log.')
    print('Workbench is ready: ' + URL)
    if not args.no_browser:
        webbrowser.open(URL)


if __name__ == '__main__':
    main()
