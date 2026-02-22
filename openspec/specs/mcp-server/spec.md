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
| `--timeout` | `BORIS_TIMEOUT` | `120` | Default bash timeout (seconds) |
| `--allow-dir` | `BORIS_ALLOW_DIRS` | (none) | Allowed directories (repeatable flag, comma-sep env) |
| `--deny-dir` | `BORIS_DENY_DIRS` | (none) | Denied dirs/patterns (repeatable flag, comma-sep env) |
| `--no-bash` | `BORIS_NO_BASH` | `false` | Disable bash tool |
| `--max-file-size` | `BORIS_MAX_FILE_SIZE` | `10MB` | Max file size for view/create |

#### Scenario: Invalid transport value
- **WHEN** boris is started with `--transport=websocket`
- **THEN** boris exits with an error message listing valid transport options

#### Scenario: Repeatable allow-dir flag
- **WHEN** boris is started with `--allow-dir=/src --allow-dir=/tests`
- **THEN** both `/src` and `/tests` are in the allow list

#### Scenario: Comma-separated env var
- **WHEN** `BORIS_ALLOW_DIRS=/src,/tests` is set and no `--allow-dir` flags are provided
- **THEN** both `/src` and `/tests` are in the allow list

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
