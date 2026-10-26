# Legacy Python backend

This directory preserves the previous implementation and its regression tests during the Go
migration. Production uses `cmd/workbench` and `internal/`; no Python module is imported by Go.

Run the migration baseline from the repository root:

```sh
python3 -m unittest discover -s legacy/python/tests -v
```

The preserved backend resolves catalog/runtime data against the installation root and frontend
assets under `web/static`. Its runner stays beside the legacy Python source. Tests use temporary
fixtures; relocating the code does not create a second personal registry.

Transitional CLI commands are under `scripts/compat`, and the optional instruction installer
is under `scripts`. See `docs/migration.md` before retiring the Python code. Neither this backend
nor the compatibility commands is included in the Go distribution archives.
