## Why

MCP clients make assumptions about the server's working directory when constructing paths for tool calls. Since Boris exposes coding tools as a standalone MCP server (not embedded in the client), the model has no way to know the initial working directory or path scoping constraints without making a tool call first. The MCP protocol's `instructions` field in the initialize response exists precisely for this — static server context provided to the model at connection time. Boris should use it.

## What Changes

- Build a server instructions string at startup containing the initial working directory and, if configured, the allowed/denied directory lists.
- Pass `&mcp.ServerOptions{Instructions: ...}` to `mcp.NewServer()` instead of `nil`.
- Add accessor methods to `pathscope.Resolver` so the canonicalized allow/deny lists can be read back for inclusion in instructions.

## Capabilities

### New Capabilities
- `server-instructions`: Building and providing the MCP server instructions string containing working directory and path scoping context.

### Modified Capabilities
- `mcp-server`: Server initialization changes to pass `ServerOptions` with instructions to `mcp.NewServer()`.
- `path-scoping`: Adding read accessors to `Resolver` for allow dirs and deny patterns.

## Impact

- `cmd/boris/main.go`: `serverConfig` gains an `opts` or instructions field; both `runHTTP` and `runSTDIO` pass options to `mcp.NewServer()`.
- `internal/pathscope/pathscope.go`: Two new exported methods (`AllowDirs()`, `DenyPatterns()`) on `Resolver`.
- No breaking changes. Existing behavior is unchanged — this only adds the `instructions` field to the initialize response which clients were not receiving before.
