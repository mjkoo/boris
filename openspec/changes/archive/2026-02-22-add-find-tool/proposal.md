## Why

Models need to discover files by name pattern — locating test files, config files, source files matching a convention, etc. Without a dedicated find/glob tool, models fall back to `bash` with `find` or `ls`, which bypasses path scoping and returns unstructured output. The `view` tool's directory listing only goes 2 levels deep, which is insufficient for targeted file discovery in deep directory trees. This is the last missing tool from the PRD's v0.2 milestone.

## What Changes

- Add a `find` tool that searches for files matching a glob pattern, returning paths sorted by modification time (newest first)
- In `--anthropic-compat` mode, expose the tool as `Glob` with Claude Code's exact schema (2 parameters: `pattern`, `path`)
- In default mode, add an optional `type` parameter (`"file"` or `"directory"`) not present in Claude Code
- Do NOT follow symbolic links (matching Claude Code's behavior and the Unix `find` default — see FIND.md for the full rationale)
- Skip `.git/` contents and respect `.gitignore` patterns (consistent with the grep tool)
- Include hidden files in results (unlike `view` directory listings)
- Enforce path scoping (allow/deny lists) on all returned paths
- Apply 30,000 character output truncation (consistent with bash tool)

## Capabilities

### New Capabilities
- `find-tool`: File name glob search tool — pattern matching, directory walking, mtime sorting, gitignore support, path scoping enforcement

### Modified Capabilities
- `mcp-server`: Tool registration — find tool registered alongside existing tools, anthropic-compat mode exposes it as `Glob`

## Impact

- **New file**: `internal/tools/find.go` — tool implementation
- **New file**: `internal/tools/find_test.go` — tests
- **Modified file**: `internal/tools/tools.go` — register find tool in `RegisterAll`, add to anthropic-compat tool list
- **Modified file**: `internal/tools/integration_test.go` — integration tests for find in both modes
- **Shared code**: Reuses gitignore machinery and directory walking patterns from `internal/tools/grep.go`
- **Dependencies**: Uses existing `doublestar` library (already a dependency for `--deny-dir` patterns)
- **No new CLI flags**: Find tool uses existing configuration (`--allow-dir`, `--deny-dir`, `--max-file-size`)
