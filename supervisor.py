"""Launch registered projects and report readiness without assuming port ownership."""
from concurrent.futures import ThreadPoolExecutor
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import sys
import threading
import time
from urllib.error import HTTPError
from urllib.parse import urlparse
from urllib.request import build_opener, ProxyHandler, HTTPRedirectHandler

import inventory

RUNTIME = inventory.ROOT / '.runtime'
START_LOCK = threading.Lock()
HEALTH_LOCK = threading.Lock()
HEALTH_CACHE = {}
CHILDREN = {}


def runtime_environment():
    env = dict(os.environ)
    paths = env.get('PATH', '').split(os.pathsep)
    paths.extend([str(Path(sys.executable).parent), '/opt/homebrew/bin', '/usr/local/bin', '/usr/bin', '/bin'])
    versions = Path.home() / '.nvm/versions/node'
    if versions.is_dir():
        paths.extend(str(p / 'bin') for p in sorted(versions.iterdir(), reverse=True) if p.is_dir())
    env['PATH'] = os.pathsep.join(dict.fromkeys(p for p in paths if p))
    env['PYTHONUNBUFFERED'] = '1'
    env['NO_COLOR'] = '1'
    return env


def parse_commands(command, cwd, env=None):
    if not isinstance(command, str) or not command.strip():
        raise ValueError('Add a launch command in project settings first.')
    if any(x in command for x in ('\n', '\r', '\x00', '`', '$(', '${')):
        raise ValueError('Use a plain launch command. Shell substitutions are not supported.')
    lexer = shlex.shlex(command, posix=True, punctuation_chars=';&|<>')
    lexer.whitespace_split = True
    lexer.commenters = ''
    commands, argv = [], []
    for token in lexer:
        if token == '&&':
            if not argv:
                raise ValueError('Each launch step needs a command.')
            commands.append(argv)
            argv = []
        elif token in {';', '&', '|', '||', '>', '>>', '<', '<<', '>&', '2>'}:
            raise ValueError('Pipes, backgrounding, and redirection are not supported. Use a project script instead.')
        else:
            argv.append(os.path.expanduser(token))
    if not argv:
        raise ValueError('The launch command ends with an empty step.')
    commands.append(argv)
    if len(commands) > 6:
        raise ValueError('Use at most six launch steps, or put them in a project script.')
    env = env or runtime_environment()
    for argv in commands:
        if argv[0] in {'python', 'python3'}:
            argv[0] = sys.executable
        executable = shutil.which(argv[0], path=env['PATH'])
        if not executable:
            raise ValueError(f'{argv[0]} is not installed or is not on PATH. Install it, then retry.')
        argv[0] = executable
        # Existing portable launchers should not open a second browser tab.
        if len(argv) == 2 and argv[1] == 'start.py':
            script = Path(cwd) / 'start.py'
            if not script.is_symlink() and script.is_file() and script.stat().st_size < 65_536 and '--no-browser' in script.read_text(errors='replace'):
                argv.append('--no-browser')
    return commands


def project_for(project_id, registry=None):
    registry = registry or inventory.read_registry()
    project = next((p for p in registry['projects'] if p['id'] == project_id), None)
    if not project:
        raise ValueError('This project is no longer in the registry. Refresh the workspace.')
    return project


def local_url(url):
    parsed = urlparse(url)
    return parsed.scheme in {'http', 'https'} and parsed.hostname in {'127.0.0.1', 'localhost', '::1'} and not parsed.username and not parsed.password


def launch_plan(project_id):
    registry = inventory.read_registry()
    project = project_for(project_id, registry)
    if project['id'] == 'workbench' or Path(project.get('path') or '/').resolve() == inventory.ROOT:
        raise ValueError('Workbench is already open. Use its launcher to reopen it after quitting.')
    if project['kind'] != 'web':
        raise ValueError('This item uses its own launcher. Open its details for the recorded command.')
    if project['lifecycle'] != 'active':
        raise ValueError('Set this project to Active in project settings before starting it.')
    cwd = Path(project.get('path') or '').expanduser()
    if not cwd.is_absolute() or not cwd.is_dir():
        raise ValueError('The project folder has moved or is missing. Update its path in project settings.')
    if not project['ports'] or not local_url(project.get('url', '')):
        raise ValueError('Add a local website URL and its expected port in project settings before starting.')
    port = urlparse(project['url']).port or (443 if project['url'].startswith('https:') else 80)
    if port not in project['ports']:
        raise ValueError('The website URL must use one of the project’s expected ports.')
    reserved = [p['name'] for p in registry['projects'] if p['id'] != project_id and set(project['ports']) & set(p['ports'])]
    if reserved:
        raise ValueError('An expected port is also reserved by ' + ', '.join(reserved) + '. Choose distinct ports first.')
    commands = parse_commands(project['launch'], cwd)
    plan = {'id': project_id, 'name': project['name'], 'cwd': str(cwd.resolve()), 'commands': commands,
            'command': ' && '.join(shlex.join(a) for a in commands), 'url': project['url'],
            'ports': project['ports'], 'revision': registry['revision']}
    plan['fingerprint'] = hashlib.sha256(json.dumps(plan, sort_keys=True).encode()).hexdigest()
    return plan


def record_path(project_id):
    if not isinstance(project_id, str) or not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,63}', project_id):
        raise ValueError('Invalid project ID.')
    return RUNTIME / 'runs' / (project_id + '.json')


def read_run(project_id):
    path = record_path(project_id)
    try:
        if path.stat().st_size > 32_768:
            return None
        return json.loads(path.read_text())
    except (OSError, ValueError):
        return None


def birth(pid):
    try:
        return inventory.run([inventory.PS, '-p', str(pid), '-o', 'lstart=']).strip() or None
    except (OSError, subprocess.SubprocessError):
        return None


def age(record):
    try:
        return (dt.datetime.now(dt.timezone.utc) - dt.datetime.fromisoformat(record['startedAt'])).total_seconds()
    except (KeyError, ValueError):
        return 999


def active_run(record):
    if not record or record.get('phase') != 'starting':
        return False
    process = CHILDREN.get(record.get('runId'))
    if process:
        return process.poll() is None
    return bool(record.get('identity') and birth(record.get('pid')) == record['identity'])


def start_project(project_id, fingerprint, revision):
    if type(revision) is not int or not isinstance(fingerprint, str):
        raise ValueError('Review the current launch plan before starting.')
    with START_LOCK:
        plan = launch_plan(project_id)
        if revision != plan['revision'] or fingerprint != plan['fingerprint']:
            raise ValueError('The launch details changed. Review the current command before starting.')
        current = inventory.snapshot()
        if current['inspectionError']:
            raise ValueError('Cannot inspect local ports. No duplicate service was started. Retry in a normal desktop session.')
        project = next(p for p in current['projects'] if p['id'] == project_id)
        if project['services']:
            return {'ok': True, 'alreadyRunning': True, 'message': 'This project already has a running service.'}
        if project['conflicts']:
            raise ValueError('An expected port is occupied. Inspect the listener in Diagnostics before starting.')
        if active_run(read_run(project_id)):
            return {'ok': True, 'alreadyRunning': True, 'message': 'This project is already starting.'}
        (RUNTIME / 'runs').mkdir(parents=True, exist_ok=True)
        (RUNTIME / 'logs').mkdir(parents=True, exist_ok=True)
        run_id = __import__('secrets').token_hex(8)
        log_path = RUNTIME / 'logs' / f'{project_id}-{run_id}.log'
        record = {'runId': run_id, 'projectId': project_id, 'startedAt': inventory.now(), 'phase': 'starting',
                  'cwd': plan['cwd'], 'command': plan['command'], 'logFile': log_path.name}
        payload = {'commands': plan['commands'], 'cwd': plan['cwd'], 'env': runtime_environment(),
                   'statePath': str(record_path(project_id)), 'logPath': str(log_path), 'record': record}
        with log_path.open('ab') as log:
            process = subprocess.Popen([sys.executable, str(inventory.ROOT / 'runner.py')], cwd=inventory.ROOT,
                                       stdin=subprocess.PIPE, stdout=log, stderr=log, start_new_session=True)
        record.update(pid=process.pid, identity=birth(process.pid))
        inventory.atomic_write(record_path(project_id), json.dumps(record))
        try:
            process.stdin.write(json.dumps(payload).encode())
            process.stdin.close()
        except (OSError, BrokenPipeError) as exc:
            raise ValueError('Could not hand the launch to the runner. Check the project logs.') from exc
        CHILDREN[run_id] = process
        # The runner writes its own state; do not overwrite a fast completion here.
        for _ in range(20):
            current_run = read_run(project_id)
            if current_run and current_run.get('runId') == run_id:
                break
            time.sleep(.02)
        return {'ok': True, 'runId': run_id, 'message': 'Starting ' + plan['name'] + '…'}


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *_args, **_kwargs):
        return None


def check_http(url):
    if not local_url(url):
        return {'state': 'unchecked', 'message': 'Website readiness is checked only for local URLs.'}
    with HEALTH_LOCK:
        cached = HEALTH_CACHE.get(url)
        if cached and time.monotonic() - cached[0] < 4:
            return cached[1]
    started = time.monotonic()
    try:
        opener = build_opener(ProxyHandler({}), NoRedirect())
        with opener.open(url, timeout=1.2) as response:
            code = response.status
        result = {'state': 'ready', 'code': code, 'message': f'Website responded with HTTP {code}.'}
    except HTTPError as exc:
        result = {'state': 'ready' if 300 <= exc.code < 400 else 'error', 'code': exc.code,
                  'message': f'The local URL returned HTTP {exc.code}.'}
        exc.close()
    except (OSError, ValueError) as exc:
        result = {'state': 'unreachable', 'message': 'The local website is not responding yet. Check the URL and project logs.'}
    result['latencyMs'] = round((time.monotonic() - started) * 1000)
    with HEALTH_LOCK:
        HEALTH_CACHE[url] = (time.monotonic(), result)
    return result


def diagnose(text, code):
    lowered = text.lower()
    if any(x in lowered for x in ('eaddrinuse', 'address already in use')):
        return 'The output reports a busy port. Inspect Diagnostics, or update the project’s URL and port before retrying.'
    if any(x in lowered for x in ('modulenotfounderror', 'cannot find module', 'module not found', 'command not found', 'not found: vite')):
        return 'The output reports a missing project dependency. Follow the project’s setup instructions, then start it again.'
    if any(x in lowered for x in ('ebadengine', 'unsupported engine', 'requires node.js')):
        return 'The output reports an unsupported runtime version. Check the version required by the project.'
    if any(x in lowered for x in ('no such file or directory', 'enoent')):
        return 'The output reports a missing file. Check the project folder and launch command in Settings.'
    if any(x in lowered for x in ('permission denied', 'eacces')):
        return 'The output reports a permission problem. Inspect the log for the affected file or port.'
    return f'Launch exited with code {code}. Open logs for the failing step.'


def enrich(snapshot):
    projects = snapshot.get('projects', [])
    checkable = [p for p in projects if p['kind'] == 'web' and (p.get('services') or p.get('conflicts')) and p.get('url')]
    with ThreadPoolExecutor(max_workers=8) as pool:
        health = dict(zip([p['id'] for p in checkable], pool.map(lambda p: check_http(p['url']), checkable)))
    for project in projects:
        record = read_run(project['id'])
        project['run'] = {k: record[k] for k in ('runId', 'startedAt', 'updatedAt', 'phase', 'exitCode', 'finishedAt') if k in record} if record else None
        project['health'] = health.get(project['id'])
        project['canLaunch'] = bool(project['kind'] == 'web' and project['id'] != 'workbench' and project['lifecycle'] == 'active')
        raw = project['status']
        message = ''
        if raw == 'unknown':
            message = 'Process inspection is unavailable. Ownership and readiness cannot be confirmed.'
        elif raw == 'conflict':
            # A listener launched from another folder may belong to this tool. Do not guess.
            if all(not s.get('projectId') for s in project['conflicts']):
                project['status'] = 'unverified'
                message = 'A listener uses this port, but its project ownership is unverified.'
            else:
                message = 'An expected port is used by another registered project.'
        elif raw == 'running':
            if project['kind'] == 'web' and project['health']:
                project['status'] = 'ready' if project['health']['state'] == 'ready' else 'starting' if record and age(record) < 45 else 'unhealthy'
                message = project['health']['message']
        elif raw == 'stopped' and record:
            if record['phase'] == 'failed':
                project['status'] = 'failed'
                try:
                    output = logs(project['id'])['text']
                except (OSError, ValueError):
                    output = ''
                message = diagnose(output, record.get('exitCode', 'unknown'))
            elif active_run(record):
                project['status'] = 'starting' if age(record) < 45 else 'unhealthy'
                message = 'Waiting for the local website.' if age(record) < 45 else 'The command is still running, but no website appeared. Check its URL, port, and logs.'
            elif record['phase'] == 'completed' and age(record) < 45:
                project['status'] = 'starting'
                message = 'The launcher finished. Waiting for the website to become available.'
            elif record['phase'] == 'completed':
                message = 'The last launch has finished. Start again to reopen, or inspect its logs and settings.'
        if project.get('extraServices'):
            message = (message + ' ' if message else '') + 'Multiple processes are associated with this project. Inspect Diagnostics before stopping one.'
        project['statusMessage'] = message
    return snapshot


def redact(text):
    text = re.sub(r'\x1b\[[0-?]*[ -/]*[@-~]', '', text)
    text = re.sub(r'(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+', r'\1[redacted]', text)
    text = re.sub(r'(?i)((?:api[_-]?key|token|password|secret)\s*[=:]\s*)\S+', r'\1[redacted]', text)
    text = re.sub(r'\b(?:gh[pousr]_[A-Za-z0-9_]{12,}|sk-[A-Za-z0-9_-]{20,})\b', '[redacted]', text)
    return text


def logs(project_id):
    project_for(project_id)
    record = read_run(project_id)
    if not record:
        return {'text': '', 'message': 'No Workbench launch logs yet. Start the project here to capture its output.'}
    filename = record.get('logFile', '')
    if Path(filename).name != filename or not re.fullmatch(r'[a-z0-9-]+-[a-f0-9]{16}\.log', filename):
        raise ValueError('The launch log reference is invalid.')
    path = RUNTIME / 'logs' / filename
    try:
        with path.open('rb') as stream:
            stream.seek(0, 2)
            size = stream.tell()
            stream.seek(max(0, size - 24_000))
            text = stream.read().decode(errors='replace')
    except FileNotFoundError:
        text = ''
    return {'text': redact('\n'.join(text.splitlines()[-160:])), 'run': {k: v for k, v in record.items() if k not in {'identity', 'logFile'}},
            'message': 'Most recent output · last 160 lines · common credentials masked'}
