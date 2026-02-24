## Context

Boris exposes file tools (view, str_replace, create_file) and a bash tool over MCP. Currently, any file can be edited without first viewing it. The session tracks CWD and background tasks but has no concept of "which files have been seen." The CLI uses `--no-bash` as a single-purpose tool disable flag and `--bg-timeout` as an abbreviated name.

Claude Code enforces view-before-edit server-side in its `str_replace_editor` tool. Boris should match this behavior to provide the same safety guardrails, regardless of which client connects.

## Goals / Non-Goals

**Goals:**
- Enforce that files must be viewed before they can be edited via `str_replace` or overwritten via `create_file`
- Make this configurable via `--require-view-before-edit` with a three-state default (`auto|true|false`)
- Replace `--no-bash` with a generalized `--disable-tools` flag
- Rename `--bg-timeout` to `--background-task-timeout` for clarity

**Non-Goals:**
- Tracking view state across sessions or persisting it to disk
- Making bash commands count as "viewing" a file (only the explicit view tool counts)
- Adding tool-level allow-lists (deny-list is sufficient for server-side control)
- Changing MCP tool input schemas

## Decisions

### Decision 1: Viewed-files tracking lives in Session

The `Session` struct gets a `viewedFiles map[string]struct{}` field, protected by the existing `sync.Mutex`. Two new methods: `MarkViewed(path string)` and `HasViewed(path string) bool`. Paths stored are resolved/canonical (post-symlink-resolution, absolute).

**Why here vs. in tool handlers**: Session is the natural owner of per-connection state. Keeping it in Session means both standard-mode tools and anthropic-compat `str_replace_editor` share the same tracking without duplication.

**Alternative considered**: A separate `ViewTracker` type composed into Session. Rejected — this is a simple set with two operations, not worth a separate type.

### Decision 2: Three-state `--require-view-before-edit` with `auto` default

The flag accepts `auto`, `true`, or `false`. At startup, `auto` is resolved to a concrete boolean before being passed into `tools.Config`. Initially `auto` resolves to `true`. This lets us change the `auto` resolution in a future release (e.g., to `false` for some deployment scenario) without breaking users who explicitly set `true` or `false`.

In `tools.Config`, the field is a plain `bool` (`RequireViewBeforeEdit`). The three-state resolution happens in `main.go` at startup — tool code never sees `auto`.

**Why not just a bool flag**: A bool flag's default can't change between releases without breaking existing invocations that rely on the implicit default.

### Decision 3: `--disable-tools` as a string slice replacing `--no-bash`

`--disable-tools` accepts a repeatable list of tool names (e.g., `--disable-tools bash --disable-tools create_file` or env `BORIS_DISABLE_TOOLS=bash,create_file`). `RegisterAll` skips any tool whose name appears in the set.

Tool names used for filtering are the MCP-registered names: `bash`, `task_output`, `view`, `str_replace`, `create_file`, `grep`, `glob`, `str_replace_editor`. In anthropic-compat mode, `view`/`str_replace`/`create_file` map to `str_replace_editor`, so disabling any of those disables the combined tool.

Validation at startup rejects unknown tool names to catch typos.

**Why deny-list over allow-list**: A server exposes everything by default; operators restrict. An allow-list forces enumerating all desired tools, which is verbose and fragile when new tools are added.

### Decision 4: Rename `BgTimeout` → `BackgroundTaskTimeout`

Straightforward rename across CLI struct, env var, and `tools.Config`. No behavior change.

### Decision 5: `create_file` only checks view-before-edit for overwrites

When `create_file` targets a path that already exists, it's an overwrite and the view check applies. When the path doesn't exist, it's a new file creation and no view is needed. The existence check uses `os.Stat` before the view check — if the file doesn't exist, skip the check entirely.

**Why**: Requiring view before creating a brand-new file makes no sense — there's nothing to view. But overwriting an existing file is an edit and should require the same safety check as `str_replace`.

## Risks / Trade-offs

- **Breaking CLI changes** (`--no-bash` removal, `--bg-timeout` rename) → Mitigated by documenting in release notes and changelog. These are pre-1.0 changes.
- **View tracking memory for long sessions** → A `map[string]struct{}` of canonical paths. Even thousands of unique file paths is negligible memory. No mitigation needed.
- **Race between stat and write in create_file overwrite check** → The file could be created between the stat and the write. This is acceptable — the check is a guardrail for model behavior, not a security boundary. A TOCTOU race here is harmless.
- **Tool name validation in `--disable-tools`** → Must handle both standard and anthropic-compat tool name sets. Validation runs at startup after `--anthropic-compat` is resolved.
