# Workbench source

Read README.md and SETUP.md first. Production uses Python standard library only.
Before starting a server, inspect the local registry and live reservations with
`python3 registry.py status --json` and `python3 registry.py check-port PORT`.
Reuse a healthy instance of this installation. Do not stop unknown or duplicate
processes automatically. Use the CLI to update registry records atomically.
Project launch requires an explicit reviewed plan; stop requires a fresh process
identity check and human confirmation. Discovery never executes project code.
Never read secret files. Keep registry.json, browser preferences, launch logs and
runtime settings out of source archives. Tests use isolated temporary fixtures.
