## MODIFIED Requirements

### Requirement: Streamable HTTP transport
When `--transport=http` (the default), the server SHALL use `mcp.NewStreamableHTTPHandler` to serve MCP over HTTP. The server SHALL listen on the port specified by `--port` (default 8080). The `getServer` factory passed to the handler SHALL create a new `mcp.Server` instance per MCP session, with tools registered via `tools.RegisterAll` bound to a new `session.Session`. Process-wide configuration (path resolver, shell path, max file size, tool config) SHALL be captured by the factory closure and reused across sessions.

#### Scenario: HTTP server starts on default port
- **WHEN** boris is started with `--transport=http` and no `--port` flag
- **THEN** the server listens on port 8080 and accepts MCP connections

#### Scenario: HTTP server starts on custom port
- **WHEN** boris is started with `--port=9000`
- **THEN** the server listens on port 9000

#### Scenario: Each HTTP session gets its own server instance
- **WHEN** two clients connect to the HTTP server with different `Mcp-Session-Id` values
- **THEN** each connection is backed by a separate `mcp.Server` with independently registered tool handlers
