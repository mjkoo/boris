### Requirement: Per-connection session isolation in HTTP mode
When using `--transport=http`, the server SHALL create a new `session.Session` and a new `mcp.Server` instance for each new MCP session (identified by a unique `Mcp-Session-Id`). Each session SHALL have independent working directory tracking, sentinel nonce, and background task state. The `getServer` factory passed to `mcp.NewStreamableHTTPHandler` SHALL create these per-connection objects.

The `StreamableHTTPHandler` SHALL be configured with `StreamableHTTPOptions` including:
- `SessionTimeout` set to 10 minutes, causing idle sessions to be automatically closed by the SDK
- `EventStore` set to a minimal implementation that bridges `SessionClosed` to Boris session cleanup via the `SessionRegistry`

A process-wide `SessionRegistry` SHALL be created in `runHTTP` and shared between the `getServer` factory (which creates Boris sessions) and the `EventStore` (which cleans them up on session close).

#### Scenario: Two clients get independent working directories
- **WHEN** client A connects and runs `cd /tmp`, then client B connects and runs `pwd`
- **THEN** client B's pwd returns the initial `--workdir` value, not `/tmp`

#### Scenario: Two clients get independent background task pools
- **WHEN** client A starts a background task and receives `task_id: abc123`, then client B calls `task_output` with `task_id: abc123`
- **THEN** client B receives a "task not found" error because task IDs are scoped to the session that created them

#### Scenario: Two clients get independent sentinel nonces
- **WHEN** client A and client B each connect and run bash commands
- **THEN** each session uses a different nonce in its pwd sentinel, preventing any possibility of sentinel collision

#### Scenario: Idle session cleaned up after timeout
- **WHEN** an HTTP client creates a session, starts a background task, and then stops sending requests
- **THEN** after the session idle timeout (10 minutes), the SDK closes the session, triggering `EventStore.SessionClosed`, which calls `Session.Close()` to kill all background tasks

#### Scenario: Client DELETE triggers immediate cleanup
- **WHEN** an HTTP client sends a DELETE request to its session endpoint after starting background tasks
- **THEN** the session is closed immediately and all background tasks are killed

### Requirement: Session state preserved across reconnects
When a client reconnects with the same `Mcp-Session-Id` header, the go-sdk SHALL reuse the existing `ServerSession` and its associated `mcp.Server` instance. The boris `session.Session` bound to that server's tool handlers SHALL retain its state (cwd, background tasks).

#### Scenario: CWD survives reconnect with same session ID
- **WHEN** a client runs `cd /tmp`, disconnects, then reconnects with the same `Mcp-Session-Id`
- **THEN** running `pwd` returns `/tmp`

#### Scenario: New session on reconnect without session ID
- **WHEN** a client runs `cd /tmp`, disconnects, then reconnects without an `Mcp-Session-Id` header
- **THEN** a new session is created and `pwd` returns the initial `--workdir` value

### Requirement: STDIO transport uses single session
When using `--transport=stdio`, the server SHALL create one `session.Session` and one `mcp.Server` at startup. The STDIO transport has exactly one connection and does not use the `getServer` factory pattern.

#### Scenario: STDIO behavior unchanged
- **WHEN** boris is started with `--transport=stdio` and a client connects
- **THEN** the client gets a single session with cwd tracking and background tasks, identical to the pre-change behavior

### Requirement: Shared immutable configuration across sessions
Process-wide configuration (workdir initial value, shell path, path resolver, max file size, tool config, anthropic-compat flag) SHALL be resolved once at startup and shared across all sessions. Only `session.Session` and `mcp.Server` instances SHALL be created per connection.

#### Scenario: Path scoping applies to all sessions
- **WHEN** boris is started with `--allow-dir=/workspace` and two clients connect
- **THEN** both clients are restricted to `/workspace` for file tool operations

#### Scenario: Tool configuration consistent across sessions
- **WHEN** boris is started with `--anthropic-compat --disable-tools bash` and two clients connect
- **THEN** both clients see the same tool set (`str_replace_editor`, `grep`, `Glob`)

### Requirement: No session management flags
Per-connection session isolation SHALL be the default behavior. No `--sessions` flag SHALL be exposed. Single-client deployments experience no behavior change because the client's `Mcp-Session-Id` ensures session reuse across reconnects.

#### Scenario: No --sessions flag
- **WHEN** boris is started with `--sessions`
- **THEN** boris exits with an error indicating an unknown flag
