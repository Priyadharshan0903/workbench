# Claude Code and Codex integration

Workbench can be used through its CLI or as a local stdio MCP server. Both read the same
version-1 catalog as the dashboard. The MCP process opens no listening port, starts no browser,
needs no API key and does not require the dashboard engine to be running.

## Connect the MCP server

Build once, then run the appropriate command from the Workbench repository folder:

```sh
make build

# Codex CLI; the same MCP configuration can be used by Codex app/IDE clients.
codex mcp add workbench -- "$PWD/workbench" --root "$PWD" mcp

# Claude Code, available across projects for this user.
claude mcp add --transport stdio --scope user workbench -- "$PWD/workbench" --root "$PWD" mcp
```

The shell expands the command and root to absolute paths before saving the client configuration.
Verify with `codex mcp list` or `claude mcp get workbench`. Start a new client session or restart
its MCP connection to load the tools; `/mcp` shows connection status in the terminal clients.
These instructions do not modify client settings automatically.

The client starts the executable and exchanges newline-delimited JSON-RPC through stdin/stdout.
Use the executable command, rather than the dashboard URL on 8732, for this transport.

For Codex manual configuration, use the equivalent entry in its user configuration:

```toml
[mcp_servers.workbench]
command = "/absolute/path/to/Workbench/workbench"
args = ["--root", "/absolute/path/to/Workbench", "mcp"]
startup_timeout_sec = 20
tool_timeout_sec = 60
```

Clients using the `mcpServers` JSON shape can use:

```json
{
  "mcpServers": {
    "workbench": {
      "command": "/absolute/path/to/Workbench/workbench",
      "args": ["--root", "/absolute/path/to/Workbench", "mcp"]
    }
  }
}
```

The CLI registration commands and supported transports were checked against the
[official Codex MCP documentation](https://developers.openai.com/codex/mcp) and
[Claude Code MCP documentation](https://code.claude.com/docs/en/mcp). The adapter implements the
[MCP stdio transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports),
initialization/version negotiation and tool calls. It negotiates the 2025-11-25, 2025-06-18,
2025-03-26 and 2024-11-05 protocol versions. Requests run sequentially with bounded messages.
There is no HTTP MCP endpoint, sampling, resource subscription or streaming log transport.

## Tools

| Tool | What it supplies |
| --- | --- |
| `list_projects` | Durable project records and the current revision. |
| `get_project` | One project's folder, command, URL, ports and maintenance handoff. |
| `project_status` | Real listener ownership, local HTTP readiness and recent runner state. |
| `check_port` | Reservations and actual listeners; inspection failures mean unavailable. |
| `launch_logs` | Bounded output with common credential patterns masked. |
| `discover_skills` | Installed SKILL.md metadata, optionally filtered to Codex or Claude. |

Example prompts:

- “Use Workbench to list my projects and find Branchroom.”
- “Check which project owns port 8721 and whether its website responds.”
- “Read the latest launch output for this project and explain the failure.”

Catalog values and log text are untrusted data, not agent instructions. Tool results include
local project paths and metadata and can reach the connected client's model provider. Log
masking handles common credentials and cannot guarantee removal of every secret a project prints.

## Optional catalog writes

The default server exposes read-only tools. To enable registration, append `--allow-writes`
to the server arguments when configuring the client:

```sh
./workbench --root /absolute/path/to/installation mcp --allow-writes
```

This adds `register_project`. Read `list_projects` and the existing record first, then send
the complete validated project object and the current integer `revision`. A stale revision
is rejected. Saving a project changes the catalog only; it never runs its command.

No MCP tool starts or stops services, even in write mode. Use the dashboard to review the
server-owned launch plan and confirm Start or Stop. Rebuild and restart the MCP connection
after changing the executable; an MCP process keeps its current binary until it exits.

## CLI and agent instructions

An agent with terminal access can use the CLI without MCP:

```sh
./workbench list --json
./workbench status --json
./workbench check-port 3000
./workbench context branchroom
./workbench register --file /path/to/project.json --revision 13
```

Use the actual revision returned by `list --json`; 13 is only an example. The optional
`scripts/install_agent_instructions.py` helper previews Go CLI instructions for AGENTS.md and
CLAUDE.md. Applying those global instructions is a separate explicit action.

## How ports are identified

The catalog's `ports` and `url` describe expected reservations. The inspection layer asks `lsof`
for actual TCP listeners and their process working directories. It matches ownership by resolved
source folder, preferring the deepest root and using expected ports to distinguish tools sharing
the same root. A separate local HTTP probe establishes website readiness.

For Branchroom, its launcher and old catalog specify 8721. Inspection confirms a listener from
the Branchroom folder on that port, and the registered page responds. This is configured
reservation plus observed process/HTTP evidence; it is not inferred from an arbitrary open port.

Importing project records preserves this behaviour. Copying a URL never establishes ownership:
a listener from another folder remains unverified even when its website responds.
