## Why

Boris currently allows `str_replace` and `create_file` (when overwriting) to modify files that have never been viewed in the current session. Claude Code enforces a "view before edit" invariant server-side, returning an error if you attempt to edit a file without first viewing it. This is a safety guardrail that prevents blind edits and ensures the model has seen the current file state before making changes. Boris should enforce this in all modes to match the expected tool contract and improve edit safety.

Additionally, the CLI has a few rough edges worth addressing alongside this change:
- `--no-bash` is too specific — a generalized `--disable-tools` flag scales better as more tools are added
- `--bg-timeout` / `BgTimeout` is needlessly abbreviated — explicit naming is preferred
- The view-before-edit behavior should be configurable with a three-state default (`auto`) that allows changing the default in future releases without breaking explicit user choices

## What Changes

- Track which files have been viewed in each session via a set of resolved paths
- The `view` tool (and `str_replace_editor` view command in anthropic-compat mode) marks files as viewed upon successful read
- `str_replace` rejects edits to files not yet viewed in the session, returning a clear error
- `create_file` rejects overwrites of existing files not yet viewed, but allows creation of new files without prior view
- Tracking is per-session and resets when the session ends
- **BREAKING**: Replace `--no-bash` with `--disable-tools <tool,...>` (e.g., `--disable-tools bash`). Env var changes from `BORIS_NO_BASH` to `BORIS_DISABLE_TOOLS`
- **BREAKING**: Rename `--bg-timeout` to `--background-task-timeout`. Env var changes from `BORIS_BG_TIMEOUT` to `BORIS_BACKGROUND_TASK_TIMEOUT`
- Add `--require-view-before-edit=auto|true|false` (default `auto`, initially resolves to `true`). Env var: `BORIS_REQUIRE_VIEW_BEFORE_EDIT`

## Capabilities

### New Capabilities
- `view-before-edit`: Session-level tracking of viewed files, enforced as a precondition on edit operations, configurable via `--require-view-before-edit`

### Modified Capabilities
- `file-tools`: `str_replace` and `create_file` gain a new precondition requiring the target file to have been viewed first (str_replace always, create_file only when overwriting an existing file)

## Impact

- **CLI**: `cmd/boris/main.go` — replace `NoBash bool` with `DisableTools []string`, rename `BgTimeout int` to `BackgroundTaskTimeout int`, add `RequireViewBeforeEdit string` with three-state enum
- **Tool config**: `internal/tools/tools.go` — `Config` struct updated: `NoBash` replaced by `DisableTools`, `BgTimeout` renamed, new `RequireViewBeforeEdit` field. `RegisterAll` updated to skip tools listed in `DisableTools`
- **Session**: `internal/session/session.go` — new viewed-files set + `MarkViewed`/`HasViewed` methods
- **Tool handlers**: `internal/tools/view.go` (mark viewed), `internal/tools/str_replace.go` (check viewed), `internal/tools/create_file.go` (check viewed for overwrites), `internal/tools/tools.go` (anthropic-compat str_replace_editor handler)
- **Error codes**: New error code for "file not viewed" rejection
- **Tests**: New unit tests for session tracking, integration tests for the enforcement flow, updated tests for CLI flag changes
- **No MCP schema changes**: Tool input schemas remain unchanged; only runtime behavior and CLI flags change
