## 1. Refactor main.go for per-connection sessions

- [x] 1.1 Extract a `serverConfig` struct in `main.go` that holds all shared immutable values computed at startup: workdir, shell path, resolver, maxFileSize, `tools.Config`, and `mcp.Implementation`. The `getServer` closure will capture this struct.
- [x] 1.2 Refactor `runHTTP` to accept `serverConfig` instead of a pre-built `*mcp.Server`. The `getServer` factory SHALL create a new `mcp.Server`, `session.Session`, and call `tools.RegisterAll` per connection.
- [x] 1.3 Keep `runSTDIO` unchanged — it creates one server and one session at startup, same as today.

## 2. Tests for per-connection session isolation

- [x] 2.1 Write a regression test: two HTTP clients connecting to the same boris server get independent cwd. Client A runs `cd /tmp`, client B runs `pwd` and sees the initial workdir (not `/tmp`).
- [x] 2.2 Write a regression test: background task IDs are scoped per connection. Client A starts a background task, client B cannot retrieve it via `task_output`.
- [x] 2.3 Write a test: a single client reconnecting with the same `Mcp-Session-Id` preserves cwd across the reconnect.
- [x] 2.4 Verify existing integration tests pass with the new per-connection model. Fix any tests that assumed shared session state between separate HTTP connections.

## 3. Verify and clean up

- [x] 3.1 Run full test suite with race detector (`go test -race ./...`) and confirm no races.
- [x] 3.2 Manual smoke test: start boris in HTTP mode, connect two separate MCP clients (or curl sessions), confirm independent cwd tracking.
