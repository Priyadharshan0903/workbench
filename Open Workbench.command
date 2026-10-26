#!/bin/zsh
cd -- "${0:A:h}" || exit 1
if [[ ! -x ./workbench ]]; then
  for workbench_go in go /opt/homebrew/bin/go /usr/local/go/bin/go; do
    if command -v "$workbench_go" >/dev/null 2>&1; then
      "$workbench_go" build -trimpath -o workbench ./cmd/workbench || exit 1
      break
    fi
  done
fi
if [[ -x ./workbench ]]; then
  exec ./workbench start
fi
echo 'Build Workbench with Go 1.24+ first, or use the executable package for your Mac.'
read -r '?Press Return to close.'
