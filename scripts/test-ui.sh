#!/bin/sh
set -eu
workbench_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
workbench_node=${NODE:-node}
for workbench_suite in interface htmx workspace launch-setup skills; do
  "$workbench_node" "$workbench_root/tests/ui/$workbench_suite.cjs"
done
