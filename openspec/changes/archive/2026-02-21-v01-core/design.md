## Context

Boris is a greenfield Go project — a single static binary exposing coding tools (bash, view, str_replace, create_file) over MCP. The project skeleton exists (`cmd/boris/main.go`, `internal/`, `go.mod` with `github.com/mjkoo/boris`). The official `github.com/modelcontextprotocol/go-sdk` (v1.3.x) provides the MCP server, transport, and tool registration APIs.

## Goals / Non-Goals

**Goals:**

- Implement a working MCP server with bash, view, str_replace, and create_file tools
- Support streamable HTTP and STDIO transports using the official Go SDK
- Provide path scoping (allow/deny dirs) for file tools
- Ship as a single statically-linked binary for linux/darwin on amd64/arm64
- Keep the codebase simple — minimal abstraction, straightforward package layout

**Non-Goals:**

- Incremental bash output streaming (v0.1 collects output and returns it as a single result — see decision below)
- Multi-session support (`--sessions` flag is v0.2)
- `grep` and `find` tools (v0.2)
- `--anthropic-compat` combined tool schema (v0.2)
- `--token` bearer auth (v0.2)
- Published Docker images or helper base images (v0.3)

## Decisions

### 1. Use `mcp.AddTool` generic API for tool registration

Use the SDK's generic `mcp.AddTool[In, Out]` function with Go struct types. Tool input schemas are auto-inferred from struct tags (`json` for names, `jsonschema` for descriptions). This avoids hand-writing JSON schemas and keeps tool definitions type-safe.

```go
type BashArgs struct {
    Command string `json:"command" jsonschema:"required,the shell command to execute"`
    Timeout int    `json:"timeout,omitempty" jsonschema:"timeout in seconds"`
}
mcp.AddTool(server, &mcp.Tool{Name: "bash", Description: "..."}, bashHandler)
```

**Alternative considered:** Raw `server.AddTool` with manual JSON schemas — more verbose, no compile-time type checking, no advantage.

### 2. StreamableHTTPHandler for HTTP transport

Use `mcp.NewStreamableHTTPHandler` (the current recommended transport), not the deprecated SSE handler. Default listen address is `:8080`. The handler is a standard `http.Handler` that plugs into `net/http`.

For STDIO, use `server.Run(ctx, &mcp.StdioTransport{})`. Transport selection via `--transport=http|stdio` flag.

**Alternative considered:** SSE handler — deprecated in the MCP spec, no reason to use it for a new project.

### 3. Bash output: collect-then-return for v0.1

MCP tool handlers are request/response — `func(...) (*CallToolResult, error)`. The SDK does not provide a mechanism for streaming partial results from within a tool handler. v0.1 will execute bash commands, collect all stdout/stderr, and return them as a single `CallToolResult`. This is the simplest correct approach.

True incremental streaming (via MCP progress notifications or a custom mechanism) can be added in a future version if needed. The timeout mechanism ensures long-running commands don't block indefinitely.

**Alternative considered:** Sending MCP `notifications/progress` during execution — adds significant complexity, the SDK's support for this from within tool handlers is unclear, and most MCP clients don't render progress notifications as streaming output anyway.

### 4. Package layout

```
cmd/boris/
  main.go              CLI entry, flag parsing, server construction, transport selection
internal/
  tools/
    bash.go            bash tool handler + cwd tracking
    view.go            view tool handler (file read + dir listing)
    str_replace.go     str_replace tool handler
    create_file.go     create_file tool handler
    tools.go           RegisterAll(server, pathResolver, session) wiring function
  pathscope/
    pathscope.go       Resolver: canonicalize, check allow/deny lists
  session/
    session.go         Session state: tracked cwd (mutex-protected)
```

- `tools` package contains each tool as a standalone file. A `RegisterAll` function wires them all to an `mcp.Server`, injecting the shared path resolver and session.
- `pathscope` is a standalone package — file tools call `resolver.Resolve(path)` which returns `(canonicalPath, error)`. Deny errors are returned as tool errors, not panics.
- `session` holds the working directory state. A single `Session` struct with `Cwd()` and `SetCwd()` behind a `sync.Mutex`. The bash tool updates it after each command; file tools use it to resolve relative paths.

### 5. Bash working directory tracking via pwd sentinel

Each bash command executes as:

```sh
cd <tracked_cwd> && <user_command> ; echo '__BORIS_CWD__' ; pwd
```

After execution, parse stdout from the end: find the `__BORIS_CWD__` sentinel, read the line after it as the new cwd. Strip both the sentinel line and the pwd line from the output returned to the caller. If the sentinel is missing (e.g., command was killed by timeout), cwd remains unchanged.

Commands run via `/bin/sh -c` with the process in its own process group (`syscall.SysProcAttr{Setpgid: true}`). On timeout, kill the entire process group via `syscall.Kill(-pgid, syscall.SIGKILL)`. Partial stdout/stderr captured before the kill is included in the response.

### 6. Path scoping architecture

`pathscope.Resolver` is constructed at startup from CLI flags:

- **No `--allow-dir`**: All paths allowed. `Resolve()` only canonicalizes.
- **With `--allow-dir`**: `Resolve()` canonicalizes (via `filepath.EvalSymlinks` + `filepath.Abs`), then checks the result is under an allowed directory.
- **`--deny-dir`**: Checked after allow, always takes precedence. Supports glob patterns via `bmatcuk/doublestar` (for `**/` patterns like `**/.env`).

The resolver is injected into all file tool handlers. The bash tool does NOT go through the path resolver — bash is best-effort containment only (the `cd` into the working directory keeps it scoped, but shell commands can access anything). `--no-bash` disables the bash tool entirely for strict file-only use cases.

### 7. CLI configuration

Use `github.com/alecthomas/kong` for CLI argument parsing. Kong provides struct-based flag definitions with built-in support for environment variable binding (`env:"BORIS_PORT"`), repeatable flags (slice types), custom type mappers, and automatic help generation. Flags:

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--port` | `BORIS_PORT` | `8080` | Listen port (HTTP mode) |
| `--transport` | `BORIS_TRANSPORT` | `http` | Transport: `http` or `stdio` |
| `--workdir` | `BORIS_WORKDIR` | `.` | Initial working directory |
| `--timeout` | `BORIS_TIMEOUT` | `120` | Default bash timeout (seconds) |
| `--allow-dir` | `BORIS_ALLOW_DIRS` | (none) | Allowed directories (repeatable flag, comma-sep env) |
| `--deny-dir` | `BORIS_DENY_DIRS` | (none) | Denied directories/patterns (repeatable flag, comma-sep env) |
| `--no-bash` | `BORIS_NO_BASH` | `false` | Disable bash tool |
| `--max-file-size` | `BORIS_MAX_FILE_SIZE` | `10MB` | Max file size for view/create |

### 8. Health check

A separate `GET /health` handler registered alongside the MCP handler when using HTTP transport. Returns `200 OK` with a simple JSON body. Implemented in `main.go` — no separate package needed.

### 9. Static binary build

Build with `CGO_ENABLED=0` and appropriate `GOOS`/`GOARCH`. A `Makefile` target or build script cross-compiles for the 4 targets (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64). Go's standard library supports this natively.

## Risks / Trade-offs

**No bash output streaming in v0.1** → For very long-running commands (multi-minute builds), the agent gets no feedback until completion or timeout. Mitigation: configurable timeout with partial output on kill. Agents can run commands in smaller increments.

**Bash is not sandboxed by path scoping** → Shell commands can access any path regardless of `--allow-dir`. This is an explicit design choice documented in the PRD. Mitigation: `--no-bash` flag for strict file-only mode; for real isolation, run boris inside a container.

**pwd sentinel can appear in user output** → If a command happens to print `__BORIS_CWD__` followed by a path, the parser could misidentify it. Mitigation: use a sufficiently unique sentinel (e.g., `__BORIS_CWD_SENTINEL_<random>__` generated at session start). Risk is negligible in practice.

**`doublestar` dependency for glob matching** → Adds an external dependency. Mitigation: `doublestar` is a small, well-maintained, widely-used library with no transitive dependencies. The alternative (Go's `filepath.Match`) doesn't support `**/` which is the primary use case for deny patterns.
