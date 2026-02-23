## Context

Background tasks (`run_in_background: true`) launch child processes that outlive the tool call that created them. The current architecture has no lifecycle management: tasks run until they exit naturally or the boris process terminates. In HTTP mode, each session can hold up to 10 background tasks. When a client disconnects, those tasks become orphans — still running, consuming resources, with no one to retrieve their output.

The go-sdk (v1.3.1) manages HTTP session lifecycle internally via `StreamableHTTPHandler`. It provides:
- `StreamableHTTPOptions.SessionTimeout` — closes sessions after an idle duration
- `EventStore` interface — `SessionClosed(sessionID)` is called when a session terminates (timeout, client DELETE, or connection drop)
- `ServerSession.ID()` — accessible from tool handlers via `req.Session.ID()`
- `ServerSessionOptions.onClose` — exists but is unexported; cannot be used directly

The `onClose` callback is set internally by `StreamableHTTPHandler` and chains into `streamableServerConn.Close()`, which calls `EventStore.SessionClosed()` if an EventStore is configured. This is our hook.

## Goals / Non-Goals

**Goals:**
- Background tasks are killed when their session ends (disconnect, timeout, DELETE)
- STDIO mode cleans up background tasks on shutdown
- Cleanup uses the same SIGTERM → SIGKILL escalation pattern as foreground timeouts
- An optional safety-net timeout prevents any single background task from running indefinitely
- No changes to the go-sdk; uses only public API

**Non-Goals:**
- Stream resumption (EventStore is used purely for the `SessionClosed` hook)
- Persisting background task output across session reconnects
- Changing single-read cleanup semantics for task output retrieval
- Adding a `--session-timeout` CLI flag (hardcode a reasonable default for now)

## Decisions

### 1. Session.Close() as the core cleanup primitive

Add `Close()` to `session.Session` that iterates all tracked background tasks, sends SIGTERM to each process group, waits up to 5 seconds, then sends SIGKILL to survivors. Idempotent via `sync.Once`. Clears the task map after cleanup.

**Rationale**: Centralizes cleanup in one method that both transports can call. The SIGTERM → SIGKILL pattern matches the existing foreground timeout behavior (`bash.go:83-89`), keeping the codebase consistent. Process group killing (`-pgid`) ensures child processes spawned by the background command are also cleaned up.

**Alternative considered**: Kill only the direct process (`cmd.Process.Kill()`). Rejected because background commands may spawn child processes that would become orphans.

### 2. SessionRegistry for SDK ↔ Boris session mapping

Introduce a `SessionRegistry` type (concurrent-safe map) that associates go-sdk session IDs with Boris `*Session` instances. The registry provides `Register(id, sess)` and `CloseAndRemove(id)`.

**Rationale**: The go-sdk creates session IDs internally and doesn't expose them at factory-creation time. We need a way to look up the Boris session when `EventStore.SessionClosed(sessionID)` fires. A registry decouples the session lookup from the EventStore implementation.

**Alternative considered**: Embedding the registry in the Session struct (e.g., `Session.sdkSessionID`). Rejected because it couples Session to HTTP-specific concerns. The registry is only used in HTTP mode; STDIO doesn't need it.

### 3. Lazy registration via tool handlers

Tool handlers register the SDK-session → Boris-session mapping on first invocation by calling `registry.Register(req.Session.ID(), sess)`, guarded by `sync.Once`.

**Rationale**: The factory function that creates the Boris session (`runHTTP`'s closure) doesn't have access to the SDK session ID — it's assigned later by `server.Connect()`. The earliest point where both the Boris session and SDK session ID are available is inside a tool handler. Using `sync.Once` ensures registration happens exactly once per session with no overhead on subsequent calls.

If no tool is ever called on a session, no registration happens — but that's fine because there are no background tasks to clean up.

**Alternative considered**: Intercepting at the `EventStore.Open()` level. Rejected because `Open()` receives a session ID but has no reference to the Boris session. Correlating them would require a fragile pending-queue mechanism.

### 4. Minimal EventStore for SessionClosed hook

Implement the `mcp.EventStore` interface with no-op methods for `Open`, `Append`, and `After`, and a meaningful `SessionClosed` that calls `registry.CloseAndRemove(sessionID)`.

**Rationale**: We don't need stream resumption — we only need the `SessionClosed` callback. The SDK calls `EventStore.SessionClosed()` when a session terminates for any reason. By providing a minimal implementation, we get the hook without taking on stream resumption complexity. The no-op `After()` means clients that attempt stream resumption simply get no replayed events, which is acceptable.

**Alternative considered**: Using `SessionTimeout` alone without an EventStore. Rejected because without an EventStore, `SessionClosed` is never called (`streamableServerConn.Close()` checks `if c.eventStore != nil`). The timeout would close the SDK session but Boris would have no notification to trigger `Session.Close()`.

### 5. Hardcoded session idle timeout

Configure `StreamableHTTPOptions.SessionTimeout` to 10 minutes. No CLI flag for now.

**Rationale**: 10 minutes is long enough that active clients with pauses between requests won't be disconnected, but short enough that abandoned sessions are cleaned up promptly. The go-sdk resets the timer on each new HTTP request, so active sessions are never timed out. A CLI flag adds complexity without clear demand — can be added later if needed.

### 6. Optional background task safety-net timeout

Add `--bg-timeout` flag (default 0, meaning disabled). When set, each background task gets a `time.AfterFunc` that kills it after the specified duration using the same SIGTERM → SIGKILL pattern.

**Rationale**: The primary cleanup mechanism is session close. The per-task timeout is defense-in-depth for edge cases: a session that somehow avoids cleanup, or a task that should have been foreground but was accidentally backgrounded. Disabled by default because the session lifecycle handles the normal case. When enabled, uses a generous value (operator's choice) — this is a safety net, not the primary mechanism.

**Alternative considered**: Always-on 30-minute default. Rejected per the principle that we should solve the foundational problem (session cleanup) rather than design around it with timeouts.

### 7. Registration scoped to bash tool handlers only

Only the `bashHandler` and `taskOutputHandler` perform lazy session registration, not all tool handlers.

**Rationale**: Only these handlers interact with background tasks. File tools (view, create, str_replace, grep, find) don't create long-lived resources that need cleanup. Adding registration to all handlers would be unnecessary overhead. If a session only uses file tools, there's nothing to clean up on session end — the session object is lightweight and will be GC'd.

## Risks / Trade-offs

**EventStore no-ops may confuse future maintainers** → Document clearly that the EventStore is used solely for the `SessionClosed` hook, not for stream resumption. Name the type explicitly (e.g., `sessionCleanupStore`).

**Lazy registration has a window where cleanup can't fire** → Between session creation and first bash tool call, `EventStore.SessionClosed` would find no Boris session in the registry. This is fine: if no bash tool was called, there are no background tasks to clean up.

**SDK `EventStore.SessionClosed` docs say "A store cannot rely on this method being called for cleanup"** → The optional `--bg-timeout` safety net handles this edge case. Additionally, the `SessionTimeout` is the primary trigger for `SessionClosed`, so in practice it will be called for idle sessions.

**10-minute hardcoded timeout may not suit all deployments** → Acceptable for now. Can be promoted to a CLI flag in a future change if operators request it.

**Process group kill may fail if process already exited** → `syscall.Kill` on a non-existent process group returns an error, which we ignore (same pattern as existing foreground timeout code).
