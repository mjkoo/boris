## 1. Core grep implementation

- [x] 1.1 Create `internal/tools/grep.go` with `GrepArgs` struct (pattern, path, include/glob, type, output_mode, case_insensitive/-i, line_numbers/-n, multiline, head_limit, offset, context_before/-B, context_after/-A, context/-C) and `grepHandler` function that wires to `doGrep`
- [x] 1.2 Implement `doGrep` core logic: resolve path via session + pathscope, validate pattern (non-empty, compiles as regexp), handle case_insensitive by prepending `(?i)`, handle multiline by prepending `(?s)`, validate output_mode enum, normalize --anthropic-compat parameter names to internal names
- [x] 1.3 Implement single-file search (line-by-line mode): open file, detect binary (first 512 bytes + `http.DetectContentType`), read line-by-line, match against compiled regex, collect results respecting output_mode, line_numbers, and head_limit/offset
- [x] 1.4 Implement single-file search (multiline mode): read entire file content, match pattern against full string, map match byte ranges back to line numbers, report all spanned lines as match lines
- [x] 1.5 Implement directory walk: use `filepath.WalkDir`, skip `.git`/`node_modules` dirs, parse and apply `.gitignore` patterns at each directory level, skip binary files, apply `include`/`glob` filter via `doublestar.Match` on base name, apply `type` filter against built-in type map, follow symlinks to directories with cycle detection (visited real path set via `filepath.EvalSymlinks`), check path scoping for each file (silently skip denied), search each qualifying file
- [x] 1.6 Implement `type` parameter: define built-in map of type names to glob patterns (c, cpp, css, go, html, java, js, json, markdown, py, rust, ts, yaml) with aliases (python→py, typescript→ts, md→markdown), validate type name and return IsError with valid types list on unknown type, when both type and include/glob set require file matches both
- [x] 1.7 Implement `.gitignore` support: parse `.gitignore` files during walk, apply patterns to files/directories beneath that level, support comment lines, negation patterns, directory-only patterns, nested gitignore overrides parent
- [x] 1.8 Implement symlink following: detect symlinks to directories during WalkDir, resolve via `filepath.EvalSymlinks`, maintain visited real path set for cycle detection, recursively walk symlink targets
- [x] 1.9 Implement context lines: ring buffer for before-context, lookahead for after-context, merge overlapping windows, context line prefix uses `-` for all separators (filepath-linenum-content), explicit context_before/context_after override context shorthand
- [x] 1.10 Implement `--` separators in content mode: emit `--` between non-contiguous output groups (different files or non-adjacent line numbers within same file), regardless of whether context is active
- [x] 1.11 Implement result path formatting: paths in output relative to search root for directory searches, as-provided for single file searches
- [x] 1.12 Implement mtime sorting for `files_with_matches` mode: stat each matching file, sort by modification time (newest first)
- [x] 1.13 Implement head_limit and offset: skip first `offset` results, then collect up to `head_limit` results (0 = unlimited), applied across all output modes, in content mode head_limit caps matching lines (context lines don't count)
- [x] 1.14 Implement `line_numbers` parameter: default true, when false omit line numbers from content mode output (filepath:content instead of filepath:linenum:content), ignored outside content mode

## 2. Tool registration

- [x] 2.1 Register `grep` tool in `RegisterAll` in `internal/tools/tools.go` — always registered (both split and anthropic-compat modes), available even with `--no-bash`
- [x] 2.2 Implement dual schema registration: in normal mode register with MCP parameter names (include, case_insensitive, line_numbers, context_before, context_after, context), in --anthropic-compat mode register with Claude Code names (glob, -i, -n, -B, -A, -C/context), handler normalizes both sets to internal field names

## 3. Tests

- [x] 3.1 Write tests for basic search: literal pattern match, regex pattern match, invalid regex error, empty pattern error
- [x] 3.2 Write tests for output modes: content mode with line numbers, files_with_matches mode (default and sorted by mtime), count mode, invalid output_mode error, default output_mode is files_with_matches
- [x] 3.3 Write tests for context lines: before context, after context, combined context shorthand, explicit overrides shorthand, overlapping context merge, context at file boundaries (clamped), context ignored outside content mode, verify context lines use all-hyphen separators (filepath-linenum-content)
- [x] 3.4 Write tests for `--` separators: separator between different files, separator between non-adjacent matches in same file, no separator for adjacent matches in same file — all without requiring context to be active
- [x] 3.5 Write tests for file filtering: include/glob matches only specified files, include/glob with brace expansion (`*.{ts,tsx}`), type filter matches compound type definitions, type and include/glob combined (must match both), invalid type returns error with valid types list, binary files skipped, `.git` and `node_modules` directories skipped
- [x] 3.6 Write tests for .gitignore: files matching gitignore patterns skipped, nested gitignore overrides parent, negation patterns work, no gitignore present searches all files
- [x] 3.7 Write tests for symlinks: symlinked directories searched, circular symlinks detected and skipped, symlinked files searched
- [x] 3.8 Write tests for path scoping: search root outside allow list returns error, denied files silently skipped during traversal, no scoping when no allow-dir set
- [x] 3.9 Write tests for pagination: head_limit truncates results, unlimited by default (head_limit 0), offset skips results, offset+head_limit pagination, offset exceeds total results returns empty
- [x] 3.10 Write tests for case-insensitive search: case_insensitive/-i flag matches mixed case, default is case-sensitive
- [x] 3.11 Write tests for multiline: multiline pattern spans lines, multiline disabled by default, multiline in files_with_matches mode
- [x] 3.12 Write tests for line_numbers parameter: default true shows line numbers, false omits line numbers, ignored outside content mode
- [x] 3.13 Write tests for path handling: relative path resolved against session cwd, absolute path used directly, single file search, nonexistent path returns error
- [x] 3.14 Write tests for --anthropic-compat parameter names: glob, -i, -n, -A, -B, -C all accepted in compat mode, include/case_insensitive/line_numbers/context_before/context_after/context accepted in normal mode, both produce identical results
- [x] 3.15 Write integration test: grep tool appears in tool list in both split and anthropic-compat modes, grep available with --no-bash, compat mode schema uses Claude Code parameter names
