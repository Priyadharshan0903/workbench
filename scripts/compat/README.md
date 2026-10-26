# Transitional Python entry points

New automation should call `./workbench` directly. During migration, these commands forward to
the same Go binary and catalog, independently of the shell's current working directory:

```sh
python3 scripts/compat/start.py --port 8732 --no-browser
python3 scripts/compat/registry.py list --json
python3 scripts/compat/package.py --runtime-only
```

If the binary is absent, the helper builds `./cmd/workbench` with an installed Go compiler.
These wrappers belong to source-checkout compatibility and are excluded from Go ZIPs.
