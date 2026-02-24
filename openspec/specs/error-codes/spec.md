## ADDED Requirements

### Requirement: Error message format
All tool error responses (`isError: true`) SHALL use the format `[CODE] Human-readable message.` where CODE is an UPPER_SNAKE_CASE error code enclosed in square brackets, followed by a space and a descriptive message. The code SHALL be the first token in the content text.

#### Scenario: Error contains code prefix
- **WHEN** any tool returns an `isError: true` response
- **THEN** the content text starts with `[` followed by an UPPER_SNAKE_CASE code, `]`, a space, and a human-readable message

#### Scenario: Code is parseable
- **WHEN** a client splits the content text on `] ` (bracket-space)
- **THEN** the first segment (after removing the leading `[`) is the error code, and the remainder is the message

### Requirement: Error codes are string constants
Each error code SHALL be defined as a Go string constant in `internal/tools/`. The `toolErr` function SHALL accept an error code as its first parameter. Every `toolErr` call site SHALL use a named constant, not a string literal.

#### Scenario: toolErr signature
- **WHEN** `toolErr` is called
- **THEN** the first argument is a string constant representing the error code, the second is a format string, and any remaining arguments are format parameters

#### Scenario: Missing code causes compile error
- **WHEN** a developer calls `toolErr` with only a format string (old signature)
- **THEN** the code does not compile

### Requirement: Cross-tool error codes
The following error codes SHALL be available for use by any tool:

| Code | Condition |
|------|-----------|
| `INVALID_INPUT` | A required parameter is missing, empty, or has an invalid value |
| `PATH_NOT_FOUND` | The resolved file or directory path does not exist |
| `ACCESS_DENIED` | Path is outside allowed directories or matches a deny pattern |
| `FILE_TOO_LARGE` | File or content exceeds the configured maximum size |
| `IO_ERROR` | A file system read, write, or stat operation failed |

#### Scenario: INVALID_INPUT on empty required parameter
- **WHEN** a tool receives an empty value for a required parameter (e.g., empty `command` for bash, empty `pattern` for grep)
- **THEN** the response is `isError: true` with code `INVALID_INPUT`

#### Scenario: PATH_NOT_FOUND on missing file
- **WHEN** a tool resolves a path that does not exist on disk
- **THEN** the response is `isError: true` with code `PATH_NOT_FOUND`

#### Scenario: ACCESS_DENIED on scoping violation
- **WHEN** a file tool receives a path that the path resolver rejects (outside allow list or matching deny pattern)
- **THEN** the response is `isError: true` with code `ACCESS_DENIED`

#### Scenario: FILE_TOO_LARGE on size limit
- **WHEN** a file exceeds `--max-file-size` or content exceeds the limit on create
- **THEN** the response is `isError: true` with code `FILE_TOO_LARGE`

#### Scenario: IO_ERROR on filesystem failure
- **WHEN** a read, write, or stat call fails for a path that exists and is within scope
- **THEN** the response is `isError: true` with code `IO_ERROR`

### Requirement: Bash tool error codes
The bash tool SHALL use the following additional codes:

| Code | Condition |
|------|-----------|
| `BASH_EMPTY_COMMAND` | The `command` parameter is empty |
| `BASH_START_FAILED` | The subprocess could not be started (pipe or exec failure) |
| `BASH_TASK_LIMIT` | Maximum concurrent background tasks reached |
| `BASH_TASK_NOT_FOUND` | The requested `task_id` does not exist in the session |

#### Scenario: Empty command
- **WHEN** bash tool receives `command: ""`
- **THEN** the response is `isError: true` with code `BASH_EMPTY_COMMAND`

#### Scenario: Task limit exceeded
- **WHEN** 10 background tasks are running and a new background command is requested
- **THEN** the response is `isError: true` with code `BASH_TASK_LIMIT`

#### Scenario: Unknown task ID
- **WHEN** `task_output` is called with a `task_id` that does not exist
- **THEN** the response is `isError: true` with code `BASH_TASK_NOT_FOUND`

#### Scenario: Subprocess start failure
- **WHEN** the bash tool fails to create pipes or start the process
- **THEN** the response is `isError: true` with code `BASH_START_FAILED`

### Requirement: Str_replace tool error codes
The str_replace tool SHALL use the following additional codes:

| Code | Condition |
|------|-----------|
| `STR_REPLACE_NOT_FOUND` | `old_str` is not present in the file |
| `STR_REPLACE_AMBIGUOUS` | `old_str` matches multiple locations and `replace_all` is false |

#### Scenario: String not found
- **WHEN** `old_str` does not appear in the target file
- **THEN** the response is `isError: true` with code `STR_REPLACE_NOT_FOUND`

#### Scenario: Ambiguous match
- **WHEN** `old_str` appears 3 times and `replace_all` is false
- **THEN** the response is `isError: true` with code `STR_REPLACE_AMBIGUOUS` and the message includes the occurrence count

### Requirement: Grep tool error codes
The grep tool SHALL use the following additional codes:

| Code | Condition |
|------|-----------|
| `GREP_INVALID_PATTERN` | The regex pattern does not compile |
| `GREP_INVALID_OUTPUT_MODE` | The `output_mode` value is not recognized |

#### Scenario: Invalid regex
- **WHEN** the grep tool receives a pattern that fails RE2 compilation
- **THEN** the response is `isError: true` with code `GREP_INVALID_PATTERN` and the message includes the compilation error

#### Scenario: Invalid output mode
- **WHEN** the grep tool receives `output_mode: "invalid"`
- **THEN** the response is `isError: true` with code `GREP_INVALID_OUTPUT_MODE` and the message lists valid values

### Requirement: Glob tool error codes
The glob tool SHALL use the following additional codes:

| Code | Condition |
|------|-----------|
| `GLOB_INVALID_PATTERN` | The glob pattern does not compile |
| `GLOB_INVALID_TYPE` | The `type` filter value is not recognized |

#### Scenario: Invalid glob pattern
- **WHEN** the glob tool receives a malformed glob pattern
- **THEN** the response is `isError: true` with code `GLOB_INVALID_PATTERN`

#### Scenario: Invalid type filter
- **WHEN** the glob tool receives `type: "symlink"` (not a valid value)
- **THEN** the response is `isError: true` with code `GLOB_INVALID_TYPE` and the message lists valid values

### Requirement: Error messages aid LLM self-correction
Every error message (the text after the `[CODE]` prefix) SHALL state what happened, include the relevant value (path, pattern, count), and when applicable suggest how to fix the request. Messages SHALL NOT contain raw Go error strings without context, stack traces, or internal implementation details.

#### Scenario: Message includes the path
- **WHEN** a `PATH_NOT_FOUND` error is returned for `/home/user/missing.txt`
- **THEN** the message includes the string `/home/user/missing.txt`

#### Scenario: Message suggests correction
- **WHEN** a `STR_REPLACE_AMBIGUOUS` error is returned
- **THEN** the message includes the occurrence count and suggests using `replace_all`

#### Scenario: IO_ERROR wraps context
- **WHEN** an `IO_ERROR` occurs while reading `/home/user/file.txt`
- **THEN** the message includes the path and a description of the operation that failed (e.g., "could not read"), not just the raw Go error string

### Requirement: Non-zero bash exit codes are not errors
When a bash command exits with a non-zero exit code, the tool SHALL return a normal response (not `isError: true`). The exit code and output SHALL be included in the content text for the LLM to interpret.

#### Scenario: Command exits with code 1
- **WHEN** a bash command exits with code 1 and produces output "file not found"
- **THEN** the response has `isError` absent or false, and the content includes the exit code and command output
