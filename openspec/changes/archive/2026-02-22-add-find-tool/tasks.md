## 1. Core find tool implementation

- [x] 1.1 Create `internal/tools/find.go` with `FindArgs` struct (pattern required, path optional, type optional) and `FindCompatArgs` struct (pattern required, path optional, no type), plus `findParams` normalization — following the grep dual-schema pattern
- [x] 1.2 Implement `doFind` function: resolve search path against session cwd, validate pattern is non-empty, validate pattern syntax via `doublestar.ValidatePattern`, validate type parameter (empty, "file", or "directory"), check path scoping on search root
- [x] 1.3 Implement directory walker: recursive `os.ReadDir` loop that skips `.git/` and `node_modules/`, uses `gitignoreStack` for gitignore support, does NOT follow directory symlinks, includes file symlinks, skips broken symlinks silently
- [x] 1.4 Implement glob matching: for each entry, match `doublestar.Match(pattern, relPath)` against relative path and `doublestar.Match(pattern, baseName)` against base name; apply type filter if set
- [x] 1.5 Implement path scoping: validate each matched path through `resolver.Resolve` before including in results, silently skip denied paths
- [x] 1.6 Implement result formatting: collect paths with mtime, sort by mtime descending, join with newlines, return "No files found" for zero results (non-error), truncate output at 30,000 characters with truncation message

## 2. Tool registration

- [x] 2.1 Add find/Glob tool registration in `RegisterAll` in `internal/tools/tools.go`: register as `find` with `FindArgs` in default mode, as `Glob` with `FindCompatArgs` in `--anthropic-compat` mode
- [x] 2.2 Set tool descriptions: default mode uses descriptive MCP description, compat mode uses Claude Code's exact system prompt description text from FIND.md

## 3. Tests — core behavior

- [x] 3.1 Test simple extension pattern matching (`*.go` finds files at all depths)
- [x] 3.2 Test recursive doublestar pattern (`**/*.test.ts`)
- [x] 3.3 Test directory-scoped search via `path` parameter
- [x] 3.4 Test relative directory path in pattern (`src/**/*.md`)
- [x] 3.5 Test brace expansion (`*.{ts,tsx}`) and character class (`[Mm]akefile`)
- [x] 3.6 Test mtime sorting (most recently modified first)
- [x] 3.7 Test output contains relative paths, not absolute
- [x] 3.8 Test "No files found" returned for zero matches (non-error)
- [x] 3.9 Test "No files found" returned for non-existent path (non-error)
- [x] 3.10 Test output truncation at 30,000 characters

## 4. Tests — symlinks

- [x] 4.1 Test directory symlink is NOT followed (files inside symlink target not returned)
- [x] 4.2 Test file symlink IS included in results
- [x] 4.3 Test broken symlink is silently skipped

## 5. Tests — filtering and scoping

- [x] 5.1 Test `.git/` directory is skipped
- [x] 5.2 Test `node_modules/` directory is skipped
- [x] 5.3 Test `.gitignore` patterns are respected (ignored files excluded)
- [x] 5.4 Test negated gitignore pattern (`!important.log` included despite `*.log` rule)
- [x] 5.5 Test hidden files are included (`.github/`, `.dockerignore`)
- [x] 5.6 Test path scoping: denied paths silently excluded
- [x] 5.7 Test path scoping: search root outside allowed dirs returns IsError

## 6. Tests — validation and type filter

- [x] 6.1 Test empty pattern returns IsError
- [x] 6.2 Test malformed pattern (`[invalid`) returns IsError
- [x] 6.3 Test type filter "file" returns only files
- [x] 6.4 Test type filter "directory" returns only directories
- [x] 6.5 Test invalid type value returns IsError

## 7. Tests — integration and compat mode

- [x] 7.1 Add integration test: find tool appears in tool list in default mode
- [x] 7.2 Add integration test: Glob tool appears in tool list in `--anthropic-compat` mode, find does not
- [x] 7.3 Add integration test: Glob schema has only `pattern` and `path` parameters (no `type`)
- [x] 7.4 Update existing integration tests that assert on exact tool list contents (add `find`/`Glob` to expected lists)
