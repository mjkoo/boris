### Requirement: Session tracks viewed files
The `Session` SHALL maintain a set of canonical file paths that have been viewed during the session. It SHALL expose `MarkViewed(path string)` to add a path to the set and `HasViewed(path string) bool` to check membership. Paths SHALL be stored in their resolved, absolute form (post-symlink resolution). The set SHALL be protected by the session's existing mutex for concurrent safety.

#### Scenario: Mark and check a viewed file
- **WHEN** `MarkViewed("/workspace/src/main.go")` is called
- **THEN** `HasViewed("/workspace/src/main.go")` returns `true`

#### Scenario: Unviewed file returns false
- **WHEN** no file has been marked as viewed
- **THEN** `HasViewed("/workspace/src/main.go")` returns `false`

#### Scenario: Concurrent mark and check
- **WHEN** multiple goroutines concurrently call `MarkViewed` and `HasViewed`
- **THEN** no data races occur (verified by the Go race detector)

#### Scenario: Viewed set is per-session
- **WHEN** session A marks `/workspace/file.go` as viewed
- **THEN** session B's `HasViewed("/workspace/file.go")` returns `false`

### Requirement: View tool marks files as viewed
The `view` tool (and the `str_replace_editor` view command in anthropic-compat mode) SHALL call `MarkViewed` with the resolved file path after successfully reading a file. Directory listings SHALL NOT mark any path as viewed. Failed view operations (file not found, access denied, etc.) SHALL NOT mark the path as viewed.

#### Scenario: Successful file view marks path
- **WHEN** `view` is called with a valid file path and returns successfully
- **THEN** the resolved path is added to the session's viewed set

#### Scenario: Directory listing does not mark viewed
- **WHEN** `view` is called with a path pointing to a directory
- **THEN** no path is added to the viewed set

#### Scenario: Failed view does not mark viewed
- **WHEN** `view` is called with a path that does not exist
- **THEN** the path is not added to the viewed set

#### Scenario: View via str_replace_editor marks viewed
- **WHEN** `str_replace_editor` is called with `command: "view"` on a valid file in anthropic-compat mode
- **THEN** the resolved path is added to the session's viewed set

### Requirement: str_replace requires prior view
When `RequireViewBeforeEdit` is enabled, the `str_replace` tool (and the `str_replace_editor` str_replace command) SHALL check `HasViewed` for the resolved file path before performing any replacement. If the file has not been viewed, the tool SHALL return an error with code `FILE_NOT_VIEWED` and a message indicating the file must be viewed first.

#### Scenario: Edit succeeds after view
- **WHEN** a file has been viewed in the session and `str_replace` is called on that file
- **THEN** the replacement proceeds normally

#### Scenario: Edit rejected without prior view
- **WHEN** a file has NOT been viewed in the session and `str_replace` is called on that file
- **THEN** the tool returns an error with code `FILE_NOT_VIEWED`

#### Scenario: Edit succeeds when enforcement disabled
- **WHEN** `RequireViewBeforeEdit` is `false` and a file has NOT been viewed
- **THEN** the replacement proceeds normally (no view check)

### Requirement: create_file requires prior view for overwrites
When `RequireViewBeforeEdit` is enabled, the `create_file` tool (and the `str_replace_editor` create command) SHALL check whether the target file already exists. If it exists and has not been viewed in the session, the tool SHALL return an error with code `FILE_NOT_VIEWED`. If the file does not exist (new file creation), the view check SHALL be skipped.

#### Scenario: Overwrite succeeds after view
- **WHEN** a file exists, has been viewed, and `create_file` is called on it
- **THEN** the file is overwritten normally

#### Scenario: Overwrite rejected without prior view
- **WHEN** a file exists, has NOT been viewed, and `create_file` is called on it
- **THEN** the tool returns an error with code `FILE_NOT_VIEWED`

#### Scenario: New file creation skips view check
- **WHEN** the target path does not exist and `create_file` is called
- **THEN** the file is created without any view check

### Requirement: --require-view-before-edit flag
The CLI SHALL accept `--require-view-before-edit` with values `auto`, `true`, or `false` (default: `auto`). The corresponding env var SHALL be `BORIS_REQUIRE_VIEW_BEFORE_EDIT`. At startup, `auto` SHALL be resolved to a concrete boolean (`true` in the initial release) before being passed to tool configuration. Tools SHALL only see the resolved boolean, never the string `auto`.

#### Scenario: Default (auto) enables enforcement
- **WHEN** boris is started without `--require-view-before-edit`
- **THEN** view-before-edit enforcement is active (auto resolves to true)

#### Scenario: Explicit true enables enforcement
- **WHEN** boris is started with `--require-view-before-edit=true`
- **THEN** view-before-edit enforcement is active

#### Scenario: Explicit false disables enforcement
- **WHEN** boris is started with `--require-view-before-edit=false`
- **THEN** view-before-edit enforcement is disabled

### Requirement: --disable-tools flag replaces --no-bash
The CLI SHALL accept `--disable-tools` as a repeatable string flag listing tool names to exclude from registration. The corresponding env var SHALL be `BORIS_DISABLE_TOOLS` (comma-separated). `RegisterAll` SHALL skip any tool whose MCP-registered name appears in the disable set. Unknown tool names SHALL cause a startup validation error. In anthropic-compat mode, disabling `view`, `str_replace`, or `create_file` SHALL disable the combined `str_replace_editor` tool.

#### Scenario: Disable bash tool
- **WHEN** boris is started with `--disable-tools bash`
- **THEN** the `bash` and `task_output` tools are not registered

#### Scenario: Disable multiple tools
- **WHEN** boris is started with `--disable-tools bash --disable-tools create_file`
- **THEN** neither `bash`, `task_output`, nor `create_file` are registered

#### Scenario: Unknown tool name rejected
- **WHEN** boris is started with `--disable-tools nonexistent`
- **THEN** startup fails with an error listing valid tool names

#### Scenario: Disable via environment variable
- **WHEN** `BORIS_DISABLE_TOOLS=bash,grep` is set
- **THEN** `bash`, `task_output`, and `grep` are not registered

#### Scenario: Anthropic-compat mode disable mapping
- **WHEN** boris is started with `--anthropic-compat --disable-tools view`
- **THEN** the `str_replace_editor` tool is not registered

### Requirement: --background-task-timeout replaces --bg-timeout
The CLI SHALL accept `--background-task-timeout` (in seconds, default 0 meaning disabled) instead of `--bg-timeout`. The corresponding env var SHALL be `BORIS_BACKGROUND_TASK_TIMEOUT` instead of `BORIS_BG_TIMEOUT`. Behavior is unchanged — only the flag and env var names change.

#### Scenario: Renamed flag works
- **WHEN** boris is started with `--background-task-timeout=300`
- **THEN** background tasks are killed after 300 seconds of runtime
