## Why

There is no reusable, framework-agnostic component that gives an LLM agent coding capabilities inside an isolated environment. Existing solutions either bundle tools into monolithic agent frameworks, orchestrate containers from outside rather than running inside them, or operate on the host filesystem with no isolation. `boris` needs its core foundation: the MCP server, transports, coding tools, and path scoping that make it a drop-in coding sandbox.

## What Changes

- Add `bash` tool with working directory tracking via pwd sentinel, streaming output via SSE, configurable timeout, and process tree cleanup on timeout
- Add `view` tool for reading files (with line numbers, line ranges, large file truncation, binary detection) and listing directories (2 levels deep, ignoring noise dirs)
- Add `str_replace` tool for unique string replacement in files with context snippet confirmation
- Add `create_file` tool for writing new files with optional overwrite, automatic parent directory creation
- Add MCP server using the official `github.com/modelcontextprotocol/go-sdk` with streamable HTTP transport (SSE streaming) on the SDK's default port
- Add STDIO transport for direct client launch (Claude Desktop, Cursor integration)
- Add health check endpoint (GET /health, HTTP mode only)
- Add path scoping via `--allow-dir` and `--deny-dir` CLI flags with symlink-aware canonicalization
- Add `--no-bash` flag to disable shell tool entirely
- Add CLI flags and environment variable configuration (`--port`, `--transport`, `--workdir`, `--timeout`)
- Cross-compile for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64

## Capabilities

### New Capabilities

- `bash-tool`: Shell command execution with per-session working directory tracking (pwd sentinel), streaming output, configurable timeout with process tree kill, and non-interactive enforcement
- `file-tools`: File viewing with line numbers and ranges, directory listing, unique string replacement in files, and file creation with overwrite protection — the core file operation primitives
- `mcp-server`: MCP protocol server built on the official `github.com/modelcontextprotocol/go-sdk`, exposing tools over streamable HTTP (with SSE) and STDIO transports, health check endpoint, CLI flag and env var configuration
- `path-scoping`: Directory allow/deny list enforcement for file tools — canonicalize paths (resolve symlinks), check against allow list, deny takes precedence, glob pattern support for deny entries

### Modified Capabilities

(none — greenfield project)

## Impact

- **Code**: New Go packages under `cmd/boris` and `internal/` — server, transport, tools, path resolver
- **Dependencies**: Official `github.com/modelcontextprotocol/go-sdk`, `github.com/alecthomas/kong` for CLI parsing, glob matching library (doublestar for `**/` patterns)
- **APIs**: Exposes MCP tool interface over HTTP (SDK default port) and STDIO
- **Build**: Cross-compilation targets for 4 OS/arch combinations, single static binary
