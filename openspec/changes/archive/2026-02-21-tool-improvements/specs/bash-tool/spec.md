## MODIFIED Requirements

### Requirement: Execute shell commands
The `bash` tool SHALL accept a `command` string parameter (required), a `timeout` integer parameter (optional, in milliseconds, default 120000, max 600000), and a `run_in_background` boolean parameter (optional, default false). It SHALL execute the command via the detected shell (`/bin/bash` if available, otherwise `/bin/sh`) and return `stdout`, `stderr`, and `exit_code` in the result.

#### Scenario: Simple command execution
- **WHEN** the tool is called with `command: "echo hello"`
- **THEN** the result contains `stdout: "hello\n"`, `stderr: ""`, and `exit_code: 0`

#### Scenario: Command with non-zero exit
- **WHEN** the tool is called with `command: "exit 42"`
- **THEN** the result contains `exit_code: 42` and the tool call does not return an error (exit code is data, not a tool failure)

#### Scenario: Command producing stderr
- **WHEN** the tool is called with `command: "echo err >&2"`
- **THEN** the result contains `stderr: "err\n"` and `exit_code: 0`

#### Scenario: Timeout in milliseconds
- **WHEN** the tool is called with `timeout: 5000`
- **THEN** the command is allowed to run for 5 seconds before being terminated

#### Scenario: Timeout max cap
- **WHEN** the tool is called with `timeout: 900000` (15 minutes)
- **THEN** the timeout is clamped to 600000 (10 minutes)

#### Scenario: Default timeout
- **WHEN** the tool is called without a `timeout` parameter
- **THEN** the default timeout of 120000ms (2 minutes) is used, or the value configured via `--timeout` CLI flag (converted from seconds to milliseconds)

#### Scenario: Shell selection prefers bash
- **WHEN** `/bin/bash` exists on the system
- **THEN** the command is executed via `/bin/bash -c`

#### Scenario: Shell selection falls back to sh
- **WHEN** `/bin/bash` does not exist on the system
- **THEN** the command is executed via `/bin/sh -c`

### Requirement: Working directory persistence via pwd sentinel
The bash tool SHALL track the working directory across calls within a session. Each command SHALL be executed as `cd <tracked_cwd> && <user_command> ; echo '<sentinel>' ; pwd`. The sentinel SHALL include a random nonce generated per session (e.g., `__BORIS_CWD_a3f7c210__`) to prevent collision with user output. After execution, the tool SHALL parse the sentinel and pwd output from the end of stdout to determine the new working directory. The sentinel and pwd lines SHALL be stripped from the returned stdout.

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

### Requirement: Timeout with process tree cleanup
The bash tool SHALL terminate the entire process group (not just the parent process) when a command exceeds the timeout. On timeout, the tool SHALL first send SIGTERM to the process group, wait up to 5 seconds for graceful shutdown, then send SIGKILL if the process has not exited. The tool SHALL use the process group ID (PGID) for signal delivery. Any stdout/stderr captured before the timeout SHALL be included in the response. The response SHALL indicate that the command was killed due to timeout.

#### Scenario: Command exceeds timeout
- **WHEN** the tool is called with `command: "sleep 300"` and `timeout: 1000`
- **THEN** the tool returns within approximately 1 second with a response indicating timeout, and the sleep process and any children are terminated

#### Scenario: Partial output preserved on timeout
- **WHEN** the tool is called with a command that prints output then hangs, and the command exceeds the timeout
- **THEN** the response includes the output produced before the timeout

#### Scenario: SIGTERM allows graceful shutdown
- **WHEN** a command has a SIGTERM handler that writes cleanup output and exits within 5 seconds
- **THEN** the cleanup output is captured and the process exits gracefully without SIGKILL

#### Scenario: SIGKILL after grace period
- **WHEN** a command ignores SIGTERM and does not exit within 5 seconds of the timeout
- **THEN** the tool sends SIGKILL to the process group to force termination

## ADDED Requirements

### Requirement: Output truncation
The bash tool SHALL truncate stdout and stderr independently at 30,000 characters. When output is truncated, the tool SHALL append a message indicating the total character count and that output was truncated.

#### Scenario: Output within limit
- **WHEN** a command produces 10,000 characters of stdout
- **THEN** the full output is returned without truncation

#### Scenario: Stdout exceeds limit
- **WHEN** a command produces 50,000 characters of stdout
- **THEN** the result contains the first 30,000 characters followed by a truncation message indicating the total was 50,000 characters

#### Scenario: Stderr truncated independently
- **WHEN** a command produces 40,000 characters of stderr and 5,000 characters of stdout
- **THEN** stderr is truncated at 30,000 characters while stdout is returned in full

#### Scenario: Truncation happens after sentinel parsing
- **WHEN** a command produces output exceeding 30,000 characters and the sentinel is at the end of stdout
- **THEN** the sentinel is parsed correctly for cwd tracking before truncation is applied to the cleaned output

### Requirement: Background command execution
When `run_in_background` is true, the bash tool SHALL start the command, store a reference in session state, and return immediately with a `task_id` string in the response. The command SHALL continue running in the background. A maximum of 10 concurrent background tasks per session SHALL be enforced.

#### Scenario: Background command returns immediately
- **WHEN** the tool is called with `command: "sleep 60"` and `run_in_background: true`
- **THEN** the tool returns immediately with a `task_id` in the response text, without waiting for the command to complete

#### Scenario: Background task limit
- **WHEN** 10 background tasks are already running and a new background command is requested
- **THEN** the tool returns an error indicating the maximum concurrent background task limit has been reached

#### Scenario: Background task cwd tracking
- **WHEN** a background command includes `cd /tmp` and completes
- **THEN** the session's working directory is NOT updated (background commands do not affect the session cwd)

### Requirement: Retrieve background task output
A `task_output` tool SHALL accept a `task_id` string parameter (required). It SHALL return the current output and status of the background task.

#### Scenario: Task still running
- **WHEN** `task_output` is called for a task that is still executing
- **THEN** the result contains any stdout/stderr captured so far and `status: running`

#### Scenario: Task completed
- **WHEN** `task_output` is called for a task that has finished
- **THEN** the result contains the final stdout/stderr, exit code, and `status: completed`

#### Scenario: Task completed cleanup
- **WHEN** `task_output` is called for a completed task and its output is returned
- **THEN** the task is removed from session state (single-read semantics)

#### Scenario: Unknown task ID
- **WHEN** `task_output` is called with a `task_id` that does not exist
- **THEN** the tool returns an error via `IsError` indicating the task was not found

### Requirement: Streaming via progress notifications
For foreground bash commands, the tool SHALL send MCP progress notifications with incremental stdout/stderr content as it is produced. The final `CallToolResult` SHALL still contain the complete (possibly truncated) output. Streaming is a progressive enhancement — clients that do not support progress notifications SHALL still receive the full result.

#### Scenario: Progress notifications sent during execution
- **WHEN** a foreground command produces output over several seconds
- **THEN** the server sends progress notifications containing incremental output lines as they are produced

#### Scenario: Final result is self-contained
- **WHEN** a client does not support or ignores progress notifications
- **THEN** the final tool result contains all stdout/stderr (subject to truncation) and the client loses no information
