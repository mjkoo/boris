## Context

Boris provides 5 tools (bash, task_output, view, str_replace, create_file) plus grep. The PRD's v0.2 milestone includes a `find` tool for file name pattern matching. The grep tool already implements recursive directory walking with gitignore support, symlink following, and mtime sorting — the find tool shares some of these concerns but differs in key ways (no content search, no symlink following, glob matching instead of regex).

All tools live in `internal/tools/` within the same Go package. The grep tool's internal helpers (`gitignoreStack`, `isSymlinkDir`, etc.) are package-level and directly accessible from a new `find.go` file without refactoring.

Reference: `FIND.md` contains the full Claude Code Glob specification and behavioral analysis.

## Goals / Non-Goals

**Goals:**
- Implement a `find` tool that matches files by glob pattern and returns paths sorted by mtime
- Support `--anthropic-compat` mode where the tool is named `Glob` with Claude Code's exact 2-parameter schema
- Reuse existing gitignore and path scoping infrastructure without duplicating or over-abstracting

**Non-Goals:**
- Content-based filtering (that's grep's job)
- Symlink following (deliberate decision — see FIND.md)
- Pagination/offset parameters (Claude Code's Glob doesn't have them; output truncation is sufficient)
- A general-purpose "walker" abstraction shared between grep and find (the symlink handling differences make this awkward and the benefit is low)

## Decisions

### 1. Tool naming and schema split

**Decision:** `find` in default mode, `Glob` in `--anthropic-compat` mode.

**Rationale:** The PRD specifies `find`. Claude models are fine-tuned on `Glob`. The `--anthropic-compat` flag exists to emulate Claude Code's exact quirks, including naming. This follows the same pattern as grep (which uses different parameter names in compat mode).

**Implementation:** Two arg structs (`FindArgs`, `FindCompatArgs`) normalized to a shared `findParams`, same pattern as grep. In compat mode, `FindCompatArgs` omits the `type` parameter.

**Alternatives considered:**
- Always name it `Glob`: Better Claude accuracy, but inconsistent with PRD and confusing for non-Claude models.
- Always name it `find`: Simpler, but defeats the purpose of `--anthropic-compat`.

### 2. No symlink following

**Decision:** Do not follow symbolic links to directories. Symlinks to files are returned as results if they match the pattern (using `os.Lstat`, the entry exists in the directory).

**Rationale:** Matches Claude Code's Glob behavior. Matches Unix `find` default, ripgrep default, Go `filepath.WalkDir` default. Avoids cycle detection complexity, scope expansion risk, and duplicate results. See FIND.md "Symlink Analysis" section for full rationale.

This is a deliberate inconsistency with Boris's grep tool, which follows symlinks. The tools serve different purposes: find discovers structure (duplicates mislead), grep searches content (completeness matters).

**Implementation:** Use `os.ReadDir` which returns `fs.DirEntry` with `Type()` from `os.Lstat`. When `entry.Type()&os.ModeSymlink != 0`:
- If target is a directory: skip (don't recurse)
- If target is a file: include in results if pattern matches (the symlink name is what gets matched)
- If target resolution fails: skip silently

**Alternatives considered:**
- Follow symlinks with cycle detection (like grep): More results but inconsistent with Claude Code, duplicates waste tokens, scope expansion risk.
- Skip all symlinks entirely: Too aggressive — file symlinks are harmless and commonly used.

### 3. Glob matching strategy

**Decision:** Use `doublestar.Match` to match the pattern against the relative path from the search root.

**Rationale:** Boris already depends on `doublestar` for `--deny-dir` patterns. It supports the full glob syntax (`**`, `{a,b}`, `[abc]`, etc.). Matching against relative paths means `**/*.go` works naturally for recursive search.

**Implementation detail:** For each file encountered during the walk, compute `relPath` (relative to search root) and match `doublestar.Match(pattern, relPath)`. Also try matching against just the base name for simple patterns like `*.go` (same approach as grep's `matchesInclude`).

**Alternatives considered:**
- `filepath.Match`: No `**` support. Insufficient.
- `doublestar.Glob` (the function, not `Match`): This does its own directory walking, which conflicts with our need to control gitignore/skip logic.

### 4. Reuse gitignoreStack from grep, no shared walker abstraction

**Decision:** The find tool's directory walker is a standalone function in `find.go` that directly uses the package-level `gitignoreStack` already defined in `grep.go`. No attempt to extract a shared "walk" function.

**Rationale:** The walkers differ in three key ways: (1) symlink handling (grep follows, find doesn't), (2) what gets collected (grep reads file content, find just records paths), (3) early termination conditions. Forcing these into a shared abstraction would require callback-heavy design or complex options structs — more complexity than the duplication it saves. Both walkers are ~50-60 lines and the shared parts (ReadDir loop, .git/node_modules skip, gitignore check) are straightforward.

**What IS shared (already package-level in grep.go):**
- `gitignoreStack` (push/pop/isIgnored)
- `isSymlinkDir`
- `toolErr`

**What is NOT shared (walker logic):**
- Directory traversal loop (different symlink handling)
- Result collection (mtime + path vs content search)

### 5. Output format and truncation

**Decision:** Return one file path per line, sorted by mtime (newest first). Apply 30,000 character truncation. Return "No files found" (non-error) for zero results.

**Rationale:** Matches Claude Code's Glob output format exactly. The 30K limit is consistent with bash tool truncation.

**Implementation:** Collect all matching paths with their mtime, sort descending by mtime, join with newlines, truncate the joined string at 30,000 characters if needed.

### 6. Path output format

**Decision:** Return paths relative to the search root when `path` is provided, relative to session cwd otherwise.

**Rationale:** Relative paths are shorter (saves tokens) and more useful to the model (can be passed directly to other tools). This matches observed Claude Code behavior.

### 7. Type parameter (default mode only)

**Decision:** Accept optional `type` parameter with values `"file"` or `"directory"`. Omitted in `--anthropic-compat` mode.

**Rationale:** Useful for `find -type d` equivalent operations. Not in Claude Code's schema so must be excluded from compat mode. Simple to implement — just filter on `entry.IsDir()` / `!entry.IsDir()` during the walk.

## Risks / Trade-offs

**[Risk] Pattern matching edge cases with relative paths** → The `doublestar.Match` function matches against a path string. If the user provides `subdir/**/*.md` as the pattern, it should work because we match against `relPath` which includes directory components. This is actually better than Claude Code's Glob, which silently fails for relative paths in the pattern. We should test this works and document it.

**[Risk] Large result sets in big repos** → A pattern like `**/*` in a monorepo could match millions of files. Mitigation: the 30K character truncation caps output naturally. The walk itself could still be slow — we accept this as a known limitation (same as Claude Code). The gitignore support mitigates the worst cases (node_modules, build output, etc.).

**[Risk] Inconsistency between find (no symlink follow) and grep (symlink follow)** → A model might find files via grep that don't appear in find results. Mitigation: this mirrors Claude Code's own behavior, so models are trained for it. Document the difference in tool descriptions.

**[Trade-off] No shared walker abstraction** → Some code duplication between grep.go and find.go walker loops (~20 lines of ReadDir + skip logic). Accepted because the symlink handling differences make abstraction awkward, and both files are in the same package with shared helpers.

**[Trade-off] No pagination** → Unlike grep (which has head_limit/offset), find has no result limiting beyond character truncation. This matches Claude Code's Glob. If needed later, adding head_limit is backwards-compatible.
