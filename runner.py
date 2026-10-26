#!/usr/bin/env python3
"""Detached command runner. Takes a validated launch plan on stdin, never a shell string."""
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import threading
import inventory

child = None
stopping = False


def terminate(_signal, _frame):
    global stopping
    stopping = True
    if child is not None and child.poll() is None:
        child.terminate()


def execute(plan):
    global child
    state_path, log_path = Path(plan['statePath']), Path(plan['logPath'])
    record = plan['record']

    def save(**values):
        record.update(values, updatedAt=inventory.now())
        inventory.atomic_write(state_path, json.dumps(record))

    def append(chunk):
        # Retain two bounded log segments for long-running services.
        if log_path.exists() and log_path.stat().st_size > 1_048_576:
            os.replace(log_path, log_path.with_suffix('.log.previous'))
        with log_path.open('ab') as stream:
            stream.write(chunk)

    signal.signal(signal.SIGTERM, terminate)
    signal.signal(signal.SIGINT, terminate)
    save(pid=os.getpid(), phase='starting')
    try:
        for argv in plan['commands']:
            if stopping:
                break
            append(('\n$ ' + __import__('shlex').join(argv) + '\n').encode())
            child = subprocess.Popen(argv, cwd=plan['cwd'], env=plan['env'], stdin=subprocess.DEVNULL,
                                     stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
            save(childPid=child.pid)
            while True:
                chunk = child.stdout.read1(8192)
                if not chunk:
                    break
                append(chunk)
            child.stdout.close()
            code = child.wait()
            if code:
                save(phase='stopped' if stopping else 'failed', exitCode=code, finishedAt=inventory.now())
                return
        save(phase='stopped' if stopping else 'completed', exitCode=0, finishedAt=inventory.now())
    except (OSError, ValueError) as exc:
        append(('\nWorkbench: ' + str(exc) + '\n').encode())
        save(phase='failed', exitCode=1, finishedAt=inventory.now())


if __name__ == '__main__':
    execute(json.load(sys.stdin))
