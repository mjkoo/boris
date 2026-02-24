## MODIFIED Requirements

### Requirement: Streamable HTTP transport
When `--transport=http` (the default), the server SHALL use `mcp.NewStreamableHTTPHandler` to serve MCP over HTTP. The server SHALL listen on the port specified by `--port` (default 8080). The `getServer` factory passed to the handler SHALL create a new `mcp.Server` instance per MCP session, passing `&mcp.ServerOptions{Instructions: cfg.instructions}` as the options argument. Tools SHALL be registered via `tools.RegisterAll` bound to a new `session.Session`. Process-wide configuration (path resolver, shell path, max file size, tool config, instructions) SHALL be captured by the factory closure and reused across sessions. When a bearer token is configured, the `/mcp` route handler SHALL be wrapped with authentication middleware before registration on the serve mux.

#### Scenario: HTTP server starts on default port
- **WHEN** boris is started with `--transport=http` and no `--port` flag
- **THEN** the server listens on port 8080 and accepts MCP connections

#### Scenario: HTTP server starts on custom port
- **WHEN** boris is started with `--port=9000`
- **THEN** the server listens on port 9000

#### Scenario: Each HTTP session gets its own server instance
- **WHEN** two clients connect to the HTTP server with different `Mcp-Session-Id` values
- **THEN** each connection is backed by a separate `mcp.Server` with independently registered tool handlers

#### Scenario: HTTP server with token wraps MCP handler in auth middleware
- **WHEN** boris is started with `--transport=http` and a token is configured
- **THEN** the `/mcp` route is wrapped with bearer token authentication middleware and the `/health` route remains unauthenticated

#### Scenario: HTTP server passes instructions via ServerOptions
- **WHEN** boris is started with `--transport=http` and `--workdir=/workspace`
- **THEN** each `mcp.Server` created by the factory receives `ServerOptions` with instructions containing the working directory
