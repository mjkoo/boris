## 1. Session Close

- [x] 1.1 Add `Close()` method to `session.Session` with SIGTERM → 5s → SIGKILL escalation for all running tasks, idempotent via `sync.Once`, clears task map
- [x] 1.2 Make `AddTask()` return error when session is closed
- [x] 1.3 Write tests: Close kills running tasks, Close is idempotent, Close skips completed tasks, AddTask rejected after Close, concurrent Close calls are safe

## 2. Session Registry

- [x] 2.1 Implement `SessionRegistry` type with `Register(id, sess)` and `CloseAndRemove(id)` methods, concurrent-safe via `sync.Mutex`
- [x] 2.2 Write tests: register and close, CloseAndRemove on unknown ID is no-op, concurrent access is race-free

## 3. EventStore

- [x] 3.1 Implement minimal `mcp.EventStore` (`sessionCleanupStore`) with no-op Open/Append/After and SessionClosed that calls `registry.CloseAndRemove`
- [x] 3.2 Write tests: SessionClosed triggers registry cleanup, no-op methods return nil

## 4. Lazy Session Registration

- [x] 4.1 Add session registration callback to `tools.Config` (e.g., `RegisterSession func(sessionID string)`) so tool handlers can trigger registration without depending on `SessionRegistry` directly
- [x] 4.2 Wire `bashHandler` and `taskOutputHandler` to call the registration callback on first invocation via `sync.Once`, using `req.Session.ID()`
- [x] 4.3 Write tests: registration callback fires on first bash call, does not fire on subsequent calls, handles nil callback (STDIO mode)

## 5. HTTP Wiring

- [x] 5.1 In `runHTTP`, create a `SessionRegistry` and construct the `sessionCleanupStore` EventStore
- [x] 5.2 Pass `&mcp.StreamableHTTPOptions{SessionTimeout: 10 * time.Minute, EventStore: store}` to `NewStreamableHTTPHandler`
- [x] 5.3 In the `getServer` factory, capture the registry and pass a registration closure through `tools.Config` that calls `registry.Register(sessionID, sess)`
- [x] 5.4 Write integration test: HTTP client starts background task, session times out or DELETE is sent, background task process is killed

## 6. STDIO Wiring

- [x] 6.1 Add `defer sess.Close()` in `runSTDIO` after session creation
- [x] 6.2 Write test: STDIO shutdown triggers session close (verify via background task cleanup)

## 7. Background Task Timeout (Safety Net)

- [x] 7.1 Add `--bg-timeout` CLI flag (int, seconds, default 0 = disabled) and `BgTimeout` field to `tools.Config`
- [x] 7.2 In `runBackground`, when `BgTimeout > 0`, start `time.AfterFunc` with SIGTERM → SIGKILL pattern, cancel timer on task completion
- [x] 7.3 Add `TimedOut` field to `BackgroundTask`, set it when safety-net fires
- [x] 7.4 In `taskOutputHandler`, include timeout message in response when `task.TimedOut` is true
- [x] 7.5 Write tests: task killed after timeout, task output reports timeout, timer cancelled on early completion, no timer when bg-timeout is 0
