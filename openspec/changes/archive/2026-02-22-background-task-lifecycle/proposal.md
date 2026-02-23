## Why

Background tasks (bash commands with `run_in_background: true`) have no cleanup lifecycle. When a client disconnects in HTTP mode, orphaned background tasks run indefinitely with no way to retrieve results and no mechanism to reclaim resources. In STDIO mode, background tasks survive until the process exits but are never explicitly cleaned up. This is a resource leak that scales with usage — every disconnected session can leave up to 10 orphaned processes running.

## What Changes

- Add a `Close()` method to `session.Session` that kills all running background tasks with SIGTERM → SIGKILL escalation, idempotent via `sync.Once`
- Introduce a `SessionRegistry` that maps go-sdk session IDs to Boris sessions, enabling cleanup when the SDK signals session end
- Implement a minimal `mcp.EventStore` that hooks `SessionClosed()` to trigger Boris session cleanup via the registry
- Wire `StreamableHTTPOptions.SessionTimeout` so idle HTTP sessions are automatically closed by the SDK, which chains into our `EventStore.SessionClosed` → `SessionRegistry.Close` → `Session.Close` path
- In STDIO mode, `defer sess.Close()` on shutdown
- Tool handlers register the SDK-session-ID → Boris-session mapping on first invocation via `req.Session.ID()`
- Add an optional safety-net timeout for individual background tasks (`--bg-timeout`, generous default like 4h) as defense-in-depth, not the primary cleanup mechanism

## Capabilities

### New Capabilities
- `session-lifecycle`: Session close/cleanup semantics, session registry for HTTP mode, EventStore integration with the go-sdk, and STDIO shutdown cleanup
- `background-task-timeout`: Optional per-task safety-net timeout with SIGTERM→SIGKILL for background commands

### Modified Capabilities
- `bash-tool`: Background task launch now respects optional timeout; tool handlers perform lazy session registration on first call
- `per-connection-sessions`: HTTP mode now configures `SessionTimeout` and provides an `EventStore` for session-end cleanup; session factory creates sessions registered for lifecycle management

## Impact

- **`internal/session/`**: New `Close()` method on `Session`, new `SessionRegistry` type, possible `onFirstCall` hook or similar for lazy registration
- **`internal/tools/bash.go`**: Background task launch gains optional timer; tool handlers gain session registration call
- **`cmd/boris/main.go`**: HTTP factory wires `SessionRegistry` and passes `StreamableHTTPOptions` with `SessionTimeout` and custom `EventStore`; STDIO path adds `defer sess.Close()`
- **New `internal/session/eventstore.go`** (or similar): Minimal `mcp.EventStore` implementation whose only purpose is bridging `SessionClosed` to the Boris session registry
- **go-sdk dependency**: No changes required — uses existing public API (`EventStore` interface, `StreamableHTTPOptions.SessionTimeout`, `ServerSession.ID()`)
- **No breaking changes**: Existing behavior is preserved; cleanup is additive
