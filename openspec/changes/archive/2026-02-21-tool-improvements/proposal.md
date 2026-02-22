## Why

Boris's tools have behavioral gaps compared to Claude Code's native tools that reduce agent effectiveness — output that blows up context windows, unnecessary round trips from strict validation, lack of bulk replacement, no background command support, missing `--anthropic-compat` mode, and a timeout API that doesn't match what models are trained on. Addressing these brings Boris closer to the "drop-in Claude Code tools over MCP" promise.

## What Changes

### Bash tool

- **BREAKING**: `timeout` parameter changes from seconds to milliseconds (default 120000, max 600000) to match Claude Code's convention
- Add `run_in_background` boolean parameter — launches command in background, returns a handle; results retrieved via a separate poll/check mechanism
- Implement SSE streaming of stdout/stderr for long-running commands (the PRD promises this but it's not implemented)
- Add random nonce to the cwd sentinel to prevent collision with user output
- Send SIGTERM before SIGKILL on timeout, giving processes a grace period for cleanup
- Truncate stdout/stderr at 30,000 characters to prevent context window blowout
- Use `/bin/bash` instead of `/bin/sh` when available — models produce bash syntax (e.g., `[[ ]]`, `source`, arrays) that breaks under POSIX sh. Fall back to `/bin/sh` if bash is not installed.

### View tool

- Truncate individual lines longer than 2,000 characters
- Clamp `view_range` end to file length instead of returning an error (reduces unnecessary round trips)
- Show dotfiles in directory listings (`.github/`, `.dockerignore`, etc.) — only exclude `.git/` and `node_modules/`
- Indicate symlinks in directory listing output (e.g., `link -> target`)
- For `view_range` requests, avoid reading the entire file into memory when only a small range is needed (efficiency improvement for large files near the size limit)
- Return image files (PNG, JPEG, GIF, WebP) as `ImageContent` with base64-encoded data and MIME type instead of "Binary file (size)" — the MCP SDK already supports `mcp.ImageContent` natively, no external dependencies needed

### str_replace tool

- Add optional `replace_all` boolean parameter for bulk replacements (variable renames, import path changes)
- Fix `new_str` JSON schema: remove `omitempty` tag so the schema correctly distinguishes between "field omitted" (deletion) and "field set to empty string" (also deletion, but explicitly). Currently the `omitempty` tag causes the schema to mark the field as optional in a way that may surprise models into accidental deletions.

### create_file tool

- **BREAKING**: Change default behavior to allow overwriting existing files (matching Claude Code's Write), remove `overwrite` parameter. Safety is the caller's responsibility, not the tool's.

### All tools

- Return errors as `isError: true` on `CallToolResult` instead of protocol-level errors, allowing models to recover gracefully

### New: `--anthropic-compat` mode

- Implement the combined `str_replace_editor` tool schema where `view`, `str_replace`, and `create_file` are sub-commands of a single tool. Claude models are specifically fine-tuned on this schema, so this flag significantly improves tool-call accuracy for Claude. Other models work fine with the split tools.

## Decisions

### Response format: flat text (keep current approach)

Boris returns flat text content (e.g., `"exit_code: 0\nstdout:\n..."`). A structured JSON response would let MCP clients build richer UIs but adds complexity and LLMs consume text naturally. **Decision: keep flat text.** This matches Claude Code's approach and is the simplest format for LLM consumption. Structured responses can be revisited if client tooling demands it.

### Shell scripts and permissions

`create_file` always sets 0644 — shell scripts aren't executable without a follow-up `chmod +x` via bash. Claude Code has the same limitation. **Decision: accept this limitation.** Adding a `mode` parameter adds complexity for a rare case. Models can use bash to set permissions when needed.

### Duplicate block protection in str_replace

If a replacement creates text identical to another block in the file, the edit succeeds but future edits to that area will fail (ambiguous match). Claude Code has the same limitation. **Decision: accept this limitation.** Detecting it would require speculative checking that adds complexity for an edge case. The error message on the subsequent call is clear enough.

## Capabilities

### New Capabilities

_(none — all changes modify existing capabilities)_

### Modified Capabilities

- `bash-tool`: Timeout units change to milliseconds with max cap; add `run_in_background` parameter; implement SSE streaming; sentinel uses random nonce; SIGTERM-then-SIGKILL on timeout; stdout/stderr output truncation at 30K characters; prefer `/bin/bash` over `/bin/sh`
- `file-tools`: `str_replace` gains `replace_all` parameter and fixes `new_str` schema; `view` clamps `view_range` instead of erroring; `view` truncates long lines at 2,000 characters; `view` adds efficient range reading for large files; `view` returns image files as `ImageContent`; `view` directory listing shows dotfiles (except `.git`) and indicates symlinks; `create_file` overwrites by default (removes `overwrite` flag)
- `mcp-server`: Add `--anthropic-compat` flag to expose combined `str_replace_editor` tool schema

## Impact

- **Breaking changes**: (1) Bash `timeout` parameter now in milliseconds — existing clients passing seconds get 1000x shorter timeouts. The CLI `--timeout` flag should remain in seconds as server config while the tool parameter uses milliseconds. (2) `create_file` now overwrites by default — existing clients relying on the safety check will silently overwrite.
- **New capability**: `run_in_background` introduces server-side state for tracking background processes. Needs a mechanism to poll for results (new tool, or parameter on bash).
- **Streaming**: Requires changing bash from synchronous `cmd.Wait()` + buffer to incremental output forwarding via MCP SSE notifications.
- **Anthropic compat**: New `--anthropic-compat` flag changes how tools are registered — combines `view`, `str_replace`, `create_file` into a single `str_replace_editor` tool with a `command` sub-parameter. Requires a routing layer.
- **Shell change**: Switching from `/bin/sh` to `/bin/bash` may change behavior for scripts relying on POSIX-only semantics. Fallback to `/bin/sh` ensures compatibility on minimal containers.
- **Code affected**: `internal/tools/bash.go`, `internal/tools/view.go`, `internal/tools/str_replace.go`, `internal/tools/create_file.go`, `internal/tools/tools.go`, `cmd/boris/main.go` (new flag), all tool handlers (error handling pattern)
- **Test impact**: Existing tests for timeout values, view_range validation, str_replace uniqueness, create_file overwrite protection, and error return format will need updates
- **Wire format**: `isError` change affects the MCP response structure — clients that inspect `CallToolResult.isError` will see tool-level errors there instead of protocol-level faults
- **Future**: `grep` and `find` tools are documented in the PRD and should be implemented in a follow-up iteration; they are out of scope here
