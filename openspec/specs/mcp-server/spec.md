## ADDED Requirements

### Requirement: MCP server built on official Go SDK
The server SHALL be built using `github.com/modelcontextprotocol/go-sdk/mcp`. Tools SHALL be registered using the SDK's `mcp.AddTool` generic function with Go struct types for automatic schema inference.

#### Scenario: Server exposes tools via MCP protocol
- **WHEN** a client connects and requests the tool list
- **THEN** the server returns all registered tools with their names, descriptions, and JSON schemas

### Requirement: Streamable HTTP transport
When `--transport=http` (the default), the server SHALL use `mcp.NewStreamableHTTPHandler` to serve MCP over HTTP. The server SHALL listen on the port specified by `--port` (default 8080).

#### Scenario: HTTP server starts on default port
- **WHEN** boris is started with `--transport=http` and no `--port` flag
- **THEN** the server listens on port 8080 and accepts MCP connections

#### Scenario: HTTP server starts on custom port
- **WHEN** boris is started with `--port=9000`
- **THEN** the server listens on port 9000

### Requirement: STDIO transport
When `--transport=stdio`, the server SHALL use `mcp.StdioTransport` to communicate over stdin/stdout. The `--port` flag SHALL be ignored in this mode.

#### Scenario: STDIO transport operation
- **WHEN** boris is started with `--transport=stdio`
- **THEN** the server reads MCP JSON-RPC messages from stdin and writes responses to stdout

#### Scenario: Port flag ignored in STDIO mode
- **WHEN** boris is started with `--transport=stdio --port=9000`
- **THEN** the server does not listen on any TCP port

### Requirement: Health check endpoint
In HTTP mode, the server SHALL expose a `GET /health` endpoint that returns HTTP 200 with a JSON body indicating the server is ready. This endpoint is separate from the MCP handler.

#### Scenario: Health check returns 200
- **WHEN** `GET /health` is requested on a running HTTP-mode server
- **THEN** the response is HTTP 200 with a JSON body

#### Scenario: Health check not available in STDIO mode
- **WHEN** boris is running in STDIO mode
- **THEN** no HTTP endpoints are available

### Requirement: CLI configuration via kong
The server SHALL use `github.com/alecthomas/kong` for CLI argument parsing. Configuration SHALL be defined as a Go struct with kong tags. Each flag SHALL have a corresponding environment variable binding via kong's `env` tag.

#### Scenario: Flag takes precedence over env var
- **WHEN** `--port=9000` is set and `BORIS_PORT=8000` is in the environment
- **THEN** the server uses port 9000

#### Scenario: Env var used when flag is absent
- **WHEN** no `--port` flag is provided and `BORIS_PORT=8000` is in the environment
- **THEN** the server uses port 8000

### Requirement: Supported CLI flags
The server SHALL support the following flags:

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--port` | `BORIS_PORT` | `8080` | Listen port (HTTP mode) |
| `--transport` | `BORIS_TRANSPORT` | `http` | Transport: `http` or `stdio` |
| `--workdir` | `BORIS_WORKDIR` | `.` | Initial working directory |
| `--timeout` | `BORIS_TIMEOUT` | `120` | Default bash timeout (seconds). Converted to milliseconds for the tool parameter. |
| `--allow-dir` | `BORIS_ALLOW_DIRS` | (none) | Allowed directories (repeatable flag, comma-sep env) |
| `--deny-dir` | `BORIS_DENY_DIRS` | (none) | Denied dirs/patterns (repeatable flag, comma-sep env) |
| `--no-bash` | `BORIS_NO_BASH` | `false` | Disable bash tool |
| `--max-file-size` | `BORIS_MAX_FILE_SIZE` | `10MB` | Max file size for view/create |
| `--anthropic-compat` | `BORIS_ANTHROPIC_COMPAT` | `false` | Expose combined `str_replace_editor` tool schema |

#### Scenario: Invalid transport value
- **WHEN** boris is started with `--transport=websocket`
- **THEN** boris exits with an error message listing valid transport options

#### Scenario: Repeatable allow-dir flag
- **WHEN** boris is started with `--allow-dir=/src --allow-dir=/tests`
- **THEN** both `/src` and `/tests` are in the allow list

#### Scenario: Comma-separated env var
- **WHEN** `BORIS_ALLOW_DIRS=/src,/tests` is set and no `--allow-dir` flags are provided
- **THEN** both `/src` and `/tests` are in the allow list

#### Scenario: Timeout flag in seconds converted for tool
- **WHEN** boris is started with `--timeout=60`
- **THEN** the bash tool's default timeout is 60000 milliseconds

### Requirement: Grep tool registered alongside existing tools
The `grep` tool SHALL be registered in `RegisterAll` alongside existing tools. It SHALL be available regardless of the `--anthropic-compat` flag — grep is always a separate tool, not part of the combined `str_replace_editor` tool. When `--no-bash` is set, the grep tool SHALL still be available (it is a file tool, not a bash tool).

When `--anthropic-compat` is set, the grep tool's JSON schema SHALL use Claude Code's exact parameter names (`glob`, `-i`, `-n`, `-A`, `-B`, `-C`). In normal mode, the schema SHALL use descriptive MCP parameter names (`include`, `case_insensitive`, `line_numbers`, `context_before`, `context_after`, `context`).

#### Scenario: Grep in split mode tool list
- **WHEN** boris is started without `--anthropic-compat`
- **THEN** the tool list contains `bash`, `task_output`, `view`, `str_replace`, `create_file`, `grep`, and `find`

#### Scenario: Grep in anthropic-compat mode tool list
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the tool list contains `bash`, `task_output`, `str_replace_editor`, `grep`, and `Glob`

#### Scenario: Grep available with --no-bash
- **WHEN** boris is started with `--no-bash`
- **THEN** the tool list contains `view`, `str_replace`, `create_file`, `grep`, and `find` (no `bash` or `task_output`)

#### Scenario: Grep schema uses compat parameter names
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the grep tool schema lists `glob`, `-i`, `-n`, `-A`, `-B`, `-C` as parameter names

### Requirement: Find tool registered alongside existing tools
The `find` tool SHALL be registered in `RegisterAll` alongside existing tools. It SHALL be available regardless of the `--anthropic-compat` flag. When `--no-bash` is set, the find tool SHALL still be available (it is a file tool, not a bash tool).

When `--anthropic-compat` is set, the tool SHALL be registered as `Glob` with Claude Code's exact parameter schema (pattern, path only). In normal mode, the tool SHALL be registered as `find` with an additional optional `type` parameter.

#### Scenario: Find in split mode tool list
- **WHEN** boris is started without `--anthropic-compat`
- **THEN** the tool list contains `find` as a separate tool

#### Scenario: Find in anthropic-compat mode tool list
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the tool list contains `Glob` (not `find`)

#### Scenario: Find available with --no-bash
- **WHEN** boris is started with `--no-bash`
- **THEN** the tool list contains `find` (or `Glob` in compat mode)

#### Scenario: Find schema in compat mode has no type parameter
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the `Glob` tool schema has only `pattern` and `path` parameters, no `type`

### Requirement: Server implementation identity
The server SHALL identify itself with `Name: "boris"` and a version string in the MCP `Implementation` metadata.

#### Scenario: Server identifies itself
- **WHEN** a client connects and requests server info
- **THEN** the response includes `name: "boris"` and a version string

### Requirement: Static binary build
The binary SHALL be built with `CGO_ENABLED=0` to produce a statically-linked executable with no external dependencies. The build system SHALL support cross-compilation for linux/amd64, linux/arm64, darwin/amd64, and darwin/arm64.

#### Scenario: Binary runs without shared libraries
- **WHEN** the boris binary is copied to a minimal container (e.g., scratch or distroless)
- **THEN** it executes without errors about missing shared libraries

#### Scenario: Cross-compilation targets
- **WHEN** the build is run for all targets
- **THEN** binaries are produced for linux/amd64, linux/arm64, darwin/amd64, and darwin/arm64

### Requirement: Tool errors use IsError instead of protocol errors
All tool handlers SHALL return operational errors (file not found, string not unique, path denied, empty command, etc.) as `CallToolResult` with `IsError: true` and the error message in the `Content` field. Protocol-level errors (returned as Go errors from the handler) SHALL be reserved for infrastructure failures only (e.g., tool handler panic, request deserialization failure).

#### Scenario: File not found returns IsError
- **WHEN** the `view` tool is called with a path that does not exist
- **THEN** the tool returns a `CallToolResult` with `IsError: true` and content describing the error, not a protocol-level error

#### Scenario: Ambiguous match returns IsError
- **WHEN** `str_replace` finds multiple occurrences of `old_str`
- **THEN** the tool returns a `CallToolResult` with `IsError: true` and content indicating the count of occurrences

#### Scenario: Path denied returns IsError
- **WHEN** a file tool is called with a path outside the allowed directories
- **THEN** the tool returns a `CallToolResult` with `IsError: true` and content indicating access is denied

#### Scenario: Bash exit code is not IsError
- **WHEN** a bash command returns exit code 1
- **THEN** the tool returns a normal `CallToolResult` (not `IsError`) with `exit_code: 1` in the text content

### Requirement: Anthropic-compatible combined tool schema
When `--anthropic-compat` is enabled, the server SHALL register a single `str_replace_editor` tool instead of the separate `view`, `str_replace`, and `create_file` tools. The combined tool SHALL accept a `command` parameter with values `view`, `str_replace`, or `create`, and dispatch to the corresponding tool logic. The `bash` tool SHALL always be registered separately regardless of this flag.

#### Scenario: Split tools when anthropic-compat disabled
- **WHEN** boris is started without `--anthropic-compat`
- **THEN** the tool list contains `view`, `str_replace`, and `create_file` as separate tools

#### Scenario: Combined tool when anthropic-compat enabled
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the tool list contains `str_replace_editor` (a single combined tool) and `bash`, but not `view`, `str_replace`, or `create_file` as separate tools

#### Scenario: Combined tool dispatches view
- **WHEN** `str_replace_editor` is called with `command: "view"` and `path: "src/main.go"`
- **THEN** the result is identical to calling the `view` tool directly with the same `path`

#### Scenario: Combined tool dispatches str_replace
- **WHEN** `str_replace_editor` is called with `command: "str_replace"`, `path`, `old_str`, and `new_str`
- **THEN** the result is identical to calling the `str_replace` tool directly with the same parameters

#### Scenario: Combined tool dispatches create
- **WHEN** `str_replace_editor` is called with `command: "create"` and `path` and `file_text`
- **THEN** the result is identical to calling the `create_file` tool with `path` and `content` set to `file_text`

#### Scenario: Combined tool with invalid command
- **WHEN** `str_replace_editor` is called with `command: "delete"`
- **THEN** the tool returns an error via `IsError` indicating the command is not recognized

### Requirement: Shell detection at startup
The server SHALL detect the available shell at startup by checking if `/bin/bash` exists. If it does, `/bin/bash` SHALL be used for bash tool command execution. Otherwise, `/bin/sh` SHALL be used as a fallback. The detected shell path SHALL be logged at startup.

#### Scenario: Bash available
- **WHEN** boris starts on a system where `/bin/bash` exists
- **THEN** the bash tool uses `/bin/bash -c` for command execution

#### Scenario: Bash not available
- **WHEN** boris starts on a minimal container where only `/bin/sh` exists
- **THEN** the bash tool uses `/bin/sh -c` for command execution
