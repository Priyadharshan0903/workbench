"""Local inventory shared by the HTTP dashboard and agent CLI. Standard library only."""
import datetime as dt
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

SOURCE_ROOT = Path(__file__).resolve().parent
ROOT = SOURCE_ROOT.parents[1]
REGISTRY = ROOT / 'registry.json'
LSOF = shutil.which('lsof') or '/usr/sbin/lsof'
PS = shutil.which('ps') or '/bin/ps'


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def read_registry(path=REGISTRY):
    data = json.loads(Path(path).read_text())
    if (not isinstance(data, dict) or data.get('version') != 1
            or type(data.get('revision')) is not int or data['revision'] < 1
            or not isinstance(data.get('projects'), list)):
        raise ValueError('Unsupported or invalid registry. Restore a valid backup before editing.')
    ids = set()
    for project in data['projects']:
        validate_project(project)
        if project['id'] in ids:
            raise ValueError('Duplicate project IDs in registry.')
        ids.add(project['id'])
    return data


def validate_project(project):
    if not isinstance(project, dict):
        raise ValueError('Project must be an object.')
    fields = {'id', 'name', 'description', 'kind', 'path', 'url', 'ports', 'lifecycle',
              'owner', 'launch', 'maintenance', 'docs', 'updatedAt'}
    if set(project) - fields:
        raise ValueError('Unknown project fields: ' + ', '.join(sorted(set(project) - fields)))
    for key in fields - {'ports'}:
        value = project.get(key, '')
        if not isinstance(value, str) or len(value) > (4000 if key in {'maintenance', 'description'} else 2000):
            raise ValueError(f'Invalid {key}.')
    if not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,63}', project.get('id', '')):
        raise ValueError('ID must contain lowercase letters, numbers, and hyphens (up to 64 characters).')
    if not project.get('name', '').strip():
        raise ValueError('A project name is required.')
    if project.get('kind') not in {'web', 'desktop', 'cli', 'library'}:
        raise ValueError('Choose web, desktop, cli, or library.')
    if project.get('lifecycle') not in {'active', 'paused', 'archived'}:
        raise ValueError('Choose active, paused, or archived.')
    ports = project.get('ports', [])
    if not isinstance(ports, list) or len(ports) > 30 or any(type(p) is not int or not 1 <= p <= 65535 for p in ports):
        raise ValueError('Ports must be integers from 1 to 65535.')
    if len(ports) != len(set(ports)):
        raise ValueError('A port is listed more than once.')
    from urllib.parse import urlparse
    url = project.get('url', '')
    parsed = urlparse(url)
    if url and (parsed.scheme not in {'http', 'https'} or not parsed.hostname or parsed.username or parsed.password):
        raise ValueError('URL must be an http(s) address without credentials.')
    path = project.get('path', '')
    if path and (not Path(path).is_absolute() or '\x00' in path):
        raise ValueError('Use an absolute project path.')
    if any(part == '.env' or part.startswith('.env.') for part in Path(path).parts):
        raise ValueError('Secret files cannot be registered as project roots.')


def save_project(project, revision=None, path=REGISTRY):
    """Atomic replacement with an OS lock, previous snapshot, and optimistic UI revisions."""
    validate_project(project)
    path = Path(path)
    with path.with_suffix('.json.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        data = read_registry(path)
        if revision is not None and revision != data['revision']:
            raise ValueError('The registry changed. Refresh before saving your edit.')
        project = dict(project, updatedAt=now())
        existing = next((i for i, p in enumerate(data['projects']) if p['id'] == project['id']), None)
        if existing is None:
            data['projects'].append(project)
        else:
            data['projects'][existing] = project
        previous = path.with_name('registry.previous.json')
        atomic_write(previous, path.read_text())
        data['revision'] += 1
        atomic_write(path, json.dumps(data, indent=2, ensure_ascii=False) + '\n')
        return data


def atomic_write(path, text):
    fd, temporary = tempfile.mkstemp(prefix='.' + path.name, dir=path.parent)
    try:
        with os.fdopen(fd, 'w') as stream:
            stream.write(text)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def run(argv, timeout=8):
    result = subprocess.run(argv, capture_output=True, text=True, timeout=timeout)
    if result.returncode not in {0, 1}:
        raise OSError(result.stderr.strip()[:300] or 'Local process inspection failed.')
    # lsof returns 1 for no matches; other failures must not look like an empty machine.
    if result.returncode == 1 and (result.stderr.strip() or result.stdout.strip()):
        raise OSError(result.stderr.strip()[:300] or 'Local process inspection failed.')
    return result.stdout


def parse_listeners(output):
    processes = {}
    current = None
    for line in output.splitlines():
        key, value = line[:1], line[1:]
        if key == 'p':
            current = processes.setdefault(int(value), {'pid': int(value), 'name': '', 'endpoints': []})
        elif current is not None and key == 'c':
            current['name'] = value
        elif current is not None and key == 'n' and re.search(r':\d+$', value):
            if value not in current['endpoints']:
                current['endpoints'].append(value)
    for process in processes.values():
        process['ports'] = sorted({int(e.rsplit(':', 1)[1]) for e in process['endpoints']})
    return list(processes.values())


def process_cwds(pids):
    if not pids:
        return {}
    result = run([LSOF, '-a', '-p', ','.join(map(str, pids)), '-d', 'cwd', '-Fn'])
    paths, pid = {}, None
    for line in result.splitlines():
        if line.startswith('p'):
            pid = int(line[1:])
        elif line.startswith('n') and pid is not None:
            paths[pid] = line[1:]
    return paths


def within(path, root):
    if not path or not root:
        return False
    try:
        Path(path).resolve().relative_to(Path(root).resolve())
        return True
    except (ValueError, OSError):
        return False


def inspect_services(projects):
    processes = parse_listeners(run([LSOF, '-nP', '-iTCP', '-sTCP:LISTEN', '-Fpcn']))
    cwds = process_cwds([p['pid'] for p in processes])
    for process in processes:
        process['cwd'] = cwds.get(process['pid'], '')
        matches = [p for p in projects if p.get('path') and within(process['cwd'], p['path'])]
        # Sibling tools may share a repository. Prefer the expected port when
        # their source roots are identical, while retaining the deepest-root rule.
        project = max(matches, key=lambda p: (len(p['path']), bool(set(process['ports']) & set(p.get('ports', []))))) if matches else None
        process['projectId'] = project['id'] if project else None
        process['projectName'] = project['name'] if project else None
        process['unexpectedPorts'] = [p for p in process['ports'] if p not in project.get('ports', [])] if project else []
        process['self'] = process['pid'] == os.getpid()
        # An installed app/native integration may not listen on TCP at all.
        process['canStop'] = bool(project and project['kind'] == 'web' and not process['self'])
    return processes


def snapshot(path=REGISTRY):
    registry = read_registry(path)
    error = None
    try:
        services = inspect_services(registry['projects'])
    except (OSError, subprocess.SubprocessError, ValueError) as exc:
        services, error = [], str(exc)
    projects = []
    for original in registry['projects']:
        project = dict(original)
        own = [s for s in services if s['projectId'] == project['id']]
        conflicts = [s for s in services if s['projectId'] != project['id'] and set(s['ports']) & set(project.get('ports', []))]
        project['services'] = own
        project['conflicts'] = conflicts
        project['pathExists'] = bool(project.get('path') and Path(project['path']).exists())
        project['extraServices'] = len(own) > 1 or any(s['unexpectedPorts'] for s in own)
        if error:
            status = 'unknown'
        elif conflicts:
            status = 'conflict'
        elif own:
            status = 'running'
        elif project.get('path') and not project['pathExists']:
            status = 'unlocated'
        elif project['kind'] == 'web' and project.get('ports'):
            status = 'stopped'
        elif project['pathExists']:
            status = 'installed' if project['kind'] == 'desktop' else 'available'
        else:
            status = 'unlocated'
        project['status'] = status
        projects.append(project)
    return {'revision': registry['revision'], 'projects': projects, 'services': services,
            'inspectionError': error, 'scannedAt': now(), 'registryPath': str(path)}


def skill_roots(projects, home=None):
    home = Path(home) if home else Path.home()
    roots = [(home / '.codex/skills', 'codex', 'Personal', 4),
             (home / '.agents/skills', 'codex', 'Shared', 3),
             (home / '.claude/skills', 'claude', 'Personal', 5),
             (home / '.codex/plugins/cache', 'codex', 'Plugin cache', 8),
             (home / '.claude/plugins/cache', 'claude', 'Plugin cache', 8),
             (home / '.claude/plugins/synced', 'claude', 'Synced plugin', 7)]
    for project in projects:
        if project.get('path') and Path(project['path']).is_dir() and not project['path'].endswith('.app'):
            root = Path(project['path'])
            for folder, provider in [('.codex/skills', 'codex'), ('.agents/skills', 'codex'), ('.claude/skills', 'claude')]:
                roots.append((root / folder, provider, project['name'], 4))
            # Skills may be defined on the repository that contains a subproject.
            if root.parent.joinpath('.git').is_dir():
                for folder, provider in [('.codex/skills', 'codex'), ('.agents/skills', 'codex'), ('.claude/skills', 'claude')]:
                    roots.append((root.parent / folder, provider, root.parent.name, 4))
    return roots


def walk_skills(root, depth):
    """Only named SKILL.md files; bounded traversal, linked skills supported, no secret/config reads."""
    seen, stack = set(), [(Path(root), 0)]
    while stack:
        directory, level = stack.pop()
        try:
            real = directory.resolve()
            if real in seen:
                continue
            seen.add(real)
            candidate = directory / 'SKILL.md'
            if candidate.is_file():
                yield candidate
                continue
            if level >= depth:
                continue
            stack.extend((p, level + 1) for p in directory.iterdir()
                         if p.is_dir() and p.name not in {'.git', 'node_modules', 'vendor', '.env'} and not p.name.startswith('.env.'))
        except (OSError, RuntimeError):
            continue


def skill_metadata(text, fallback):
    fields = {}
    if text.startswith('---\n') or text.startswith('---\r\n'):
        parts = text.split('---', 2)
        if len(parts) == 3:
            key = None
            for line in parts[1].splitlines():
                match = re.match(r'^(name|description):\s*(.*)$', line)
                if match:
                    key, value = match.groups()
                    fields[key] = '' if value in {'>', '|', '>-', '|-'} else value.strip('"\'')
                elif key and line[:1].isspace() and line.strip():
                    fields[key] += ' ' + line.strip()
                else:
                    key = None
    return {'name': fields.get('name') or fallback,
            'description': fields.get('description', '').strip() or 'No description recorded in SKILL.md.'}


def discover_skills(projects, home=None):
    skills, warnings = {}, []
    for root, provider, scope, depth in skill_roots(projects, home):
        for file in walk_skills(root, depth):
            try:
                real = str(file.resolve())
                if real not in skills:
                    if file.stat().st_size > 512_000:
                        warnings.append(f'Skipped oversized skill: {file}')
                        continue
                    metadata = skill_metadata(file.read_text(errors='replace'), file.parent.name)
                    skills[real] = dict(metadata, id=hashlib.sha256(real.encode()).hexdigest()[:24],
                                        path=real, providers=[], scopes=[], locations=[])
                item = skills[real]
                for field, value in [('providers', provider), ('scopes', scope), ('locations', str(file))]:
                    if value not in item[field]:
                        item[field].append(value)
            except (OSError, RuntimeError) as exc:
                warnings.append(f'Cannot read {file}: {exc}')
    return {'skills': sorted(skills.values(), key=lambda x: (x['name'].lower(), x['path'])),
            'warnings': warnings, 'scannedAt': now(),
            'note': 'Discovery shows files on disk, not proof a skill is enabled in the current agent session. Plugin caches may contain older versions.'}
