## ADDED Requirements

### Requirement: Search file contents with regex pattern
The `grep` tool SHALL accept a `pattern` string parameter (required) and search file contents using Go's `regexp` (RE2) syntax. The pattern MUST be compiled as a valid regular expression — if compilation fails, the tool SHALL return an error via `IsError` indicating the pattern is invalid with the compilation error message.

#### Scenario: Simple literal search
- **WHEN** `grep` is called with `pattern: "TODO"` and the working directory contains files with "TODO" in them
- **THEN** the tool returns matching lines from those files

#### Scenario: Regex pattern search
- **WHEN** `grep` is called with `pattern: "func\s+\w+\("` on a Go project
- **THEN** the tool returns lines matching the function definition pattern

#### Scenario: Invalid regex pattern
- **WHEN** `grep` is called with `pattern: "[invalid"`
- **THEN** the tool returns an `IsError` result with the regex compilation error

#### Scenario: Empty pattern
- **WHEN** `grep` is called with `pattern: ""`
- **THEN** the tool returns an `IsError` result indicating the pattern must not be empty

### Requirement: Search path defaults to session cwd
The `grep` tool SHALL accept an optional `path` string parameter. When provided, it specifies the file or directory to search. When omitted, the tool SHALL search the session's current working directory. Relative paths SHALL be resolved against the session cwd.

#### Scenario: Path omitted defaults to cwd
- **WHEN** `grep` is called with `pattern: "error"` and no `path`, and the session cwd is `/workspace`
- **THEN** the tool searches recursively under `/workspace`

#### Scenario: Relative path resolved against cwd
- **WHEN** the session cwd is `/workspace` and `grep` is called with `path: "src"`
- **THEN** the tool searches recursively under `/workspace/src`

#### Scenario: Absolute path used directly
- **WHEN** `grep` is called with `path: "/workspace/src"`
- **THEN** the tool searches recursively under `/workspace/src`

#### Scenario: Path is a single file
- **WHEN** `grep` is called with `path: "main.go"` pointing to a regular file
- **THEN** the tool searches only that file

#### Scenario: Path does not exist
- **WHEN** `grep` is called with `path: "nonexistent"`
- **THEN** the tool returns an `IsError` result indicating the path was not found

### Requirement: File filtering via include glob
The `grep` tool SHALL accept an optional `include` string parameter containing a glob pattern (e.g., `"*.py"`, `"*.{ts,tsx}"`). When set, only files whose names match the glob SHALL be searched. The glob SHALL be matched against the file's base name using `doublestar.Match` semantics (supporting brace expansion like `{ts,tsx}`).

#### Scenario: Include filter matches
- **WHEN** `grep` is called with `pattern: "import"` and `include: "*.py"` on a directory containing `.py`, `.js`, and `.go` files
- **THEN** only `.py` files are searched

#### Scenario: Include filter with no matches
- **WHEN** `grep` is called with `include: "*.rs"` on a directory containing no `.rs` files
- **THEN** the tool returns an empty result (no error)

#### Scenario: Include filter omitted
- **WHEN** `grep` is called without `include`
- **THEN** all non-binary, non-skipped files are searched

### Requirement: File type filtering via type parameter
The `grep` tool SHALL accept an optional `type` string parameter. When set, only files matching the type's built-in glob patterns SHALL be searched. The tool SHALL support a built-in map of type names to glob patterns covering the most common languages (see design.md Decision #12 for the full table). Type aliases SHALL be supported (`python` → `py`, `typescript` → `ts`, `md` → `markdown`).

When both `type` and `include`/`glob` are specified, both filters apply — a file must match both to be searched. An unrecognized type name SHALL return an `IsError` listing the valid type names.

#### Scenario: Type filter matches
- **WHEN** `grep` is called with `pattern: "func"` and `type: "go"` on a directory containing `.go`, `.py`, and `.js` files
- **THEN** only `.go` files are searched

#### Scenario: Compound type filter
- **WHEN** `grep` is called with `type: "ts"` on a directory containing `app.ts`, `component.tsx`, `helper.mts`, and `style.css`
- **THEN** `app.ts`, `component.tsx`, and `helper.mts` are searched; `style.css` is not

#### Scenario: Type alias
- **WHEN** `grep` is called with `type: "python"`
- **THEN** this is equivalent to `type: "py"` (searches `*.py`, `*.pyi`)

#### Scenario: Invalid type name
- **WHEN** `grep` is called with `type: "brainfuck"`
- **THEN** the tool returns an `IsError` listing the valid type names

#### Scenario: Type and include both specified
- **WHEN** `grep` is called with `type: "js"` and `include: "*.mjs"` (or `glob: "*.mjs"` in compat mode)
- **THEN** only `.mjs` files are searched (must match both filters)

### Requirement: Three output modes
The `grep` tool SHALL accept an optional `output_mode` string parameter with values `"content"`, `"files_with_matches"` (default), or `"count"`.

#### Scenario: Content mode
- **WHEN** `grep` is called with `pattern: "error"` and `output_mode: "content"`
- **THEN** the result contains matching lines prefixed with `filepath:line_number:` (e.g., `src/main.go:42:  if err != nil {`)

#### Scenario: Files with matches is the default
- **WHEN** `grep` is called with `pattern: "error"` and `output_mode` is omitted
- **THEN** the result contains only file paths (same as `output_mode: "files_with_matches"`)

#### Scenario: Files with matches mode
- **WHEN** `grep` is called with `output_mode: "files_with_matches"`
- **THEN** the result contains only file paths (one per line) that contain at least one match, sorted by file modification time (newest first)

#### Scenario: Count mode
- **WHEN** `grep` is called with `output_mode: "count"`
- **THEN** the result contains file paths with match counts (e.g., `src/main.go:5`)

#### Scenario: Invalid output mode
- **WHEN** `grep` is called with `output_mode: "summary"`
- **THEN** the tool returns an `IsError` result indicating the valid output mode values

### Requirement: Content mode line number display
The `grep` tool SHALL accept an optional boolean `line_numbers` parameter (default `true`; named `-n` in `--anthropic-compat` mode). In `content` output mode, when `line_numbers` is true, each matching line SHALL be prefixed with the file path and 1-indexed line number, separated by colons: `filepath:line_number:line_content`. When `line_numbers` is false, line numbers SHALL be omitted: `filepath:line_content`. This parameter SHALL be ignored outside `content` mode.

#### Scenario: Line numbers shown by default
- **WHEN** a match is found on line 42 of `src/main.go` with content `  if err != nil {`
- **THEN** the output line is `src/main.go:42:  if err != nil {`

#### Scenario: Line numbers suppressed
- **WHEN** `grep` is called with `line_numbers: false` (or `-n: false` in compat mode) and a match is found on line 42 of `src/main.go`
- **THEN** the output line is `src/main.go:  if err != nil {`

#### Scenario: Multiple matches in same file
- **WHEN** pattern matches lines 10 and 25 of `src/main.go`
- **THEN** both lines appear in order: `src/main.go:10:...` followed by `src/main.go:25:...`

#### Scenario: Line numbers ignored outside content mode
- **WHEN** `grep` is called with `output_mode: "files_with_matches"` and `line_numbers: false`
- **THEN** the `line_numbers` parameter is ignored and only file paths are returned

### Requirement: Context lines in content mode
The `grep` tool SHALL accept optional integer parameters `context_before` (lines before each match), `context_after` (lines after each match), and `context` (shorthand for both). These SHALL only take effect when `output_mode` is `"content"`.

Context lines SHALL be prefixed with `filepath-line_number-` (using `-` for all separators instead of `:`, to distinguish from match lines). When context windows from adjacent matches overlap, the overlapping lines SHALL appear only once (merged).

In `content` mode, a `--` separator line SHALL appear between non-contiguous output groups — both between different files and between non-adjacent match regions within the same file. This applies regardless of whether context lines are requested. When context lines cause adjacent match regions to overlap, they merge into a single contiguous group (no `--`).

#### Scenario: Before context
- **WHEN** `grep` is called with `pattern: "error"`, `context_before: 2`, and a match is found on line 10
- **THEN** lines 8 and 9 appear before the match line, prefixed with `filepath-8-` and `filepath-9-`

#### Scenario: After context
- **WHEN** `grep` is called with `pattern: "error"`, `context_after: 2`, and a match is found on line 10
- **THEN** lines 11 and 12 appear after the match line, prefixed with `filepath-11-` and `filepath-12-`

#### Scenario: Context shorthand
- **WHEN** `grep` is called with `context: 3`
- **THEN** 3 lines before and 3 lines after each match are shown (equivalent to `context_before: 3, context_after: 3`)

#### Scenario: Explicit overrides shorthand
- **WHEN** `grep` is called with `context: 3` and `context_before: 1`
- **THEN** 1 line before and 3 lines after each match are shown (explicit `context_before`/`context_after` override the `context` shorthand for that direction)

#### Scenario: Overlapping context merged
- **WHEN** matches are found on lines 10 and 12 with `context: 1`
- **THEN** lines 9-13 are shown as a single contiguous block (line 11 appears once, not twice)

#### Scenario: Non-contiguous matches separated within file
- **WHEN** matches are found on lines 10 and 50 in the same file
- **THEN** a `--` separator line appears between the two match lines

#### Scenario: Separator between different files
- **WHEN** matches are found in `a.go` and `b.go`
- **THEN** a `--` separator line appears between the match lines from each file

#### Scenario: No separator for adjacent matches in same file
- **WHEN** matches are found on lines 10 and 11 of the same file with no context
- **THEN** both match lines appear consecutively with no `--` separator (line numbers are adjacent)

#### Scenario: Context at file boundaries
- **WHEN** a match is on line 2 with `context_before: 5`
- **THEN** only line 1 is shown as before-context (clamped to file start)

#### Scenario: Context ignored outside content mode
- **WHEN** `grep` is called with `output_mode: "files_with_matches"` and `context: 3`
- **THEN** the context parameter is ignored and only file paths are returned

### Requirement: Case-insensitive search
The `grep` tool SHALL accept an optional boolean `case_insensitive` parameter (default false). When true, the search SHALL be case-insensitive. This SHALL be implemented by prepending `(?i)` to the pattern before regex compilation.

#### Scenario: Case-insensitive match
- **WHEN** `grep` is called with `pattern: "error"` and `case_insensitive: true`
- **THEN** lines containing "Error", "ERROR", "error", etc. are all matched

#### Scenario: Case-sensitive by default
- **WHEN** `grep` is called with `pattern: "Error"` without `case_insensitive`
- **THEN** only lines containing exactly "Error" (capital E) are matched

### Requirement: Pagination via head_limit and offset
The `grep` tool SHALL accept optional integer parameters `head_limit` (default 0, meaning unlimited) and `offset` (default 0). `head_limit` caps the number of results returned. `offset` skips the first N results before applying `head_limit`.

In `content` mode, `head_limit` caps the number of **matching lines** (context lines do not count toward the limit). In `files_with_matches` mode, it caps file paths. In `count` mode, it caps file entries. `offset` skips entries before `head_limit` is applied, across all modes.

#### Scenario: head_limit truncates results
- **WHEN** `grep` is called with `head_limit: 10` and there are 50 matches
- **THEN** the result contains the first 10 matches only

#### Scenario: Unlimited by default
- **WHEN** `grep` is called without `head_limit` (or `head_limit: 0`) and there are 200 matches
- **THEN** all 200 matches are returned

#### Scenario: Offset skips results
- **WHEN** `grep` is called with `head_limit: 10, offset: 20` and there are 50 matches
- **THEN** the result contains matches 21-30 (skip first 20, then take 10)

#### Scenario: Offset without head_limit
- **WHEN** `grep` is called with `offset: 10` and there are 50 matches
- **THEN** the result contains matches 11-50 (skip first 10, return the rest)

#### Scenario: Offset exceeds total results
- **WHEN** `grep` is called with `offset: 100` and there are 50 matches
- **THEN** the result is empty (no error)

### Requirement: Skip binary files
The `grep` tool SHALL skip binary files during search. Binary detection SHALL read the first 512 bytes of each file and use `http.DetectContentType` to determine the MIME type. Files whose MIME type does not start with `text/` and is not in the set of known text MIME types (`application/json`, `application/xml`, `application/javascript`, `application/x-yaml`, `application/toml`) SHALL be skipped silently.

#### Scenario: Binary file skipped
- **WHEN** the search directory contains a compiled binary file
- **THEN** the binary file is not searched and does not appear in results

#### Scenario: JSON file searched
- **WHEN** the search directory contains a `.json` file detected as `application/json`
- **THEN** the JSON file is searched normally

#### Scenario: Text file searched
- **WHEN** the search directory contains a `.py` file detected as `text/plain` or `text/x-python`
- **THEN** the file is searched normally

### Requirement: Skip noise directories
The `grep` tool SHALL skip `.git/` and `node_modules/` directories during recursive directory traversal. These directories SHALL be skipped via `fs.SkipDir` during `filepath.WalkDir` to avoid descending into them entirely.

#### Scenario: .git directory skipped
- **WHEN** the search root contains a `.git/` directory
- **THEN** no files within `.git/` are searched

#### Scenario: node_modules skipped
- **WHEN** the search root contains a `node_modules/` directory
- **THEN** no files within `node_modules/` are searched

#### Scenario: Nested node_modules skipped
- **WHEN** a subdirectory contains its own `node_modules/`
- **THEN** that nested `node_modules/` is also skipped

### Requirement: Respect .gitignore patterns
The `grep` tool SHALL parse `.gitignore` files during directory traversal and skip files/directories matching gitignore patterns. Gitignore files SHALL be processed at each directory level, with child `.gitignore` files overriding parent patterns for their subtree. Standard gitignore semantics SHALL be followed: comment lines (`#`), negation patterns (`!`), directory-only patterns (trailing `/`), and glob patterns.

#### Scenario: Build output ignored
- **WHEN** the project's `.gitignore` contains `dist/` and the search directory contains a `dist/` directory with matching files
- **THEN** files in `dist/` are not searched

#### Scenario: Generated files ignored
- **WHEN** `.gitignore` contains `*.generated.go` and the directory contains `schema.generated.go`
- **THEN** `schema.generated.go` is not searched

#### Scenario: Nested gitignore overrides parent
- **WHEN** the root `.gitignore` ignores `*.log` but `src/.gitignore` contains `!debug.log`
- **THEN** `src/debug.log` IS searched (negation overrides parent)

#### Scenario: No gitignore file present
- **WHEN** no `.gitignore` file exists in the search path or its parents
- **THEN** all non-binary, non-skipped files are searched (no error)

### Requirement: Follow symlinks during traversal
The `grep` tool SHALL follow symbolic links to directories during recursive directory traversal. To prevent infinite loops from circular symlinks, the tool SHALL maintain a set of visited real paths (resolved via `filepath.EvalSymlinks`). When a symlink target has already been visited, it SHALL be skipped silently.

#### Scenario: Symlinked directory searched
- **WHEN** the search directory contains a symlink `vendor -> ../shared/vendor` pointing to a directory with matching files
- **THEN** files in the symlinked directory are searched

#### Scenario: Circular symlink detected
- **WHEN** directory `a/` contains a symlink `b -> ../a` creating a cycle
- **THEN** the cycle is detected and the redundant traversal is skipped silently (no error, no infinite loop)

#### Scenario: Symlinked file searched
- **WHEN** the search directory contains a symlink `config.json -> /etc/app/config.json` pointing to a regular file
- **THEN** the symlinked file is searched normally

### Requirement: Multiline matching
The `grep` tool SHALL accept an optional boolean `multiline` parameter (default false). When true, the tool SHALL prepend `(?s)` to the pattern (enabling dot-all mode where `.` matches newlines) and search each file's content as a whole string instead of line-by-line.

For `files_with_matches` and `count` modes, the match is checked against the full file content. For `content` mode, each match's byte range SHALL be mapped back to line numbers, and all lines spanned by the match SHALL be reported as match lines. Context lines work based on the line range of the match.

#### Scenario: Multiline pattern spans lines
- **WHEN** `grep` is called with `pattern: "struct \{.*?\}"`, `multiline: true`, and `output_mode: "content"` on a file containing a struct definition spanning lines 5-8
- **THEN** lines 5 through 8 are all reported as match lines

#### Scenario: Multiline disabled by default
- **WHEN** `grep` is called with `pattern: "foo.*bar"` without `multiline` on a file where `foo` is on line 1 and `bar` is on line 2
- **THEN** no match is found (`.` does not match newlines by default)

#### Scenario: Multiline in files_with_matches mode
- **WHEN** `grep` is called with `pattern: "func.*\n.*return"`, `multiline: true`, and `output_mode: "files_with_matches"`
- **THEN** file paths containing the multi-line pattern are returned

### Requirement: Files with matches sorted by modification time
In `files_with_matches` output mode, results SHALL be sorted by file modification time with newest files first. This requires one `os.Stat` call per matching file.

#### Scenario: Newest files first
- **WHEN** `grep` is called with `output_mode: "files_with_matches"` and matches are found in `old.go` (modified 2024-01-01) and `new.go` (modified 2025-06-15)
- **THEN** `new.go` appears before `old.go` in the result

#### Scenario: Sorting does not affect other modes
- **WHEN** `grep` is called with `output_mode: "content"`
- **THEN** results appear in file-system walk order (no mtime sorting)

### Requirement: Path scoping enforcement
The `grep` tool SHALL resolve the search `path` through the path scoping resolver (allow/deny lists) before searching. If the resolved path is denied, the tool SHALL return an `IsError`. During directory traversal, individual files whose resolved paths are denied SHALL be silently skipped (not errors).

#### Scenario: Search root outside allow list
- **WHEN** `--allow-dir=/workspace` is set and `grep` is called with `path: "/etc"`
- **THEN** the tool returns an `IsError` indicating the path is outside allowed directories

#### Scenario: Files matching deny pattern skipped
- **WHEN** `--deny-dir='**/.env'` is set and the search directory contains `.env` files
- **THEN** `.env` files are silently skipped during search

#### Scenario: No scoping when no allow-dir set
- **WHEN** no `--allow-dir` flags are set
- **THEN** all paths are searchable (no scoping enforcement)

### Requirement: Parameter naming conditional on --anthropic-compat
The `grep` tool SHALL expose different parameter names in its JSON schema depending on the `--anthropic-compat` flag. In normal MCP mode, descriptive parameter names SHALL be used. In `--anthropic-compat` mode, Claude Code's exact parameter names SHALL be used. The handler SHALL accept both name sets and normalize internally. Behavior SHALL be identical regardless of which name set is used.

| Normal MCP Mode | `--anthropic-compat` Mode |
|----------------|--------------------------|
| `include` | `glob` |
| `case_insensitive` | `-i` |
| `line_numbers` | `-n` |
| `context_before` | `-B` |
| `context_after` | `-A` |
| `context` | `-C` (and `context` alias) |

Parameters identical in both modes: `pattern`, `path`, `type`, `output_mode`, `multiline`, `head_limit`, `offset`.

#### Scenario: Normal mode parameter names
- **WHEN** `--anthropic-compat` is NOT set and `grep` is called with `include: "*.go"` and `case_insensitive: true`
- **THEN** the tool accepts these parameters and searches `.go` files case-insensitively

#### Scenario: Compat mode parameter names
- **WHEN** `--anthropic-compat` IS set and `grep` is called with `glob: "*.go"` and `-i: true`
- **THEN** the tool accepts these parameters and searches `.go` files case-insensitively

#### Scenario: Compat mode schema exposes Claude Code names
- **WHEN** `--anthropic-compat` IS set
- **THEN** the tool's JSON schema lists `glob`, `-i`, `-n`, `-A`, `-B`, `-C` as parameter names (not `include`, `case_insensitive`, etc.)

### Requirement: Result paths are relative to search root
File paths in the output SHALL be relative to the search `path` when the search path is a directory. When the search path is a single file, the path in the output SHALL be the path as provided by the caller.

#### Scenario: Relative paths in directory search
- **WHEN** `grep` is called with `path: "/workspace/project"` and a match is found in `/workspace/project/src/main.go`
- **THEN** the output shows `src/main.go:42:...` (relative to search root)

#### Scenario: Single file search path
- **WHEN** `grep` is called with `path: "main.go"` (a single file)
- **THEN** the output shows `main.go:42:...`
