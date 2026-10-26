"""Portable local engine settings and explicit in-place source reloads."""
import hashlib
import json
import os
from pathlib import Path
import sys
import time

SOURCE_ROOT = Path(__file__).resolve().parent
ROOT = SOURCE_ROOT.parents[1]


def port():
    file = ROOT / '.runtime/local.json'
    if not file.exists():
        return 8730
    value = json.loads(file.read_text()).get('port')
    if type(value) is not int or not 1024 <= value <= 65535:
        raise ValueError('Invalid local engine port in .runtime/local.json.')
    return value


def source_stamp():
    digest = hashlib.sha256()
    for name in ('server.py', 'inventory.py', 'supervisor.py', 'runner.py', 'discovery.py', 'engine.py'):
        digest.update((SOURCE_ROOT / name).read_bytes())
    return digest.hexdigest()[:20]


def reload_current():
    # Only this engine is replaced. Detached project runners keep their processes.
    time.sleep(.3)
    os.execv(sys.executable, [sys.executable, *sys.argv])
