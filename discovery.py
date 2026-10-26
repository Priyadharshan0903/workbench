"""Bounded, read-only project discovery. No project configuration is executed."""
import getpass
import hashlib
import json
from pathlib import Path
import re
import shlex

SKIP = {'.git', 'node_modules', 'vendor', '.venv', 'venv', 'dist', 'build', '__pycache__',
        '.ssh', '.aws', '.gnupg', '.azure', '.config', '.codex', '.claude', '.cache', '.Trash', '.env'}


def readable(path, limit=65_536):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > limit:
        return ''
    return path.read_text(errors='replace')


def inspect_project(root, existing=()):
    root = Path(root).resolve()
    if any(p['path'] and Path(p['path']).resolve() == root for p in existing):
        return None
    manifest, command, kind, name, description, port = '', '', 'web', root.name, '', None
    reasons = []
    package = readable(root / 'package.json')
    if package:
        try:
            data = json.loads(package)
        except ValueError:
            return None
        scripts = data.get('scripts') or {}
        if not isinstance(scripts, dict):
            return None
        script = next((s for s in ('dev', 'start', 'serve', 'preview') if isinstance(scripts.get(s), str)), None)
        if not script:
            return None
        manager = 'pnpm' if (root / 'pnpm-lock.yaml').is_file() else 'yarn' if (root / 'yarn.lock').is_file() else 'bun' if (root / 'bun.lock').is_file() or (root / 'bun.lockb').is_file() else 'npm'
        command = f'{manager} run {script}'
        source = scripts[script]
        name = str(data.get('name') or root.name).split('/')[-1].replace('-', ' ').title()
        description = str(data.get('description') or '')[:4000]
        match = re.search(r'(?:--port[=\s]+|(?:^|\s)-p\s+|PORT=)(\d{2,5})', source)
        if match:
            port = int(match[1])
            reasons.append('Port found in the package script')
        else:
            dependencies = {**(data.get('dependencies') or {}), **(data.get('devDependencies') or {})}
            port = 5173 if 'vite' in dependencies else 3000 if 'next' in dependencies else 4321 if 'astro' in dependencies else None
            if port:
                reasons.append('Framework default port; review before adding')
        manifest = 'package.json'
        reasons.insert(0, f'{script} script found in package.json')
    elif (root / 'start.py').is_file() or (root / 'server.py').is_file():
        script = 'start.py' if (root / 'start.py').is_file() else 'server.py'
        source = readable(root / script)
        command = f'python3 {shlex.quote(script)}'
        if '--no-browser' in source:
            command += ' --no-browser'
        match = re.search(r'https?://(?:127\.0\.0\.1|localhost):(\d{2,5})', source)
        if not match:
            match = re.search(r"['\"]--port['\"].{0,80}?default\s*=\s*(\d{2,5})", source)
        if match:
            port = int(match[1])
            reasons.append('Port found in the Python launcher')
        manifest = script
        reasons.insert(0, f'{script} launcher found')
    elif (root / 'index.html').is_file():
        # Pick a suggested port without taking another catalog entry's reservation.
        reserved = {n for p in existing for n in p.get('ports', [])}
        port = next((n for n in range(8800, 8900) if n not in reserved), None)
        if port is None:
            return None
        command = f'python3 -m http.server {port} --bind 127.0.0.1'
        title = re.search(r'<title[^>]*>([^<]{1,160})</title>', readable(root / 'index.html'), re.I)
        name = title.group(1).strip() if title else name
        manifest = 'index.html'
        reasons = ['Static website found', 'Suggested local port; checked again before launch']
    else:
        return None
    if port is not None and not 1 <= port <= 65535:
        port = None
    slug = re.sub(r'[^a-z0-9]+', '-', root.name.lower()).strip('-')[:48] or 'project'
    if any(p['id'] == slug for p in existing):
        slug += '-' + hashlib.sha256(str(root).encode()).hexdigest()[:7]
    docs = '; '.join(n for n in ('README.md', 'AGENTS.md', 'CLAUDE.md') if (root / n).is_file())
    return {'project': {'id': slug, 'name': name[:160], 'description': description or f'Local project in {root.name}.',
            'kind': kind, 'path': str(root), 'url': f'http://127.0.0.1:{port}/' if port else '',
            'ports': [port] if port else [], 'lifecycle': 'active', 'owner': getpass.getuser(),
            'launch': command, 'docs': docs, 'maintenance': 'Discovered from ' + manifest + '. Review the launch command and URL before starting.'},
            'reasons': reasons, 'needsPort': port is None}


def discover(path, existing=(), depth=2):
    root = Path(path).expanduser()
    if not root.is_absolute() or not root.is_dir() or root.is_symlink():
        raise ValueError('Choose an existing absolute project folder.')
    root = root.resolve()
    if root == Path(root.anchor) or root == Path.home() or any(p in SKIP or p.startswith('.env') for p in root.parts):
        raise ValueError('Choose a project or workspace folder, rather than your home or a system folder.')
    depth = max(0, min(int(depth), 3))
    candidates, warnings, stack, seen = [], [], [(root, 0)], 0
    while stack and seen < 300 and len(candidates) < 40:
        directory, level = stack.pop()
        seen += 1
        try:
            candidate = inspect_project(directory, existing)
            if candidate:
                candidates.append(candidate)
            if level < depth:
                stack.extend((p, level + 1) for p in sorted(directory.iterdir(), reverse=True)
                             if not p.is_symlink() and p.is_dir() and p.name not in SKIP and not p.name.startswith('.'))
        except (OSError, ValueError, TypeError) as exc:
            warnings.append(f'Could not inspect {directory.name}: {str(exc)[:120]}')
    return {'candidates': candidates, 'warnings': warnings, 'scanned': seen,
            'limited': bool(stack), 'root': str(root)}
