## 1. Session: Viewed-files tracking

- [x] 1.1 Add `viewedFiles map[string]struct{}` field to `Session` struct, initialize in `New()`
- [x] 1.2 Add `MarkViewed(path string)` and `HasViewed(path string) bool` methods (mutex-protected)
- [x] 1.3 Write unit tests: mark/check, unviewed returns false, concurrent access passes race detector

## 2. CLI flag changes

- [x] 2.1 Replace `NoBash bool` with `DisableTools []string` in CLI struct (`--disable-tools`, env `BORIS_DISABLE_TOOLS`)
- [x] 2.2 Add startup validation for `--disable-tools` that rejects unknown tool names (considering anthropic-compat mode)
- [x] 2.3 Rename `BgTimeout int` to `BackgroundTaskTimeout int` (`--background-task-timeout`, env `BORIS_BACKGROUND_TASK_TIMEOUT`)
- [x] 2.4 Add `RequireViewBeforeEdit string` to CLI struct (`auto|true|false`, default `auto`)
- [x] 2.5 Add startup resolution of `auto` → `true` and pass resolved bool into `tools.Config`

## 3. Tools config update

- [x] 3.1 Update `tools.Config`: replace `NoBash bool` with `DisableTools map[string]struct{}`, rename `BgTimeout` to `BackgroundTaskTimeout`, add `RequireViewBeforeEdit bool`
- [x] 3.2 Update `RegisterAll` to skip tools whose names are in `DisableTools` set (handle `bash` implying `task_output`, anthropic-compat mapping)

## 4. View-before-edit enforcement in tool handlers

- [x] 4.1 Update `doView` to call `sess.MarkViewed(resolvedPath)` after successful file reads (not directory listings, not failures)
- [x] 4.2 Update `doStrReplace` to check `sess.HasViewed(resolvedPath)` when `RequireViewBeforeEdit` is true, return `FILE_NOT_VIEWED` error if not viewed
- [x] 4.3 Update `doCreateFile` to check `os.Stat` + `sess.HasViewed(resolvedPath)` when `RequireViewBeforeEdit` is true and file exists, return `FILE_NOT_VIEWED` error if not viewed
- [x] 4.4 Add `ErrFileNotViewed = "FILE_NOT_VIEWED"` error code constant

## 5. Tests

- [x] 5.1 Write unit tests for `str_replace` rejection when file not viewed (both enforcement on and off)
- [x] 5.2 Write unit tests for `create_file` overwrite rejection when file not viewed, and new file creation without view
- [x] 5.3 Write unit tests for view marking via `str_replace_editor` in anthropic-compat mode
- [x] 5.4 Write integration test for full flow: view → str_replace succeeds, str_replace without view fails
- [x] 5.5 Write tests for `--disable-tools` flag: single tool, multiple tools, unknown tool validation, anthropic-compat mapping
- [x] 5.6 Update existing tests that use `NoBash`/`BgTimeout` to use new field names
