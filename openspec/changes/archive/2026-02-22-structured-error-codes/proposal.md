## Why

Tool errors are plain `fmt.Errorf` strings with no codes, inconsistent phrasing, and some messages that expose raw Go errors. LLM clients cannot programmatically distinguish error categories, and error messages don't consistently help the model self-correct. The v0.2 roadmap calls for "structured error codes" to fix this.

## What Changes

- Introduce a `[CODE]` prefix convention on all `isError: true` tool responses (e.g., `[PATH_NOT_FOUND] /foo does not exist.`)
- Define a fixed set of cross-tool and tool-specific error codes
- Update `toolErr` helper to accept a code parameter
- Rewrite all ~47 `toolErr` call sites to use codes and improved messages
- Add test assertions that error responses contain the expected `[CODE]` prefix
- No changes to success responses, protocol errors, or bash non-zero exit handling

## Capabilities

### New Capabilities
- `error-codes`: Defines the structured error code scheme, the `toolErr` helper API, error message format rules, and the catalog of valid codes across all tools.

### Modified Capabilities
None. Existing tool specs describe *which conditions* produce errors but don't specify error message format. The error conditions themselves are unchanged — only the message content changes, which is an implementation detail covered by the new `error-codes` capability.

## Impact

- **Code**: `internal/tools/tools.go` (`toolErr` signature change), all tool files (`bash.go`, `view.go`, `str_replace.go`, `create_file.go`, `grep.go`, `find.go`), and their corresponding test files
- **API**: Error response text changes for all tools (codes prepended). Clients parsing exact error strings will see different messages. This is not a breaking change since error text is not a stable API surface.
- **Dependencies**: None — uses only `fmt.Sprintf` and existing `mcp.CallToolResult.SetError`
