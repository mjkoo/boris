## Why

Boris currently creates a single `session.Session` at startup and shares it across all MCP connections. This means all connected clients share one working directory and one background task pool. The go-sdk's `StreamableHTTPHandler` already creates separate transport-level sessions per client (tracked via `Mcp-Session-Id` header), but boris ignores this by returning the same `mcp.Server` instance from the `getServer` factory. Moving session creation into the factory gives each client independent state while preserving cwd across reconnects — with no configuration flags needed.

## What Changes

- **Per-connection session isolation**: Each new MCP connection (unique `Mcp-Session-Id`) gets its own `session.Session` with independent cwd and background task tracking. The go-sdk calls `getServer` once per new session and reuses it for reconnects with the same session ID, so cwd survives network blips.
- **New server per session**: Instead of a single shared `mcp.Server`, each connection gets its own server instance with tools bound to its own session. Tool registration moves from startup into the `getServer` factory.
- **STDIO unchanged**: STDIO transport has exactly one connection, so behavior is identical to today.
- **Remove `--sessions` flag from scope**: The PRD proposed opt-in multi-session via `--sessions`. Since per-connection sessions are strictly better for all use cases (single-client cwd persists via `Mcp-Session-Id`; multi-client gets isolation), this should be the default behavior with no flag.

## Capabilities

### New Capabilities

- `per-connection-sessions`: Per-connection session isolation — each MCP connection gets independent cwd and background task state, managed by the go-sdk's session lifecycle.

### Modified Capabilities

- `mcp-server`: Server initialization changes — tool registration moves from startup into the per-connection `getServer` factory. HTTP handler setup changes.
- `bash-tool`: No behavioral change, but the session instance backing cwd tracking and background tasks is now per-connection rather than global.

## Session State Inventory

### Explicit state (per `session.Session`)

| Field | Type | Per-connection? | Notes |
|-------|------|-----------------|-------|
| `cwd` | `string` | Yes | Updated via pwd sentinel after each bash command |
| `nonce` | `string` | Yes | Random hex for sentinel `__BORIS_CWD_<nonce>__`, prevents collisions |
| `tasks` | `map[string]*BackgroundTask` | Yes | Up to 10 concurrent background commands, keyed by random task ID |

### Implicit shared state (process-wide, NOT per-session)

| State | Shared? | Notes |
|-------|---------|-------|
| Filesystem | Yes, by design | PRD: "the filesystem is the primary state." Race conditions between sessions are the user's concern. |
| Environment variables | Yes | Inherited from boris process. Every `sh -c` gets the same env. No per-session overrides. |
| Shell state | None persists | Each command is a fresh `sh -c` invocation. No aliases, functions, or variables carry over between commands. |
| Process table | Yes | Background tasks are OS processes. Task IDs are session-scoped, but PIDs are global. |
| Path scoping | Yes | `--allow-dir` / `--deny-dir` is process-wide config. |

### Note: Shell session persistence

Boris runs each bash command as a fresh `/bin/sh -c` invocation — only cwd persists (via sentinel tracking). This differs from the Anthropic bash tool spec, which describes a persistent shell session where environment variables and shell functions survive across commands. However, Claude Code's actual behavior also loses env vars between commands (ref: [anthropics/claude-code#2508](https://github.com/anthropics/claude-code/issues/2508)), so models are trained to work around this by chaining dependent commands with `&&`. Moving to persistent shell sessions is a potential future enhancement orthogonal to this change — the per-connection session model built here is the right foundation for it.

## Impact

- **`cmd/boris/main.go`**: HTTP handler setup refactored — `getServer` factory creates new `mcp.Server` + `session.Session` per connection instead of returning a shared instance. STDIO path unchanged.
- **`internal/tools/tools.go`**: `RegisterAll` called per-connection instead of once at startup. No signature changes needed.
- **`internal/session/session.go`**: No changes to the Session type itself.
- **Integration tests**: Tests that assume a single shared session may need to account for per-connection isolation, or explicitly test multi-client scenarios.
- **No new dependencies**: Relies entirely on existing go-sdk session management (`StreamableHTTPHandler`, `Mcp-Session-Id` header).
- **No breaking changes**: Single-client behavior is identical. Multi-client behavior improves from shared-state to isolated-state.
