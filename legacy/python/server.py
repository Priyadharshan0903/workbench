#!/usr/bin/env python3
"""Workbench: loopback-only registry, skill discovery, and explicit service cleanup."""
import argparse
import hashlib
import hmac
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import secrets
import signal
import subprocess
import threading
import time
from urllib.parse import urlparse, parse_qs

import inventory
import discovery
import supervisor
import engine

SOURCE_ROOT = Path(__file__).resolve().parent
ROOT = SOURCE_ROOT.parents[1]
CSRF = secrets.token_urlsafe(32)
SIGNING_KEY = secrets.token_bytes(32)
SKILLS_LOCK = threading.Lock()
SKILLS_CACHE = None
BOOT_STAMP = engine.source_stamp()


def stop_token(service):
    started = inventory.run([inventory.PS, '-p', str(service['pid']), '-o', 'lstart=']).strip()
    if not started:
        return None
    identity = json.dumps([service['pid'], started, service['cwd'], service['endpoints']], sort_keys=True)
    return hmac.new(SIGNING_KEY, identity.encode(), hashlib.sha256).hexdigest()


def state():
    result = supervisor.enrich(inventory.snapshot())
    result['features'] = {'launch': True, 'discovery': True, 'logs': True, 'readiness': True}
    result['discoveryRoots'] = sorted({str(Path(p['path']).parent) for p in result.get('projects', [])
                                      if p.get('path') and not p['path'].endswith('.app') and Path(p['path']).is_dir()})[:8]
    result['controlWarnings'] = []
    for service in result['services']:
        if service['canStop']:
            try:
                service['stopToken'] = stop_token(service)
                service['canStop'] = bool(service['stopToken'])
            except (OSError, subprocess.SubprocessError) as exc:
                service['canStop'] = False
                service['stopUnavailable'] = 'Cannot verify process identity: ' + str(exc)
                warning = 'Stop controls are unavailable because the environment blocks process identity checks. Open Workbench from its launcher in a normal desktop session to retry.'
                if warning not in result['controlWarnings']:
                    result['controlWarnings'].append(warning)
    result['csrfToken'] = CSRF
    return result


def stop_service(pid, token):
    if type(pid) is not int or pid <= 1 or not isinstance(token, str):
        raise ValueError('Choose a currently listed service.')
    current = inventory.snapshot()
    if current['inspectionError']:
        raise ValueError('Process inspection is unavailable; no stop was attempted.')
    service = next((s for s in current['services'] if s['pid'] == pid), None)
    if not service or not service['canStop']:
        raise ValueError('This process is no longer a stoppable registered web service. Refresh the list.')
    expected = stop_token(service)
    if not expected or not hmac.compare_digest(expected, token):
        raise ValueError('The process changed since it was listed. Refresh and inspect it again.')
    os.kill(pid, signal.SIGTERM)
    return {'ok': True, 'message': f'Sent a graceful stop request to PID {pid}. Refresh to verify it exited.'}


def skills(force=False):
    global SKILLS_CACHE
    with SKILLS_LOCK:
        if not force and SKILLS_CACHE and time.monotonic() - SKILLS_CACHE[0] < 30:
            return SKILLS_CACHE[1]
        result = inventory.discover_skills(inventory.read_registry()['projects'])
        SKILLS_CACHE = (time.monotonic(), result)
        return result


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        # Request bodies and local file contents are never logged.
        print(f'{self.address_string()} {fmt % args}', flush=True)

    def allowed(self):
        hosts = {f'127.0.0.1:{self.server.server_port}', f'localhost:{self.server.server_port}'}
        return (self.headers.get('Host') in hosts
                and self.headers.get('Origin', f'http://127.0.0.1:{self.server.server_port}') in {f'http://{h}' for h in hosts}
                and self.headers.get('Sec-Fetch-Site') != 'cross-site')

    def reply(self, status, data, mime='application/json; charset=utf-8'):
        body = json.dumps(data, ensure_ascii=False).encode() if isinstance(data, dict) else data
        self.send_response(status)
        self.send_header('Content-Type', mime)
        self.send_header('Content-Length', str(len(body)))
        self.send_header('Cache-Control', 'no-store')
        self.send_header('X-Content-Type-Options', 'nosniff')
        self.send_header('Referrer-Policy', 'no-referrer')
        self.send_header('Content-Security-Policy', "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if not self.allowed():
            return self.reply(403, {'error': 'Local same-origin access only.'})
        url = urlparse(self.path)
        try:
            if url.path == '/api/health':
                return self.reply(200, {'ok': True, 'app': 'workbench', 'version': 2, 'sourceStamp': BOOT_STAMP, 'sourceRoot': str(ROOT)})
            if url.path == '/api/state':
                return self.reply(200, state())
            if url.path == '/api/registry':
                return self.reply(200, inventory.read_registry())
            if url.path == '/api/skills':
                return self.reply(200, skills(parse_qs(url.query).get('refresh') == ['1']))
            if url.path == '/api/launch-plan':
                return self.reply(200, supervisor.launch_plan(parse_qs(url.query).get('id', [''])[0]))
            if url.path == '/api/logs':
                return self.reply(200, supervisor.logs(parse_qs(url.query).get('id', [''])[0]))
            files = {'/': ('index.html', 'text/html; charset=utf-8'),
                     '/app.js': ('app.js', 'text/javascript; charset=utf-8'),
                     '/styles.css': ('styles.css', 'text/css; charset=utf-8'),
                     '/board-grid.svg': ('board-grid.svg', 'image/svg+xml'),
                     '/favicon.svg': ('favicon.svg', 'image/svg+xml')}
            if url.path not in files:
                return self.reply(404, {'error': 'Not found.'})
            name, mime = files[url.path]
            return self.reply(200, (ROOT / 'web' / 'static' / name).read_bytes(), mime)
        except (ValueError, KeyError, OSError, subprocess.SubprocessError) as exc:
            return self.reply(500, {'error': str(exc)})

    def do_POST(self):
        if not self.allowed() or not hmac.compare_digest(self.headers.get('X-Workbench-Token', ''), CSRF):
            return self.reply(403, {'error': 'Refresh this local dashboard before making changes.'})
        try:
            size = int(self.headers.get('Content-Length', '0'))
            if size < 1 or size > 32_768 or self.headers.get_content_type() != 'application/json':
                return self.reply(400, {'error': 'A JSON request up to 32 KB is required.'})
            data = json.loads(self.rfile.read(size))
            if not isinstance(data, dict):
                raise ValueError('A JSON object is required.')
            if self.path == '/api/project':
                if type(data.get('revision')) is not int:
                    raise ValueError('Refresh to get the current registry revision.')
                inventory.save_project(data.get('project'), data['revision'])
                return self.reply(200, {'ok': True})
            if self.path == '/api/stop':
                return self.reply(200, stop_service(data.get('pid'), data.get('stopToken')))
            if self.path == '/api/start':
                return self.reply(200, supervisor.start_project(data.get('id'), data.get('fingerprint'), data.get('revision')))
            if self.path == '/api/discover':
                path = data.get('path')
                if not isinstance(path, str) or len(path) > 2000:
                    raise ValueError('Enter a project or workspace folder.')
                return self.reply(200, discovery.discover(path, inventory.read_registry()['projects']))
            if self.path == '/api/choose-folder':
                if __import__('sys').platform != 'darwin':
                    raise ValueError('Paste an absolute folder path to discover projects on this platform.')
                result = subprocess.run(['osascript', '-e', 'POSIX path of (choose folder with prompt "Choose a project or workspace folder for Workbench")'],
                                        capture_output=True, text=True, timeout=120)
                if result.returncode:
                    raise ValueError('Folder selection was cancelled or unavailable. You can paste a folder path instead.')
                return self.reply(200, {'path': result.stdout.strip()})
            if self.path == '/api/reload':
                self.reply(200, {'ok': True, 'message': 'Reloading this Workbench engine. Project runners stay active.'})
                threading.Thread(target=engine.reload_current, daemon=True).start()
                return
            return self.reply(404, {'error': 'Not found.'})
        except (ValueError, KeyError, OSError, subprocess.SubprocessError) as exc:
            return self.reply(400, {'error': str(exc)})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--port', type=int, default=engine.port())
    args = parser.parse_args()
    inventory.read_registry()  # Corrupt state must not be silently replaced.
    server = ThreadingHTTPServer(('127.0.0.1', args.port), Handler)
    print(f'Workbench is running at http://127.0.0.1:{server.server_port}', flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
