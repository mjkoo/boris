## Why

Without a grep tool, agents must fall back to `bash` with `grep`/`rg` for content search — the most common discovery operation in coding workflows. This bypasses path scoping (bash is intentionally unscoped), loses structured output, and requires the model to construct correct CLI invocations. A native grep tool is the highest-impact addition to boris's tool set.

## What Changes

- Add a `grep` tool that searches file contents using Go's `regexp` (RE2) engine
- Support three output modes: `content` (matching lines with file:line prefixes), `files_with_matches` (file paths only), and `count` (per-file match counts)
- Support context lines (`context_before`/`-B`, `context_after`/`-A`, `context`/`-C`), case-insensitive search (`case_insensitive`/`-i`), line number toggle (`line_numbers`/`-n`), multiline matching (`multiline`), and pagination (`head_limit`, `offset`)
- Support file filtering via `include`/`glob` glob pattern and `type` parameter with built-in compound type definitions
- Automatically skip binary files, `.git/`, `node_modules/`, and files matching `.gitignore` patterns
- Follow symlinks during directory traversal with cycle detection
- Enforce path scoping (allow/deny lists) on the search root and all result paths
- Parameter names conditional on `--anthropic-compat`: descriptive MCP names in normal mode, Claude Code exact names in compat mode
- Register the tool alongside existing tools in both split and anthropic-compat modes

## Capabilities

### New Capabilities
- `grep-tool`: Content search across files with regex patterns, multiple output modes, context lines, and file filtering

### Modified Capabilities
- `mcp-server`: The grep tool must be registered in the MCP server tool list. Registration logic in `RegisterAll` needs to include the new tool.
- `path-scoping`: The grep tool must use the existing path resolver. No new scoping requirements — just applying the existing system to a new tool.

## Impact

- **New file**: `internal/tools/grep.go` — tool implementation
- **New file**: `internal/tools/grep_test.go` — test suite
- **Modified file**: `internal/tools/tools.go` — register grep tool in `RegisterAll`
- **Modified file**: `internal/tools/tools.go` — grep tool available in both split and anthropic-compat modes (grep is always a separate tool, not part of `str_replace_editor`)
- **New dependency** (optional): lightweight pure-Go gitignore library for `.gitignore` parsing, or implement basic gitignore parsing with Go stdlib
- Uses Go stdlib `regexp`, `bufio`, `filepath`, `os`, and existing `pathscope` + `session` + `doublestar` packages
