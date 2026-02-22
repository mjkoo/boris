## MODIFIED Requirements

### Requirement: Working directory persistence via pwd sentinel
The bash tool SHALL track the working directory across calls within a session. Each command SHALL be executed as `cd <tracked_cwd> && <user_command> ; echo '<sentinel>' ; pwd`. The sentinel SHALL include a random nonce generated per session (e.g., `__BORIS_CWD_a3f7c210__`) to prevent collision with user output. After execution, the tool SHALL parse the sentinel and pwd output from the end of stdout to determine the new working directory. The sentinel and pwd lines SHALL be stripped from the returned stdout.

Note: No behavioral change. The existing "per session" scoping is now realized as per-connection rather than per-process, but the tool's contract is identical.

#### Scenario: cd changes persist across calls
- **WHEN** the tool is called with `command: "cd /tmp"` followed by `command: "pwd"`
- **THEN** the second call returns stdout containing `/tmp`

#### Scenario: Sentinel is stripped from output
- **WHEN** the tool is called with `command: "echo hello"`
- **THEN** the returned stdout contains `hello` and does not contain the sentinel string or the pwd output

#### Scenario: Missing sentinel preserves previous cwd
- **WHEN** a command is killed by timeout before the sentinel is printed
- **THEN** the tracked working directory remains unchanged from before that command

#### Scenario: Sentinel nonce prevents collision
- **WHEN** a command outputs text containing `__BORIS_CWD__` (the old sentinel format without nonce)
- **THEN** the parser does not misinterpret it as the sentinel because the actual sentinel includes a session-specific nonce

#### Scenario: CWD isolated between connections
- **WHEN** client A runs `cd /tmp` and client B runs `pwd` on the same boris HTTP server
- **THEN** client B sees the initial `--workdir` value, not `/tmp`

### Requirement: Background command execution
When `run_in_background` is true, the bash tool SHALL start the command, store a reference in session state, and return immediately with a `task_id` string in the response. The command SHALL continue running in the background. A maximum of 10 concurrent background tasks per session SHALL be enforced.

Note: No behavioral change. Background task tracking was already per-session; sessions are now per-connection.

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
