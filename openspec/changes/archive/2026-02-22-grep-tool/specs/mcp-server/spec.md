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

### Requirement: Grep tool registered alongside existing tools
The `grep` tool SHALL be registered in `RegisterAll` alongside existing tools. It SHALL be available regardless of the `--anthropic-compat` flag — grep is always a separate tool, not part of the combined `str_replace_editor` tool. When `--no-bash` is set, the grep tool SHALL still be available (it is a file tool, not a bash tool).

When `--anthropic-compat` is set, the grep tool's JSON schema SHALL use Claude Code's exact parameter names (`glob`, `-i`, `-n`, `-A`, `-B`, `-C`). In normal mode, the schema SHALL use descriptive MCP parameter names (`include`, `case_insensitive`, `line_numbers`, `context_before`, `context_after`, `context`).

#### Scenario: Grep in split mode tool list
- **WHEN** boris is started without `--anthropic-compat`
- **THEN** the tool list contains `bash`, `task_output`, `view`, `str_replace`, `create_file`, and `grep`

#### Scenario: Grep in anthropic-compat mode tool list
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the tool list contains `bash`, `task_output`, `str_replace_editor`, and `grep`

#### Scenario: Grep available with --no-bash
- **WHEN** boris is started with `--no-bash`
- **THEN** the tool list contains `view`, `str_replace`, `create_file`, and `grep` (no `bash` or `task_output`)

#### Scenario: Grep schema uses compat parameter names
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the grep tool schema lists `glob`, `-i`, `-n`, `-A`, `-B`, `-C` as parameter names
