## MODIFIED Requirements

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
