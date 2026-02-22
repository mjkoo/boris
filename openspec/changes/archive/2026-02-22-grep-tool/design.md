## Context

Boris currently has four tools: `bash`, `view`, `str_replace`, `create_file`. Content search is the most common agent discovery operation — finding function definitions, usages, error strings, imports. Without a native grep tool, agents fall back to `bash grep`/`bash rg`, which bypasses path scoping and returns unstructured output.

The existing codebase provides clear patterns to follow: path resolution via `session.ResolvePath` + `pathscope.Resolver`, tool error handling via `toolErr()`, and tool registration via `mcp.AddTool` in `RegisterAll`. The grep tool follows these same patterns.

Reference: `GREP.md` documents the Anthropic Grep tool specification. Boris implements a feature-complete grep tool matching GREP.md's capabilities, adapted for Go's stdlib (RE2 regex instead of ripgrep).

## Goals / Non-Goals

**Goals:**
- Provide a `grep` tool with regex search, multiple output modes, context lines, and file filtering
- Follow existing patterns (path resolution, error handling, tool registration)
- Enforce path scoping on the search root and all result paths
- Keep it self-contained: Go stdlib + lightweight pure-Go dependencies only (no ripgrep dependency, no CGo)
- Handle large codebases gracefully via result limits and directory skipping

**Non-Goals:**
- Ripgrep-level performance (Go's regexp is slower but sufficient for typical project sizes)

## Decisions

### 1. Regex engine: Go `regexp` (RE2)

**Choice:** Use Go's stdlib `regexp` package.

**Alternatives considered:**
- Shell out to `rg` (ripgrep): Faster, but adds an external dependency, defeats the "single static binary" principle, and complicates path scoping enforcement.
- Use a PCRE2 Go binding: Requires CGo, breaking `CGO_ENABLED=0`.

**Rationale:** RE2 supports all common patterns agents use (character classes, quantifiers, alternation, anchors). The main gaps vs ripgrep (no lookahead/lookbehind, no backreferences) are rarely needed in coding search. Performance is adequate for the project sizes boris targets.

### 2. File walking: `filepath.WalkDir` with symlink following

**Choice:** Use `filepath.WalkDir` for directory traversal with inline skip logic, extended with symlink following and gitignore support.

Walk skips:
- `.git/` directories (return `fs.SkipDir`)
- `node_modules/` directories (return `fs.SkipDir`)
- Files/directories matching `.gitignore` patterns (see Decision #14)
- Binary files (detected by reading first 512 bytes and calling `http.DetectContentType`, same approach as `view` tool)
- Files not matching the `include`/`glob` filter (when specified, matched via `doublestar.Match` against the file's base name to support brace expansion like `*.{ts,tsx}`)
- Files not matching the `type` filter (when specified, see Decision #12)

Symlinks to directories are followed with cycle detection (see Decision #15).

**Rationale:** `WalkDir` is efficient (uses `os.DirEntry` to avoid extra `Stat` calls) and lets us apply path scoping, gitignore, and skip logic inline during traversal. Using `doublestar.Match` for include globs (already a project dependency for deny patterns) gives us brace expansion support that `filepath.Match` lacks.

### 3. Output modes as a parameter

**Choice:** Single `output_mode` parameter with three values: `content`, `files_with_matches` (default), `count`.

Results in `files_with_matches` mode are sorted by file modification time (newest first), matching GREP.md. This requires one `os.Stat` call per matching file and a sort before returning results.

In `content` mode, a `--` separator line appears between non-contiguous output groups — both between different files and between non-adjacent match regions within the same file. This applies regardless of whether context lines are requested. When context lines cause adjacent match regions to overlap, they merge into a single group (no `--`).

**Rationale:** Matches the Anthropic Grep tool's approach. `files_with_matches` as the default matches Claude Code's behavior — models are fine-tuned to expect this. mtime sorting matches GREP.md's specified behavior for `files_with_matches`. `--` separators in content mode match standard grep/ripgrep output conventions.

### 4. Context lines via `context_before`, `context_after`, `context` parameters

**Choice:** Use descriptive parameter names in normal MCP mode: `context_before`, `context_after`, `context` (shorthand for both). In `--anthropic-compat` mode, use Claude Code's exact names: `-A`, `-B`, `-C`/`context` (see Decision #11).

**Implementation:** When context is requested in `content` mode, read the file line-by-line and maintain a ring buffer of previous lines (for before-context). After a match, continue reading for after-context lines. Overlapping context windows are merged (no duplicate lines).

**Rationale:** Context lines are heavily used by agents to understand matches. Descriptive names are clearer in an MCP tool schema than terse Unix flags. For --anthropic-compat, the models are fine-tuned on the terse names.

### 5. Pagination via `head_limit` and `offset`

**Choice:** Two parameters matching GREP.md: `head_limit` (default 0, unlimited) and `offset` (default 0). `head_limit` caps the number of results returned; `offset` skips the first N results before applying the limit.

In `content` mode, `head_limit` caps **matching lines** (context lines don't count toward the limit). In `files_with_matches` mode, it caps file paths. In `count` mode, it caps file entries. `offset` skips entries before `head_limit` is applied.

**Rationale:** Matches GREP.md parameter names and semantics. Models are fine-tuned on these names. `offset` enables pagination for large result sets at negligible implementation cost.

### 6. Path scoping enforcement

**Choice:** Resolve the search `path` through the path resolver before walking. During the walk, each file's absolute path is checked against the path resolver — denied paths are silently skipped (not errors).

**Rationale:** The search root must be scoped (error if outside allow list). Individual files within the walk are silently skipped if denied — erroring on every denied file during a directory walk would be noisy and unhelpful.

### 7. Binary file detection

**Choice:** Read the first 512 bytes of each file, call `http.DetectContentType`, check if the MIME type starts with `text/` or is `application/json`, `application/xml`, `application/javascript`, etc. Skip files detected as binary.

**Rationale:** Same approach used by the `view` tool for binary detection. Consistent behavior across tools.

### 8. Case-insensitive search via `case_insensitive`

**Choice:** When `case_insensitive` (or `-i` in `--anthropic-compat` mode) is true, prepend `(?i)` to the pattern before compiling the regex. See Decision #11 for parameter naming.

**Rationale:** Simple, no-dependency approach. Go's `regexp` supports inline flags natively. If the user also uses `(?i)` in their pattern, the duplicate is harmless.

### 9. Multiline matching

**Choice:** When `multiline` is true, prepend `(?s)` to the pattern and search each file's content as a whole string instead of line-by-line. Go's `regexp` supports `(?s)` natively (dot matches newline).

For `files_with_matches` and `count` modes, the match is simply checked against the full file content. For `content` mode, each match's byte range is mapped back to line numbers, and all lines spanned by the match are reported as match lines. Context lines work the same way (based on the line range of the match).

**Rationale:** Matches GREP.md spec. The per-file memory cost is bounded by `--max-file-size` (default 10MB). Multiline patterns like `struct \{[\s\S]*?field` are useful for finding struct definitions, multi-line function signatures, etc.

### 10. Tool registration

The `grep` tool is always registered as a separate tool, regardless of `--anthropic-compat`. It is not part of the combined `str_replace_editor` — that tool only covers view/str_replace/create_file (matching Anthropic's schema). Grep is registered alongside bash and the file tools.

### 11. Parameter naming conditional on `--anthropic-compat`

**Choice:** The grep tool exposes different parameter names in its JSON schema depending on the `--anthropic-compat` flag. The handler accepts both sets of names and normalizes internally.

| Normal MCP Mode | `--anthropic-compat` Mode |
|----------------|--------------------------|
| `include` | `glob` |
| `case_insensitive` | `-i` |
| `line_numbers` | `-n` |
| `context_before` | `-B` |
| `context_after` | `-A` |
| `context` | `-C` (and `context` alias) |

Parameters that are identical in both modes: `pattern`, `path`, `type`, `output_mode`, `multiline`, `head_limit`, `offset`.

**Rationale:** CLAUDE.md states: "We should follow the schema laid out by tools like Claude Code as closely as possible, the models we're targeting are fine-tuned for this." In `--anthropic-compat` mode, models expect `-i`, `-A`, `-B`, `-C`, `glob`, `-n`. In normal MCP mode, descriptive self-documenting names are more appropriate. The handler normalizes both sets to the same internal fields, so behavior is identical.

### 12. File type filtering via `type` parameter

**Choice:** Accept an optional `type` string parameter with a built-in map of type names to glob patterns, matching ripgrep's type system. When set, only files matching the type's glob patterns are searched. The most common types are supported:

| Type | Extensions |
|------|-----------|
| `c` | `*.c`, `*.h` |
| `cpp` | `*.cpp`, `*.cc`, `*.cxx`, `*.hpp`, `*.hh`, `*.hxx`, `*.h`, `*.inl` |
| `css` | `*.css`, `*.scss` |
| `go` | `*.go` |
| `html` | `*.html`, `*.htm` |
| `java` | `*.java` |
| `js` | `*.js`, `*.mjs`, `*.cjs`, `*.jsx` |
| `json` | `*.json` |
| `markdown` | `*.md`, `*.markdown`, `*.mdx` |
| `py` | `*.py`, `*.pyi` |
| `rust` | `*.rs` |
| `ts` | `*.ts`, `*.tsx`, `*.mts`, `*.cts` |
| `yaml` | `*.yml`, `*.yaml` |

Aliases: `python` → `py`, `typescript` → `ts`, `md` → `markdown`.

When both `type` and `include`/`glob` are specified, both filters apply (a file must match both). An unrecognized type name returns an `IsError` listing the valid type names.

**Rationale:** GREP.md includes `type` as a parameter. The `include` glob covers simple cases, but `type` provides ergonomic compound definitions (e.g., `ts` matches `.ts`, `.tsx`, `.mts`, `.cts`). Agents frequently use type filters. The built-in map covers the most common languages and is extensible later.

### 13. Line number display via `line_numbers` parameter

**Choice:** Accept an optional boolean `line_numbers` parameter (default `true`; named `-n` in `--anthropic-compat` mode). When `true` (default), content-mode output includes line numbers (`filepath:linenum:content`). When `false`, content-mode output omits line numbers (`filepath:content`). Ignored outside content mode.

**Rationale:** GREP.md includes `-n` with default `true`. While suppressing line numbers is rare, including the parameter matches the spec and costs almost nothing to implement.

### 14. `.gitignore` support

**Choice:** During directory traversal, parse `.gitignore` files and skip matching entries. The implementation reads `.gitignore` at each directory level during the walk and applies patterns to files/directories beneath that level, following standard gitignore semantics (child overrides parent, negation patterns, directory-only patterns).

Use the `go-gitignore` library (`github.com/sabhiram/go-gitignore`) for pattern matching, or implement basic gitignore parsing with Go stdlib. If a lightweight Go library is used, it must be pure Go (no CGo).

**Rationale:** GREP.md explicitly lists `.gitignore` under automatic filtering. Without gitignore support, search results include build output (`dist/`, `build/`, `.next/`), generated files, vendor directories, and other noise. This is a meaningful behavioral difference. The hardcoded `.git/` and `node_modules/` skips help but don't cover project-specific ignore patterns.

### 15. Symlink following with cycle detection

**Choice:** During directory traversal, follow symbolic links to directories. Maintain a set of visited real paths (resolved via `filepath.EvalSymlinks`) to detect and break cycles. When a symlink target has already been visited, skip it silently.

For `filepath.WalkDir`, this requires custom logic: when a `DirEntry` is a symlink to a directory, resolve it, check the visited set, and recursively walk the target if not already visited.

**Rationale:** GREP.md states "Followed by default. Ripgrep handles circular symlink detection internally." Not following symlinks means symlinked source directories (common in monorepos) won't be searched. The visited-set approach prevents cycles at the cost of one `EvalSymlinks` call per symlink encountered.

## Risks / Trade-offs

**[Performance on large codebases]** → Go's `regexp` + `filepath.WalkDir` is slower than ripgrep's parallel search. Mitigated by: directory skipping (.git, node_modules), `head_limit` cap, and the fact that boris typically runs scoped to a project directory. For truly massive codebases (>100K files), agents can narrow the search with `path` and `include`.

**[Memory usage with multiline and context]** → Multiline mode reads entire files into memory; bounded by `--max-file-size` (default 10MB). Before-context requires buffering N lines per file via a ring buffer — O(B) memory where B is typically small (1-5 lines).

**[mtime sorting cost]** → `files_with_matches` mode requires one `os.Stat` call per matching file for mtime sorting. For most searches this is negligible (tens to hundreds of files). For pathological cases (pattern matching thousands of files), the stat overhead is still far less than the search itself.

**[.gitignore parsing complexity]** → Gitignore semantics have edge cases (negation patterns, directory-only markers, nested overrides). Using a well-tested library reduces risk. If implementing from scratch, test against real-world `.gitignore` files from popular projects.

**[Symlink cycle detection cost]** → Each symlink to a directory requires `filepath.EvalSymlinks` to get the real path for cycle detection. This is one syscall per symlink encountered, which is negligible for typical projects. Pathological cases with thousands of symlinks are unlikely in real codebases.

**[Dual parameter naming maintenance]** → Supporting two sets of parameter names (normal vs `--anthropic-compat`) adds schema duplication. Mitigated by: the handler normalizes both sets to the same internal fields, so only the schema definitions differ. Test coverage should verify both name sets.
