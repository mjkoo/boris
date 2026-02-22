## 1. Foundation (cross-cutting)

- [x] 1.1 Add `toolErr` helper function in `internal/tools/tools.go` that returns `CallToolResult` with `IsError: true` via `SetError()`
- [x] 1.2 Add `Shell` and `AnthropicCompat` fields to `tools.Config` struct
- [x] 1.3 Add shell detection in `cmd/boris/main.go` — check `/bin/bash` exists, fall back to `/bin/sh`, pass to `Config.Shell`
- [x] 1.4 Add `--anthropic-compat` CLI flag and `BORIS_ANTHROPIC_COMPAT` env var to kong struct in `main.go`
- [x] 1.5 Add `Nonce` field to `session.Session`, generate 8-char hex string via `crypto/rand` in `session.New()`
- [x] 1.6 Add `Sentinel()` method to `session.Session` that returns `__BORIS_CWD_<nonce>__`
- [x] 1.7 Add `BackgroundTask` struct and task map to `session.Session` with `AddTask`, `GetTask`, `RemoveTask`, `TaskCount` methods

## 2. Bash tool — core improvements

- [x] 2.1 Change `BashArgs.Timeout` from seconds to milliseconds; update jsonschema description; add `RunInBackground bool` field
- [x] 2.2 Update timeout handling: convert CLI `--timeout` (seconds) to ms default; clamp tool param to max 600000
- [x] 2.3 Replace hardcoded `/bin/sh` with `cfg.Shell` in `exec.Command` call
- [x] 2.4 Replace `cwdSentinel` constant with `sess.Sentinel()` in command wrapping and `parseSentinel`
- [x] 2.5 Implement two-phase timeout: SIGTERM to process group, 5s grace timer, then SIGKILL
- [x] 2.6 Add `truncateOutput` function (30,000 char cap) and apply to stdout/stderr after sentinel parsing
- [x] 2.7 Convert all `return nil, nil, fmt.Errorf(...)` in bash handler to `return toolErr(...)`

## 3. Bash tool — background commands

- [x] 3.1 Implement `run_in_background` branch in bash handler: start command, store `BackgroundTask` in session, return task_id immediately
- [x] 3.2 Ensure background commands do NOT update session cwd (skip sentinel parsing)
- [x] 3.3 Enforce max 10 concurrent background tasks per session
- [x] 3.4 Implement `task_output` tool handler (`TaskOutputArgs` struct, register in `tools.go`)
- [x] 3.5 Implement task_output behavior: return current output + status (running/completed), cleanup completed tasks after retrieval

## 4. Bash tool — streaming

- [x] 4.1 Refactor bash handler to pipe stdout/stderr through `bufio.Scanner` instead of `bytes.Buffer` for foreground commands
- [x] 4.2 Extract `ServerSession` from `CallToolRequest` (stop discarding the request parameter)
- [x] 4.3 Send `NotifyProgress` with incremental output lines during foreground command execution
- [x] 4.4 Ensure final `CallToolResult` still contains complete (truncated) output regardless of notifications sent

## 5. View tool improvements

- [x] 5.1 Add `truncateLine` function (2,000 char cap with suffix) and apply in `formatLines`
- [x] 5.2 Change `view_range` validation: clamp `end` to `totalLines` instead of erroring; keep error for `start > totalLines`
- [x] 5.3 Update directory listing exclusion: replace blanket dotfile filter with specific `{".git": true, "node_modules": true}` map
- [x] 5.4 Add symlink indication in directory listing: check `entry.Type()&os.ModeSymlink`, use `os.Readlink`, append ` -> target`
- [x] 5.5 Add `detectImage` function using `net/http.DetectContentType` on header bytes, with SVG extension fallback
- [x] 5.6 Integrate image detection into `readFile`: return `mcp.ImageContent` for recognized images instead of "Binary file" message
- [x] 5.7 Implement `readFileRange` using `bufio.Scanner` for `view_range` requests to avoid full file allocation
- [x] 5.8 Convert all `return nil, nil, fmt.Errorf(...)` in view handler to `return toolErr(...)`

## 6. str_replace improvements

- [x] 6.1 Add `ReplaceAll bool` field to `StrReplaceArgs` with jsonschema tag
- [x] 6.2 Implement `replace_all` branch: skip uniqueness check, use `strings.ReplaceAll`, return replacement count
- [x] 6.3 Update tool description to document `replace_all` and `new_str` deletion behavior
- [x] 6.4 Convert all `return nil, nil, fmt.Errorf(...)` in str_replace handler to `return toolErr(...)`

## 7. create_file improvements

- [x] 7.1 Remove `Overwrite` field from `CreateFileArgs`; update jsonschema description to "create or overwrite"
- [x] 7.2 Remove existence check from handler — `os.WriteFile` already overwrites
- [x] 7.3 Convert all `return nil, nil, fmt.Errorf(...)` in create_file handler to `return toolErr(...)`

## 8. Anthropic compat mode

- [x] 8.1 Extract core logic from view, str_replace, create_file handlers into standalone `doView`, `doStrReplace`, `doCreateFile` functions
- [x] 8.2 Update existing split-tool handlers to call the extracted functions
- [x] 8.3 Implement `StrReplaceEditorArgs` struct with `command` enum and combined parameter set
- [x] 8.4 Implement `str_replace_editor` handler that dispatches to `doView`/`doStrReplace`/`doCreateFile` based on `command`
- [x] 8.5 Update `RegisterAll` to branch on `cfg.AnthropicCompat` — register combined tool or split tools

## 9. Tests

- [x] 9.1 Update bash timeout tests: change values from seconds to milliseconds, test max cap clamping
- [x] 9.2 Add bash sentinel nonce tests: verify nonce in sentinel string, verify old sentinel format doesn't trigger parser
- [x] 9.3 Add bash SIGTERM/SIGKILL tests: verify graceful shutdown within grace period, verify SIGKILL after 5s
- [x] 9.4 Add bash output truncation tests: within limit, exceeds limit, stderr independent truncation, truncation after sentinel
- [x] 9.5 Add bash background command tests: immediate return with task_id, task limit enforcement, cwd not updated
- [x] 9.6 Add task_output tool tests: running status, completed status with cleanup, unknown task_id error
- [x] 9.7 Update view_range tests: end clamping behavior, start-past-end error preserved
- [x] 9.8 Add view line truncation tests: line within limit, line exceeds limit with suffix
- [x] 9.9 Update view directory listing tests: dotfiles visible (except .git), symlink indication
- [x] 9.10 Add view image detection tests: PNG/JPEG/GIF via magic bytes, SVG via extension, misnamed image, unrecognized binary
- [x] 9.11 Add str_replace replace_all tests: multiple replacements with count, no match error, replace_all false preserves uniqueness check
- [x] 9.12 Update create_file tests: remove overwrite guard tests, verify overwrite-by-default behavior
- [x] 9.13 Add IsError tests: verify operational errors use IsError, verify bash exit codes are NOT IsError
- [x] 9.14 Add anthropic-compat tests: combined tool registration, dispatch to view/str_replace/create, invalid command error
- [x] 9.15 Update integration tests for all changed behaviors
