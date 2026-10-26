"""Compatibility entry points for older source-checkout automation."""
import os
from pathlib import Path
import shutil
import subprocess
import sys


def run(command, args=None):
    root = Path(__file__).resolve().parents[2]
    binary = root / 'workbench'
    if not binary.is_file():
        compiler = shutil.which('go')
        if not compiler:
            raise SystemExit('Build Workbench with Go 1.24+ first: make build')
        subprocess.run([compiler, 'build', '-trimpath', '-o', str(binary), './cmd/workbench'], cwd=root, check=True)
    arguments = sys.argv[1:] if args is None else args
    os.execv(str(binary), [str(binary), '--root', str(root), *command, *arguments])
