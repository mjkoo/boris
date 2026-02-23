## MODIFIED Requirements

### Requirement: Background command execution
When `run_in_background` is true, the bash tool SHALL start the command, store a reference in session state, and return immediately with a `task_id` string in the response. The command SHALL continue running in the background. A maximum of 10 concurrent background tasks per session SHALL be enforced. If the session has been closed, the tool SHALL return an error instead of starting the command.

When `--bg-timeout` is set to a positive value, a safety-net timer SHALL be started for the background task. If the timer expires before the task completes, the task's process group SHALL receive SIGTERM followed by SIGKILL after 5 seconds. The timer SHALL be cancelled if the task completes before it fires.

In HTTP mode, the bash tool handler SHALL perform lazy session registration on first invocation by calling `registry.Register(req.Session.ID(), sess)` via `sync.Once`, enabling session lifecycle cleanup.

#### Scenario: Background command returns immediately
- **WHEN** the tool is called with `command: "sleep 60"` and `run_in_background: true`
- **THEN** the tool returns immediately with a `task_id` in the response text, without waiting for the command to complete

#### Scenario: Background task limit
- **WHEN** 10 background tasks are already running and a new background command is requested
- **THEN** the tool returns an error indicating the maximum concurrent background task limit has been reached

#### Scenario: Background task cwd tracking
- **WHEN** a background command includes `cd /tmp` and completes
- **THEN** the session's working directory is NOT updated (background commands do not affect the session cwd)

#### Scenario: Background tasks isolated between connections
- **WHEN** client A starts a background task receiving `task_id: abc123`, and client B calls `task_output` with `task_id: abc123`
- **THEN** client B receives a "task not found" error

#### Scenario: Background command rejected after session close
- **WHEN** a session's `Close()` has been called and a bash command with `run_in_background: true` is submitted
- **THEN** the tool returns an error indicating the session is closed and does not start the command
