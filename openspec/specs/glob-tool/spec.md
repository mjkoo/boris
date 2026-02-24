### Requirement: Find files by glob pattern
The `glob` tool SHALL accept a `pattern` string parameter (required) and a `path` string parameter (optional). The tool SHALL recursively search the directory tree rooted at `path` (or the session's current working directory if `path` is omitted) and return all file and directory paths whose relative path matches `pattern` using doublestar glob syntax.

Supported glob syntax SHALL include: `*` (any characters except `/`), `**` (recursive across subdirectories), `?` (single character), `{a,b}` (alternation), `[abc]` (character class), `[a-z]` (character range), `[!abc]` (negated class).

Pattern matching SHALL be attempted against both the full relative path from the search root and the base name of each entry, so that `*.go` matches files at any depth and `src/**/*.go` matches files under a specific subdirectory.

#### Scenario: Simple extension pattern
- **WHEN** `glob` is called with `pattern: "*.go"` in a directory containing `main.go`, `internal/tools/grep.go`, and `README.md`
- **THEN** the result contains `main.go` and `internal/tools/grep.go` but not `README.md`

#### Scenario: Recursive doublestar pattern
- **WHEN** `glob` is called with `pattern: "**/*.test.ts"` in a directory containing `src/app.test.ts` and `src/utils/helper.test.ts`
- **THEN** the result contains both files

#### Scenario: Directory-scoped pattern
- **WHEN** `glob` is called with `pattern: "*.go"` and `path: "internal/tools"`
- **THEN** the result contains only `.go` files under `internal/tools/`, not `.go` files in other directories

#### Scenario: Relative directory path in pattern
- **WHEN** `glob` is called with `pattern: "src/**/*.md"` in a directory containing `src/README.md` and `docs/README.md`
- **THEN** the result contains `src/README.md` but not `docs/README.md`

#### Scenario: Brace expansion
- **WHEN** `glob` is called with `pattern: "*.{ts,tsx}"`
- **THEN** the result contains both `.ts` and `.tsx` files

#### Scenario: Character class
- **WHEN** `glob` is called with `pattern: "[Mm]akefile"`
- **THEN** the result contains files named `Makefile` or `makefile`

### Requirement: Glob returns results sorted by modification time
Results SHALL be returned as a flat list of file paths, one per line, sorted by modification time with the most recently modified files first.

#### Scenario: Mtime sorting
- **WHEN** `glob` is called and file `a.go` was modified 1 hour ago and `b.go` was modified 1 minute ago
- **THEN** `b.go` appears before `a.go` in the output

### Requirement: Glob returns relative paths
Paths in the output SHALL be relative to the search root. When `path` is provided, paths are relative to `path`. When `path` is omitted, paths are relative to the session's current working directory.

#### Scenario: Relative path output
- **WHEN** `glob` is called with `pattern: "*.go"` and `path: "/workspace/project"`
- **THEN** the result contains paths like `main.go` and `internal/tools/grep.go`, not `/workspace/project/main.go`

### Requirement: Glob returns "No files found" for zero results
When no files match the pattern, the tool SHALL return a normal (non-error) result with the text "No files found". The `IsError` flag SHALL NOT be set.

#### Scenario: No matches
- **WHEN** `glob` is called with `pattern: "*.xyz"` and no files match
- **THEN** the result text is "No files found" and `IsError` is false

#### Scenario: Non-existent path
- **WHEN** `glob` is called with `path` pointing to a directory that does not exist
- **THEN** the result text is "No files found" and `IsError` is false

### Requirement: Glob truncates output at 30,000 characters
When the combined output exceeds 30,000 characters, the tool SHALL truncate the output at that limit and append a message indicating truncation.

#### Scenario: Large result set truncated
- **WHEN** `glob` is called with a broad pattern that matches thousands of files
- **THEN** the output is truncated at 30,000 characters with a truncation message appended

### Requirement: Glob does not follow symbolic links to directories
The tool SHALL NOT follow symbolic links that point to directories. Symlinks to directories SHALL NOT be recursed into. Symlinks to files SHALL be included in results if their name matches the pattern.

#### Scenario: Directory symlink not followed
- **WHEN** a directory contains a symlink `linked_dir -> /some/other/directory` and `glob` is called with `pattern: "**/*.go"`
- **THEN** files inside the symlink target are NOT included in results

#### Scenario: File symlink included
- **WHEN** a directory contains a symlink `link.go -> ../other/file.go` and `glob` is called with `pattern: "*.go"`
- **THEN** `link.go` IS included in results

#### Scenario: Broken symlink skipped
- **WHEN** a directory contains a symlink pointing to a nonexistent target
- **THEN** the tool silently skips the broken symlink without error

### Requirement: Glob skips .git and respects .gitignore
The tool SHALL skip `.git/` directories and `node_modules/` directories. The tool SHALL respect `.gitignore` patterns at each directory level during traversal, consistent with the grep tool's gitignore support.

#### Scenario: .git directory skipped
- **WHEN** `glob` is called with `pattern: "**/*"` in a git repository
- **THEN** files inside `.git/` are not included in results

#### Scenario: node_modules skipped
- **WHEN** `glob` is called with `pattern: "**/*.js"` in a project with `node_modules/`
- **THEN** files inside `node_modules/` are not included in results

#### Scenario: Gitignored files excluded
- **WHEN** a `.gitignore` contains `*.log` and `glob` is called with `pattern: "**/*.log"`
- **THEN** `.log` files matched by the gitignore pattern are not included

#### Scenario: Negated gitignore pattern
- **WHEN** a `.gitignore` contains `*.log` and `!important.log`
- **THEN** `important.log` IS included in results despite the `*.log` rule

### Requirement: Glob includes hidden files
The tool SHALL include hidden files and directories (names starting with `.`) in results, except for `.git/` which is always skipped. This is unlike the `view` tool's directory listing which excludes dotfiles.

#### Scenario: Hidden files included
- **WHEN** `glob` is called with `pattern: "**/*"` in a directory containing `.github/workflows/ci.yml` and `.dockerignore`
- **THEN** both `.github/workflows/ci.yml` and `.dockerignore` appear in results

### Requirement: Glob resolves paths and checks scoping
The tool SHALL resolve relative `path` values against the session's current working directory. All matched file paths SHALL be validated against the path scoping resolver (allow/deny lists) before inclusion in results. Files outside allowed directories or inside denied directories SHALL be silently excluded.

#### Scenario: Path scoping enforced
- **WHEN** `--allow-dir=/workspace` is set and `glob` is called with `pattern: "*.go"` in a session with cwd `/workspace`
- **THEN** only files under `/workspace` are included

#### Scenario: Denied paths excluded
- **WHEN** `--deny-dir=**/.env` is set and `glob` is called with `pattern: "**/*"`
- **THEN** `.env` files are excluded from results

#### Scenario: Scoping violation on search root
- **WHEN** `glob` is called with `path: "/etc"` and `--allow-dir=/workspace` is set
- **THEN** the tool returns an error via `IsError` indicating access is denied

### Requirement: Glob returns errors for invalid patterns
The tool SHALL return an `IsError` result when the glob pattern is syntactically invalid.

#### Scenario: Malformed glob pattern
- **WHEN** `glob` is called with `pattern: "[invalid"`
- **THEN** the tool returns `IsError: true` with a message describing the pattern error

#### Scenario: Empty pattern
- **WHEN** `glob` is called with `pattern: ""`
- **THEN** the tool returns `IsError: true` indicating the pattern must not be empty

### Requirement: Glob type filter (default mode only)
In default mode (not `--anthropic-compat`), the `glob` tool SHALL accept an optional `type` string parameter with values `"file"` or `"directory"`. When set to `"file"`, only regular files (and file symlinks) SHALL be returned. When set to `"directory"`, only directories SHALL be returned. When omitted, both files and directories are returned.

#### Scenario: Type filter file
- **WHEN** `glob` is called with `pattern: "**/*"` and `type: "file"` in a directory containing files and subdirectories
- **THEN** only files are returned, no directories

#### Scenario: Type filter directory
- **WHEN** `glob` is called with `pattern: "**/*"` and `type: "directory"`
- **THEN** only directories are returned, no files

#### Scenario: Invalid type value
- **WHEN** `glob` is called with `type: "symlink"`
- **THEN** the tool returns `IsError: true` indicating valid type values are "file" and "directory"

### Requirement: Glob in anthropic-compat mode uses reduced schema
When `--anthropic-compat` is set, the `glob` tool schema SHALL have exactly two parameters: `pattern` (required string) and `path` (optional string). The `type` parameter SHALL NOT be present in the compat schema.

#### Scenario: Compat mode schema
- **WHEN** boris is started with `--anthropic-compat`
- **THEN** the `glob` tool schema has parameters `pattern` (required) and `path` (optional), and no `type` parameter
