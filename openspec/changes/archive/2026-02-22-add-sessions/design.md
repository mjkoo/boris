## Context

Boris currently creates a single `session.Session` and `mcp.Server` at startup. The go-sdk's `StreamableHTTPHandler` accepts a `getServer func(*http.Request) *mcp.Server` factory that is called **once per new MCP session** (identified by `Mcp-Session-Id` header). Today, this factory ignores the request and returns the shared server, so all connections share one cwd and one background task pool.

The `session.Session` type is already well-encapsulated — it holds cwd, nonce, and background tasks behind a mutex. No changes to the Session type are needed. The change is purely in how and when sessions are created.

## Goals / Non-Goals

**Goals:**

- Each MCP connection (unique `Mcp-Session-Id`) gets independent cwd and background task state
- Reconnects with the same `Mcp-Session-Id` reuse the existing session (cwd preserved)
- STDIO behavior is unchanged (single connection, single session)
- No new CLI flags — this is the default behavior

**Non-Goals:**

- Named/user-specified session IDs (go-sdk generates them automatically)
- Session listing, inspection, or management APIs
- Per-session environment variable isolation (shell commands already get fresh `sh -c` each time)
- Persistent shell sessions (potential future work, orthogonal to this change)
- Session persistence across boris restarts

## Decisions

### 1. Create server + session per connection in `getServer` factory

**Choice**: Move `mcp.NewServer()`, `session.New()`, and `tools.RegisterAll()` into the `getServer` callback.

**Alternative considered**: Keep a single `mcp.Server` and use middleware to swap session context per-request. Rejected because:
- The go-sdk binds tool handlers at server creation time via `mcp.AddTool()`. There's no per-request context injection for tool handlers.
- Creating a server per session is the intended pattern — `getServer` exists for this purpose.
- `mcp.Server` is lightweight (just a handler registry). The cost of creating one per connection is negligible.

**Alternative considered**: Maintain a `map[sessionID]*session.Session` and look up sessions in tool handlers. Rejected because:
- Requires threading session ID through every tool handler
- The go-sdk already manages session lifecycle and maps `Mcp-Session-Id` to server instances — duplicating this mapping adds complexity for no benefit

### 2. Extract shared config resolution to a struct

**Choice**: Move the values computed once at startup (workdir, shell path, resolver, maxFileSize, tool config) into a struct that the `getServer` closure captures. The closure creates new per-connection objects (server, session) but reuses the shared immutable config.

**Rationale**: These values are process-wide and immutable after startup. They should not be recomputed per connection.

### 3. STDIO path unchanged

**Choice**: For `--transport=stdio`, keep the current pattern — create one server and one session at startup, pass to `server.Run()`.

**Rationale**: STDIO has exactly one connection. There's no `getServer` factory in the STDIO path (`server.Run()` takes a transport directly). Per-connection sessions would add no value and require restructuring the STDIO path for no reason.

### 4. No session timeout configuration

**Choice**: Use the go-sdk's default session timeout behavior. Don't add a `--session-timeout` flag.

**Rationale**: The go-sdk handles session cleanup internally. If a session times out and the client reconnects, it gets a fresh session with the initial workdir — which is reasonable. Adding timeout configuration is premature; we can add it later if users report issues.

## Risks / Trade-offs

**[Per-connection server creation overhead]** → Negligible. `mcp.NewServer` is a struct allocation + handler map. `RegisterAll` registers ~6-8 tool handlers. Total: microseconds per new connection.

**[Session loss on timeout]** → If the go-sdk cleans up an idle session and the client reconnects without the old `Mcp-Session-Id`, they get a fresh cwd. Mitigation: this is expected behavior and matches what would happen if boris restarted. Clients should send the session ID header on reconnect (per MCP spec).

**[Background task orphaning]** → If a session is cleaned up while background tasks are still running, the OS processes continue but the task tracking is lost. Mitigation: background tasks already have a 10-task limit and are expected to be short-lived. This is an edge case of an edge case. A future enhancement could kill background tasks on session cleanup.

**[Test isolation]** → Integration tests that create an HTTP test server will now get per-connection sessions. Tests that relied on shared session state between separate HTTP requests will need updating. Mitigation: this is a small test surface and the tests should verify the new (correct) behavior anyway.

## Open Questions

None — the scope is intentionally narrow. The go-sdk does the heavy lifting.
