#!/bin/zsh
cd -- "${0:A:h}" || exit 1
for workbench_python in python3 /opt/homebrew/bin/python3 /usr/local/bin/python3; do
  if "$workbench_python" -c 'import sys; raise SystemExit(sys.version_info < (3, 10))' 2>/dev/null; then
    exec "$workbench_python" start.py
  fi
done
echo 'Workbench needs Python 3.10 or newer. Install Python, then open this launcher again.'
read -r '?Press Return to close.'
