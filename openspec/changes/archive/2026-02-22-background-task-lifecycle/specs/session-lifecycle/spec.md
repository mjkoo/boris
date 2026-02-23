## ADDED Requirements

### Requirement: Session close kills all background tasks
`session.Session` SHALL provide a `Close()` method that terminates all running background tasks. For each task whose `Done` channel is not yet closed, `Close()` SHALL send SIGTERM to the process group (`-pgid`), wait up to 5 seconds for the process to exit, then send SIGKILL to the process group if it is still running. After all tasks are terminated, the task map SHALL be cleared. `Close()` SHALL be idempotent — calling it multiple times SHALL have no additional effect.

#### Scenario: Close kills running background tasks
- **WHEN** a session has 3 running background tasks and `Close()` is called
- **THEN** all 3 processes receive SIGTERM, and after up to 5 seconds any survivors receive SIGKILL, and the session's task count becomes 0

#### Scenario: Close is idempotent
- **WHEN** `Close()` is called twice on the same session
- **THEN** the second call returns immediately with no effect and no error

#### Scenario: Close skips already-completed tasks
- **WHEN** a session has 2 background tasks, one completed and one running, and `Close()` is called
- **THEN** only the running task's process group receives SIGTERM; the completed task is simply removed from the map

#### Scenario: No new tasks after close
- **WHEN** `Close()` has been called on a session and `AddTask()` is subsequently called
- **THEN** `AddTask()` SHALL return an error indicating the session is closed

### Requirement: Session registry maps SDK session IDs to Boris sessions
A `SessionRegistry` type SHALL provide a concurrent-safe mapping from go-sdk session ID strings to Boris `*Session` instances. It SHALL expose `Register(id string, sess *Session)` to add a mapping and `CloseAndRemove(id string)` to close the session and remove it from the registry. `CloseAndRemove` on an unknown ID SHALL be a no-op.

#### Scenario: Register and close via registry
- **WHEN** a session is registered with ID "abc" and `CloseAndRemove("abc")` is called
- **THEN** the session's `Close()` method is invoked and the mapping is removed from the registry

#### Scenario: CloseAndRemove on unknown ID is no-op
- **WHEN** `CloseAndRemove("unknown-id")` is called on an empty registry
- **THEN** no error occurs and no session is affected

#### Scenario: Concurrent registration and cleanup
- **WHEN** multiple goroutines concurrently register sessions and call `CloseAndRemove`
- **THEN** no data races occur (verified by the Go race detector)

### Requirement: EventStore bridges SDK session close to Boris cleanup
A minimal `mcp.EventStore` implementation SHALL be provided whose `SessionClosed(ctx, sessionID)` method calls `SessionRegistry.CloseAndRemove(sessionID)`. The `Open`, `Append`, and `After` methods SHALL be no-ops that return nil errors (or empty iterators for `After`). This EventStore SHALL be passed via `StreamableHTTPOptions.EventStore` in HTTP mode.

#### Scenario: SDK session timeout triggers Boris cleanup
- **WHEN** an HTTP client creates a session, starts a background task, and then disconnects, and the session idle timeout elapses
- **THEN** the SDK closes the session, which calls `EventStore.SessionClosed`, which calls `SessionRegistry.CloseAndRemove`, which calls `Session.Close()`, which kills the background task's process group

#### Scenario: SDK session DELETE triggers Boris cleanup
- **WHEN** an HTTP client sends a DELETE request to end its session after starting a background task
- **THEN** the same cleanup chain fires and the background task is killed

### Requirement: STDIO session cleanup on shutdown
In STDIO mode, `Session.Close()` SHALL be called when the server shuts down (context cancellation from SIGINT/SIGTERM). This SHALL be wired via `defer sess.Close()` or equivalent in `runSTDIO`.

#### Scenario: STDIO shutdown kills background tasks
- **WHEN** boris is running in STDIO mode with active background tasks and receives SIGTERM
- **THEN** `Session.Close()` is called, killing all background task process groups before the process exits

### Requirement: Lazy session registration from tool handlers
In HTTP mode, the bash tool handler and task_output handler SHALL register the SDK-session → Boris-session mapping on first invocation. Registration SHALL use `req.Session.ID()` to obtain the SDK session ID and SHALL be guarded by `sync.Once` to execute at most once per session.

#### Scenario: First bash call registers session
- **WHEN** an HTTP client's first tool call is a bash command
- **THEN** the session is registered in the `SessionRegistry` with the SDK session ID, and subsequent bash calls on the same session do not re-register

#### Scenario: Session cleanup works after registration
- **WHEN** an HTTP client calls a bash tool (triggering registration), starts a background task, and then disconnects
- **THEN** the session timeout fires, `EventStore.SessionClosed` finds the session in the registry, and `Session.Close()` kills the background task

#### Scenario: No registration if only file tools used
- **WHEN** an HTTP client only uses file tools (view, create, grep, find) and never calls bash
- **THEN** no entry is added to the `SessionRegistry`, and session timeout simply finds no matching entry (no-op cleanup)
