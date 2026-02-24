# Boris

*I am invincible!*

Boris is a drop-in [MCP](https://modelcontextprotocol.io/) server that gives any AI agent coding capabilities - shell execution, file editing, search - as a single static Go binary.
It doesn't care where it runs: inside a Docker container, on a CI runner, on your laptop. Point any MCP client at it and your agent can write code.

## Why

Building a coding agent means implementing the same set of tools every time: run a command, read a file, edit a file, search for things.
These tools end up tightly coupled to whatever agent framework you're using.

Boris extracts these into a standalone server that speaks MCP, so any framework that supports MCP can use them.
Copy the binary into a Docker image, add it to your Claude Desktop config, or hit it over HTTP from your own agent - it all works the same way.

## Tools

Boris exposes the core tools that coding agents need:

| Tool | Description |
|------|-------------|
| **bash** | Execute shell commands with streaming output. Working directory persists across calls. Background task support. |
| **view** | Read files with line numbers, or list directories. Supports line ranges for large files. |
| **str_replace** | Replace a unique string in a file. The workhorse of AI code editing. |
| **create_file** | Create or overwrite files. Creates parent directories as needed. |
| **grep** | Search file contents with regex patterns. Multiple output modes. |
| **glob** | Find files by glob pattern. Respects `.gitignore`. |
| **task_output** | Retrieve output from background bash tasks. |

With `--anthropic-compat`, tools are exposed using the schemas Claude models are fine-tuned on (e.g., the combined `str_replace_editor` tool). Other models work fine with the default schemas.

## Quick Start

### Build from source

```bash
go build -o boris ./cmd/boris
```

### Run locally, scoped to a project

```bash
boris --allow-dir=./my-project --workdir=./my-project
```

### Run inside Docker

```dockerfile
COPY boris /usr/local/bin/boris
WORKDIR /workspace
EXPOSE 8080
ENTRYPOINT ["boris"]
```

```bash
docker run -d -p 8080:8080 -v $(pwd):/workspace my-image
# Point any MCP client at http://localhost:8080/mcp
```

### Use with Claude Desktop or Cursor (STDIO)

```json
{
  "mcpServers": {
    "boris": {
      "command": "boris",
      "args": ["--transport=stdio", "--allow-dir=./project", "--workdir=./project"]
    }
  }
}
```

## Configuration

All configuration is via CLI flags or environment variables. No config files required.

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--port` | `BORIS_PORT` | `8080` | Listen port (HTTP mode) |
| `--transport` | `BORIS_TRANSPORT` | `http` | `http` or `stdio` |
| `--workdir` | `BORIS_WORKDIR` | `.` | Initial working directory |
| `--timeout` | `BORIS_TIMEOUT` | `120` | Default bash timeout (seconds) |
| `--allow-dir` | `BORIS_ALLOW_DIRS` | (none) | Allowed directories for file tools (repeatable) |
| `--deny-dir` | `BORIS_DENY_DIRS` | (none) | Denied directories/patterns for file tools (repeatable) |
| `--token` | `BORIS_TOKEN` | (none) | Bearer token for HTTP auth |
| `--generate-token` | `BORIS_GENERATE_TOKEN` | `false` | Generate a random bearer token on startup |
| `--disable-tools` | `BORIS_DISABLE_TOOLS` | (none) | Tools to disable (repeatable, e.g. bash) |
| `--background-task-timeout` | `BORIS_BACKGROUND_TASK_TIMEOUT` | `0` | Background task safety-net timeout in seconds (0=disabled) |
| `--max-file-size` | `BORIS_MAX_FILE_SIZE` | `10MB` | Max file size for view/create |
| `--require-view-before-edit` | `BORIS_REQUIRE_VIEW_BEFORE_EDIT` | `auto` | Require files to be viewed before editing: `auto`, `true`, `false` |
| `--anthropic-compat` | `BORIS_ANTHROPIC_COMPAT` | `false` | Use Claude-compatible tool schemas |
| `--log-level` | `BORIS_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `--log-format` | `BORIS_LOG_FORMAT` | `text` | `text` or `json` |

### Path scoping

File tools (`view`, `str_replace`, `create_file`, `grep`, `glob`) enforce an allow/deny list for paths. Symlinks are resolved before checking.

- **No `--allow-dir`**: all paths allowed (appropriate inside a container).
- **With `--allow-dir`**: only paths within allowed directories are accessible.
- **`--deny-dir`**: always takes precedence over allow. Supports glob patterns (e.g., `**/.env`).

```bash
# Scoped to a project, deny .env files
boris --allow-dir=./my-project --deny-dir='**/.env' --deny-dir='**/.git'

# File tools only, no shell access
boris --allow-dir=./src --allow-dir=./tests --disable-tools bash
```

### Transports

- **HTTP** (default): MCP over streamable HTTP with SSE. Serves on `/mcp` with a health check at `GET /health`. Supports CORS for browser-based clients. Each MCP session gets independent state.
- **STDIO**: MCP over stdin/stdout. The client spawns Boris as a child process. Zero-config integration with Claude Desktop, Cursor, and similar tools.

## Development

Common tasks are codified in the [`justfile`](justfile). Install [just](https://just.systems) to use them, or run the underlying commands directly.

```bash
just          # list available recipes
just test     # go test -race ./...
just vet      # go vet ./...
just build    # go build -o boris ./cmd/boris
just snapshot # goreleaser release --snapshot --clean
```

### Test Environment

Spin up a Docker container running boris against a known copy of the boris repo, then connect Claude Code to it as the sole tool provider.

**Prerequisites:** Docker, [just](https://just.systems), [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code)

```bash
# Terminal 1: start boris in a container
just test-env

# Terminal 2: connect Claude Code (boris-only tools, no built-in tools)
just test-claude
```

Ctrl-C in Terminal 1 stops and removes the container.

## Security

Boris provides shell execution as a tool. This is powerful but inherently dangerous if exposed to untrusted networks.

**Key points:**

- The bash tool **does not enforce path scoping** - this is deliberate. Application-level shell restrictions are fundamentally bypassable. Isolation must come from the deployment environment (containers, OS sandboxes).
- File tools enforce path scoping with symlink resolution, which prevents accidental access outside allowed directories.
- **Always use authentication** (`--token` or `--generate-token`) when Boris is network-accessible.
- The recommended deployment is **inside a container** with only the workspace directory mounted.
- Use `--disable-tools bash` if you need to guarantee that only file operations are available.
