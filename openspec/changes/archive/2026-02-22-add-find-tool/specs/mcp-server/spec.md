## MODIFIED Requirements

### Requirement: Grep tool registered alongside existing tools
The `grep` tool SHALL be registered in `RegisterAll` alongside existing tools. It SHALL be available regardless of the `--anthropic-compat` flag — grep is always a separate tool, not part of the combined `str_replace_editor` tool. When `--no-bash` is set, the grep tool SHALL still be available (it is a file tool, not a bash tool).

When `--anthropic-compat` is set, the grep tool's JSON schema SHALL use Claude Code's exact parameter names (`glob`, `-i`, `-n`, `-A`, `-B`, `-C`). In normal mode, the schema SHALL use descriptive MCP parameter names (`include`, `case_insensitive`, `line_numbers`, `context_before`, `context_after`, `context`).

#### Scenario: Grep in split mode tool list
- **WHEN** boris is started without `--anthropic-compat`
- **THEN** the tool list contains `bash`, `task_output`, `view`, `str_replace`, `create_file`, `grep`, and `find`

#### Scenario: Grep in anthropic-compat mode tool list
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the tool list contains `bash`, `task_output`, `str_replace_editor`, `grep`, and `Glob`

#### Scenario: Grep available with --no-bash
- **WHEN** boris is started with `--no-bash`
- **THEN** the tool list contains `view`, `str_replace`, `create_file`, `grep`, and `find` (no `bash` or `task_output`)

#### Scenario: Grep schema uses compat parameter names
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the grep tool schema lists `glob`, `-i`, `-n`, `-A`, `-B`, `-C` as parameter names

## ADDED Requirements

### Requirement: Find tool registered alongside existing tools
The `find` tool SHALL be registered in `RegisterAll` alongside existing tools. It SHALL be available regardless of the `--anthropic-compat` flag. When `--no-bash` is set, the find tool SHALL still be available (it is a file tool, not a bash tool).

When `--anthropic-compat` is set, the tool SHALL be registered as `Glob` with Claude Code's exact parameter schema (pattern, path only). In normal mode, the tool SHALL be registered as `find` with an additional optional `type` parameter.

#### Scenario: Find in split mode tool list
- **WHEN** boris is started without `--anthropic-compat`
- **THEN** the tool list contains `find` as a separate tool

#### Scenario: Find in anthropic-compat mode tool list
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the tool list contains `Glob` (not `find`)

#### Scenario: Find available with --no-bash
- **WHEN** boris is started with `--no-bash`
- **THEN** the tool list contains `find` (or `Glob` in compat mode)

#### Scenario: Find schema in compat mode has no type parameter
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the `Glob` tool schema has only `pattern` and `path` parameters, no `type`
