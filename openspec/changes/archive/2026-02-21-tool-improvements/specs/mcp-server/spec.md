## MODIFIED Requirements

### Requirement: Supported CLI flags
The server SHALL support the following flags:

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--port` | `BORIS_PORT` | `8080` | Listen port (HTTP mode) |
| `--transport` | `BORIS_TRANSPORT` | `http` | Transport: `http` or `stdio` |
| `--workdir` | `BORIS_WORKDIR` | `.` | Initial working directory |
| `--timeout` | `BORIS_TIMEOUT` | `120` | Default bash timeout (seconds). Converted to milliseconds for the tool parameter. |
| `--allow-dir` | `BORIS_ALLOW_DIRS` | (none) | Allowed directories (repeatable flag, comma-sep env) |
| `--deny-dir` | `BORIS_DENY_DIRS` | (none) | Denied dirs/patterns (repeatable flag, comma-sep env) |
| `--no-bash` | `BORIS_NO_BASH` | `false` | Disable bash tool |
| `--max-file-size` | `BORIS_MAX_FILE_SIZE` | `10MB` | Max file size for view/create |
| `--anthropic-compat` | `BORIS_ANTHROPIC_COMPAT` | `false` | Expose combined `str_replace_editor` tool schema |

#### Scenario: Invalid transport value
- **WHEN** boris is started with `--transport=websocket`
- **THEN** boris exits with an error message listing valid transport options

#### Scenario: Repeatable allow-dir flag
- **WHEN** boris is started with `--allow-dir=/src --allow-dir=/tests`
- **THEN** both `/src` and `/tests` are in the allow list

#### Scenario: Comma-separated env var
- **WHEN** `BORIS_ALLOW_DIRS=/src,/tests` is set and no `--allow-dir` flags are provided
- **THEN** both `/src` and `/tests` are in the allow list

#### Scenario: Timeout flag in seconds converted for tool
- **WHEN** boris is started with `--timeout=60`
- **THEN** the bash tool's default timeout is 60000 milliseconds

## ADDED Requirements

### Requirement: Tool errors use IsError instead of protocol errors
All tool handlers SHALL return operational errors (file not found, string not unique, path denied, empty command, etc.) as `CallToolResult` with `IsError: true` and the error message in the `Content` field. Protocol-level errors (returned as Go errors from the handler) SHALL be reserved for infrastructure failures only (e.g., tool handler panic, request deserialization failure).

#### Scenario: File not found returns IsError
- **WHEN** the `view` tool is called with a path that does not exist
- **THEN** the tool returns a `CallToolResult` with `IsError: true` and content describing the error, not a protocol-level error

#### Scenario: Ambiguous match returns IsError
- **WHEN** `str_replace` finds multiple occurrences of `old_str`
- **THEN** the tool returns a `CallToolResult` with `IsError: true` and content indicating the count of occurrences

#### Scenario: Path denied returns IsError
- **WHEN** a file tool is called with a path outside the allowed directories
- **THEN** the tool returns a `CallToolResult` with `IsError: true` and content indicating access is denied

#### Scenario: Bash exit code is not IsError
- **WHEN** a bash command returns exit code 1
- **THEN** the tool returns a normal `CallToolResult` (not `IsError`) with `exit_code: 1` in the text content

### Requirement: Anthropic-compatible combined tool schema
When `--anthropic-compat` is enabled, the server SHALL register a single `str_replace_editor` tool instead of the separate `view`, `str_replace`, and `create_file` tools. The combined tool SHALL accept a `command` parameter with values `view`, `str_replace`, or `create`, and dispatch to the corresponding tool logic. The `bash` tool SHALL always be registered separately regardless of this flag.

#### Scenario: Split tools when anthropic-compat disabled
- **WHEN** boris is started without `--anthropic-compat`
- **THEN** the tool list contains `view`, `str_replace`, and `create_file` as separate tools

#### Scenario: Combined tool when anthropic-compat enabled
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the tool list contains `str_replace_editor` (a single combined tool) and `bash`, but not `view`, `str_replace`, or `create_file` as separate tools

#### Scenario: Combined tool dispatches view
- **WHEN** `str_replace_editor` is called with `command: "view"` and `path: "src/main.go"`
- **THEN** the result is identical to calling the `view` tool directly with the same `path`

#### Scenario: Combined tool dispatches str_replace
- **WHEN** `str_replace_editor` is called with `command: "str_replace"`, `path`, `old_str`, and `new_str`
- **THEN** the result is identical to calling the `str_replace` tool directly with the same parameters

#### Scenario: Combined tool dispatches create
- **WHEN** `str_replace_editor` is called with `command: "create"` and `path` and `file_text`
- **THEN** the result is identical to calling the `create_file` tool with `path` and `content` set to `file_text`

#### Scenario: Combined tool with invalid command
- **WHEN** `str_replace_editor` is called with `command: "delete"`
- **THEN** the tool returns an error via `IsError` indicating the command is not recognized

### Requirement: Shell detection at startup
The server SHALL detect the available shell at startup by checking if `/bin/bash` exists. If it does, `/bin/bash` SHALL be used for bash tool command execution. Otherwise, `/bin/sh` SHALL be used as a fallback. The detected shell path SHALL be logged at startup.

#### Scenario: Bash available
- **WHEN** boris starts on a system where `/bin/bash` exists
- **THEN** the bash tool uses `/bin/bash -c` for command execution

#### Scenario: Bash not available
- **WHEN** boris starts on a minimal container where only `/bin/sh` exists
- **THEN** the bash tool uses `/bin/sh -c` for command execution
