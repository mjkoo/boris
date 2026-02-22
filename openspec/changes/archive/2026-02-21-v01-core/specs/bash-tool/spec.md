## ADDED Requirements

### Requirement: Execute shell commands
The `bash` tool SHALL accept a `command` string parameter (required) and a `timeout` integer parameter (optional, default 120 seconds). It SHALL execute the command via `/bin/sh -c` and return `stdout`, `stderr`, and `exit_code` in the result.

#### Scenario: Simple command execution
- **WHEN** the tool is called with `command: "echo hello"`
- **THEN** the result contains `stdout: "hello\n"`, `stderr: ""`, and `exit_code: 0`

#### Scenario: Command with non-zero exit
- **WHEN** the tool is called with `command: "exit 42"`
- **THEN** the result contains `exit_code: 42` and the tool call does not return an error (exit code is data, not a tool failure)

#### Scenario: Command producing stderr
- **WHEN** the tool is called with `command: "echo err >&2"`
- **THEN** the result contains `stderr: "err\n"` and `exit_code: 0`

### Requirement: Working directory persistence via pwd sentinel
The bash tool SHALL track the working directory across calls within a session. Each command SHALL be executed as `cd <tracked_cwd> && <user_command> ; echo '<sentinel>' ; pwd`. After execution, the tool SHALL parse the sentinel and pwd output from the end of stdout to determine the new working directory. The sentinel and pwd lines SHALL be stripped from the returned stdout.

#### Scenario: cd changes persist across calls
- **WHEN** the tool is called with `command: "cd /tmp"` followed by `command: "pwd"`
- **THEN** the second call returns stdout containing `/tmp`

#### Scenario: Sentinel is stripped from output
- **WHEN** the tool is called with `command: "echo hello"`
- **THEN** the returned stdout contains `hello` and does not contain the sentinel string or the pwd output

#### Scenario: Missing sentinel preserves previous cwd
- **WHEN** a command is killed by timeout before the sentinel is printed
- **THEN** the tracked working directory remains unchanged from before that command

### Requirement: Initial working directory
The bash tool SHALL use the `--workdir` flag value (default: current directory at startup) as the initial working directory for the session.

#### Scenario: Default working directory
- **WHEN** boris is started without `--workdir` and the first bash command is `pwd`
- **THEN** the result contains the directory boris was launched from

#### Scenario: Custom working directory
- **WHEN** boris is started with `--workdir=/tmp` and the first bash command is `pwd`
- **THEN** the result contains `/tmp`

### Requirement: Timeout with process tree cleanup
The bash tool SHALL kill the entire process group (not just the parent process) when a command exceeds the timeout. The tool SHALL use the process group ID (PGID) to send SIGKILL to all child processes. Any stdout/stderr captured before the timeout SHALL be included in the response. The response SHALL indicate that the command was killed due to timeout.

#### Scenario: Command exceeds timeout
- **WHEN** the tool is called with `command: "sleep 300"` and `timeout: 1`
- **THEN** the tool returns within approximately 1 second with a response indicating timeout, and the sleep process and any children are terminated

#### Scenario: Partial output preserved on timeout
- **WHEN** the tool is called with a command that prints output then hangs, and the command exceeds the timeout
- **THEN** the response includes the output produced before the timeout

### Requirement: Process group isolation
Each bash command SHALL run in its own process group (`Setpgid: true`). This ensures timeout cleanup kills the entire process tree spawned by the command, not just the shell.

#### Scenario: Child processes cleaned up on timeout
- **WHEN** a command spawns background child processes and the command times out
- **THEN** all child processes in the process group are killed

### Requirement: Non-interactive execution
The bash tool SHALL NOT allocate a TTY or support interactive commands. Commands MUST be non-interactive.

#### Scenario: Interactive command fails gracefully
- **WHEN** a command that requires TTY input is executed (e.g., a program waiting on stdin)
- **THEN** the command either times out or exits without hanging the server

### Requirement: Disable bash tool via --no-bash
When boris is started with `--no-bash`, the bash tool SHALL NOT be registered with the MCP server. The tool SHALL not appear in the tool listing.

#### Scenario: Bash tool hidden when disabled
- **WHEN** boris is started with `--no-bash` and a client requests the tool list
- **THEN** the `bash` tool does not appear in the response
