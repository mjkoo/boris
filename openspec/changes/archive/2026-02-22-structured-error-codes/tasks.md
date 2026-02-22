## 1. Core infrastructure

- [x] 1.1 Define error code constants in `internal/tools/tools.go` — 5 cross-tool (`ErrInvalidInput`, `ErrPathNotFound`, `ErrAccessDenied`, `ErrFileTooLarge`, `ErrIO`), 4 bash (`ErrBashEmptyCommand`, `ErrBashStartFailed`, `ErrBashTaskLimit`, `ErrBashTaskNotFound`), 2 str_replace (`ErrStrReplaceNotFound`, `ErrStrReplaceAmbiguous`), 2 grep (`ErrGrepInvalidPattern`, `ErrGrepInvalidOutputMode`), 2 find (`ErrFindInvalidPattern`, `ErrFindInvalidType`)
- [x] 1.2 Change `toolErr` signature from `toolErr(msg string, args ...any)` to `toolErr(code string, msg string, args ...any)` and format output as `[CODE] message`
- [x] 1.3 Add `hasErrorCode(r *mcp.CallToolResult, code string) bool` test helper to `test_helpers_test.go`

## 2. Migrate bash tool

- [x] 2.1 Update 8 `toolErr` call sites in `bash.go` — map empty command to `BASH_EMPTY_COMMAND`, pipe/exec failures to `BASH_START_FAILED`, task limit to `BASH_TASK_LIMIT`, task not found to `BASH_TASK_NOT_FOUND`, task ID generation to `IO_ERROR`
- [x] 2.2 Rewrite bare `%v` error messages in `bash.go` to include operation context (e.g., "could not create stdout pipe: ...")
- [x] 2.3 Add `hasErrorCode` assertions to existing bash error tests

## 3. Migrate view tool

- [x] 3.1 Update 13 `toolErr` call sites in `view.go` — map path resolution to `ACCESS_DENIED`, path not found to `PATH_NOT_FOUND`, file size to `FILE_TOO_LARGE`, I/O failures to `IO_ERROR`, range validation to `INVALID_INPUT`
- [x] 3.2 Rewrite bare `%v` error messages in `view.go` to include path and operation context
- [x] 3.3 Add `hasErrorCode` assertions to existing view error tests

## 4. Migrate str_replace tool

- [x] 4.1 Update 9 `toolErr` call sites in `str_replace.go` — map empty old_str to `INVALID_INPUT`, path resolution to `ACCESS_DENIED`, file not found to `PATH_NOT_FOUND`, string not found to `STR_REPLACE_NOT_FOUND`, ambiguous match to `STR_REPLACE_AMBIGUOUS`, I/O failures to `IO_ERROR`
- [x] 4.2 Rewrite bare `%v` error messages in `str_replace.go` to include path and operation context
- [x] 4.3 Add `hasErrorCode` assertions to existing str_replace error tests

## 5. Migrate create_file tool

- [x] 5.1 Update 4 `toolErr` call sites in `create_file.go` — map content size to `FILE_TOO_LARGE`, path resolution to `ACCESS_DENIED`, mkdir/write failures to `IO_ERROR`
- [x] 5.2 Rewrite bare `%v` error messages in `create_file.go` to include path and operation context
- [x] 5.3 Add `hasErrorCode` assertions to existing create_file error tests

## 6. Migrate grep tool

- [x] 6.1 Update 13 `toolErr` call sites in `grep.go` — map empty pattern to `INVALID_INPUT`, invalid output_mode to `GREP_INVALID_OUTPUT_MODE`, invalid regex to `GREP_INVALID_PATTERN`, path not found to `PATH_NOT_FOUND`, path resolution to `ACCESS_DENIED`, I/O failures to `IO_ERROR`
- [x] 6.2 Rewrite bare `%v` error messages in `grep.go` to include path and operation context
- [x] 6.3 Add `hasErrorCode` assertions to existing grep error tests

## 7. Migrate find tool

- [x] 7.1 Update 6 `toolErr` call sites in `find.go` — map empty pattern to `INVALID_INPUT`, invalid glob to `FIND_INVALID_PATTERN`, invalid type to `FIND_INVALID_TYPE`, path resolution to `ACCESS_DENIED`, lstat/walk failures to `IO_ERROR`
- [x] 7.2 Rewrite bare `%v` error messages in `find.go` to include path and operation context
- [x] 7.3 Add `hasErrorCode` assertions to existing find error tests

## 8. Migrate combined editor handler

- [x] 8.1 Update the 1 `toolErr` call site in `tools.go` (unknown command) to use `INVALID_INPUT`

## 9. Verify

- [x] 9.1 Run `go build ./...` to confirm all call sites compile
- [x] 9.2 Run `go test -race ./...` to confirm all tests pass with race detector
- [x] 9.3 Grep for any remaining `toolErr` calls that don't use a named constant — there should be zero string literals as the first argument
