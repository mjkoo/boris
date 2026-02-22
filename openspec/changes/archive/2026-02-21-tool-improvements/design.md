## Context

Boris currently exposes 4 MCP tools (`bash`, `view`, `str_replace`, `create_file`) with behaviors that diverge from Claude Code's native tools in ways that reduce agent effectiveness. The proposal identifies 16 changes across all tools plus a new `--anthropic-compat` mode. This design document covers the technical approach for each.

The existing codebase is compact: ~140 lines per tool handler, a session struct tracking cwd, and a path scoping resolver. Tool handlers are registered in `tools.go` via `mcp.AddTool` with Go struct types for automatic schema inference.

## Goals / Non-Goals

**Goals:**

- Match Claude Code's tool behavior where it improves agent effectiveness
- Maintain the zero-dependency static binary property
- Keep the implementation simple — no over-engineering for hypothetical future needs
- Preserve backward compatibility where possible, clearly document breaking changes

**Non-Goals:**

- `grep` and `find` tools (follow-up iteration)
- PDF support, notebook support, web tools
- Multi-session support (separate feature)
- Structured JSON tool responses (keep flat text)

## Decisions

### D1: Error handling — use `CallToolResult.SetError()`

The MCP Go SDK provides `CallToolResult.SetError(err)` which sets `IsError: true` and populates `Content` with the error text. The SDK docs explicitly state: "Any errors that originate from the tool should be reported inside the Content field, with IsError set to true, not as an MCP protocol-level error response."

**Approach:** Create a helper function that all handlers use instead of returning Go errors:

```go
func toolErr(msg string, args ...any) (*mcp.CallToolResult, any, error) {
    r := &mcp.CallToolResult{}
    r.SetError(fmt.Errorf(msg, args...))
    return r, nil, nil
}
```

All current `return nil, nil, fmt.Errorf(...)` calls become `return toolErr(...)`. Protocol-level errors (`return nil, nil, err`) are reserved for truly exceptional cases (tool infrastructure failures, not operational errors).

**Bash exit codes remain data, not errors.** A command returning exit code 1 is reported in the text content as `exit_code: 1`, not via `IsError`. Only bash-level failures (empty command, failed to start process) use `IsError`.

### D2: Bash timeout — milliseconds at the tool API, seconds at the CLI

**Tool parameter:** `timeout` in milliseconds, default 120000, max 600000 (10 minutes). Values above 600000 are clamped to 600000.

**CLI flag:** `--timeout` remains in seconds for human ergonomics (default 120). The handler converts: `defaultTimeoutMs = cfg.DefaultTimeout * 1000`.

**BashArgs struct change:**

```go
type BashArgs struct {
    Command        string `json:"command" jsonschema:"the shell command to execute"`
    Timeout        int    `json:"timeout,omitempty" jsonschema:"timeout in milliseconds (default 120000, max 600000)"`
    RunInBackground bool  `json:"run_in_background,omitempty" jsonschema:"run command in background, returns a task_id"`
}
```

### D3: Sentinel nonce — per-session random suffix

Generate a random 8-character hex string when the session is created. The sentinel becomes `__BORIS_CWD_<nonce>__` (e.g., `__BORIS_CWD_a3f7c210__`). This eliminates collision with user output while keeping the sentinel deterministic within a session (simplifies testing — tests can access the session's nonce).

**Implementation:** Add a `Nonce` field to `session.Session`, generated in `session.New()` via `crypto/rand`. The sentinel format string is built once and reused. `parseSentinel` uses the session's sentinel value instead of a package-level constant.

### D4: SIGTERM before SIGKILL — 5 second grace period

On timeout:
1. Send `SIGTERM` to the process group (`-pgid`)
2. Start a 5-second grace timer
3. If the process exits within 5 seconds, collect output normally
4. If not, send `SIGKILL` to the process group

**Implementation:** Replace the single `time.AfterFunc` with a two-phase approach:

```go
timer := time.AfterFunc(timeoutDuration, func() {
    timedOut.Store(true)
    _ = syscall.Kill(-pgid, syscall.SIGTERM)
    time.AfterFunc(5*time.Second, func() {
        _ = syscall.Kill(-pgid, syscall.SIGKILL)
    })
})
```

The 5-second grace period is not configurable. It's long enough for most cleanup and short enough to not frustrate agents.

### D5: Output truncation — 30,000 character cap

After collecting stdout and stderr (and parsing the sentinel from stdout), truncate each independently:

```go
const maxOutputChars = 30000

func truncateOutput(s string) string {
    if len(s) <= maxOutputChars {
        return s
    }
    return s[:maxOutputChars] + fmt.Sprintf("\n\n[Truncated: output was %d characters, showing first %d]", len(s), maxOutputChars)
}
```

Truncation happens after sentinel parsing so the cwd tracking is unaffected.

### D6: Shell selection — prefer `/bin/bash`, detect at startup

At process startup (in `main.go` or `tools.RegisterAll`), check if `/bin/bash` exists. Store the shell path in `Config`:

```go
type Config struct {
    // ...existing fields...
    Shell string // resolved shell path, "/bin/bash" or "/bin/sh"
}
```

Detection in `main.go`:

```go
shell := "/bin/sh"
if _, err := os.Stat("/bin/bash"); err == nil {
    shell = "/bin/bash"
}
```

The bash handler uses `exec.Command(cfg.Shell, "-c", wrappedCmd)` instead of hardcoded `/bin/sh`.

### D7: Background commands — `run_in_background` + `task_output` tool

**Approach:** Add `run_in_background` parameter to bash. When true, the tool starts the command, stores a handle in session state, and returns immediately with a `task_id`. A new `task_output` tool retrieves results.

**Session state addition:**

```go
type BackgroundTask struct {
    ID       string
    Cmd      *exec.Cmd
    Stdout   *bytes.Buffer
    Stderr   *bytes.Buffer
    Done     chan struct{}
    ExitCode int
}
```

The `Session` struct gains a `map[string]*BackgroundTask` protected by its existing mutex.

**Bash handler behavior when `run_in_background: true`:**
1. Start the command as normal (with process group, sentinel wrapping)
2. Launch a goroutine to wait for completion and collect output
3. Return immediately with `task_id` in the text content

**New `task_output` tool:**

```go
type TaskOutputArgs struct {
    TaskID string `json:"task_id" jsonschema:"the task ID returned by a background bash command"`
}
```

Returns:
- If task is still running: current stdout/stderr captured so far, `status: running`
- If task is done: final stdout/stderr, exit code, `status: completed`
- If task ID is unknown: error via `IsError`

Background tasks are cleaned up after output is retrieved from a completed task (single-read semantics), or after a configurable inactivity timeout.

### D8: Streaming — use MCP progress notifications

The MCP SDK doesn't support streaming tool output directly — tools return a single `CallToolResult`. However, the SDK provides `ServerSession.NotifyProgress()` for sending incremental updates during tool execution.

**Approach:** For foreground bash commands, pipe stdout/stderr through a scanner. On each line, send a progress notification with the line content. The final `CallToolResult` still contains the complete (truncated) output.

This requires access to the `ServerSession` from within the tool handler. The `CallToolRequest` carries a `Session()` method, but the current handler signature discards it (uses `_`). Update handlers to capture the request and extract the session for notifications.

**Trade-off:** Progress notifications are optional in the MCP spec — not all clients support them. The tool result is self-contained regardless. Streaming is a progressive enhancement.

### D9: View — line truncation

After reading lines, truncate any line longer than 2,000 characters:

```go
const maxLineChars = 2000

func truncateLine(line string) string {
    if len(line) <= maxLineChars {
        return line
    }
    return line[:maxLineChars] + fmt.Sprintf("... [truncated, %d chars total]", len(line))
}
```

Applied in `formatLines()` before writing each line.

### D10: View — clamp view_range instead of erroring

Change the validation logic:

```go
// Before: error if end > totalLines
// After: clamp
if end > totalLines {
    end = totalLines
}
if start > totalLines {
    return "", fmt.Errorf("invalid view_range: start %d exceeds total lines %d", start, totalLines)
}
```

Start still errors if it exceeds total lines (there's no reasonable clamp for "start past the end of the file"). End is silently clamped.

### D11: View — directory listing changes

**Dotfile visibility:** Remove the blanket `strings.HasPrefix(name, ".")` filter. Replace with a specific exclusion list:

```go
excluded := map[string]bool{".git": true, "node_modules": true}
```

This shows `.github/`, `.dockerignore`, `.env`, `.eslintrc`, etc.

**Symlink indication:** Use `entry.Type()` to check for symlinks. When a symlink is found, resolve the target with `os.Readlink` and append ` -> target` to the display name:

```go
if entry.Type()&os.ModeSymlink != 0 {
    target, err := os.Readlink(filepath.Join(path, entry.Name()))
    if err == nil {
        name += " -> " + target
    }
}
```

Note: `os.ReadDir` returns entries with type info from `Lstat` (no follow), so symlinks are already detectable without additional stat calls.

### D12: View — efficient range reading

For `view_range` requests on large files, avoid reading the entire file. Use a `bufio.Scanner` to count and skip lines:

```go
func readFileRange(path string, start, end int, maxFileSize int64) (string, int, error) {
    f, err := os.Open(path)
    // ...
    scanner := bufio.NewScanner(f)
    lineNum := 0
    var lines []string
    for scanner.Scan() {
        lineNum++
        if lineNum >= start && lineNum <= end {
            lines = append(lines, scanner.Text())
        }
        if lineNum > end {
            break
        }
    }
    // Continue scanning to get totalLines for clamp validation
    for scanner.Scan() {
        lineNum++
    }
    return formatLines(lines, start), lineNum, nil
}
```

The full-file read path remains for no-range requests (needed for line counting and truncation messaging). Binary detection still reads the first 512 bytes regardless.

**Trade-off:** We still need to scan to the end to know `totalLines` for the clamp behavior. This means we read through the whole file but don't hold it all in memory. For truly huge files near the size limit, this is still a win (streaming read vs. full allocation).

### D13: View — image support

Detect image files using magic bytes via `net/http.DetectContentType`, which sniffs the first 512 bytes — the same bytes we already read for binary detection. This is more resilient than extension-based detection (handles misnamed files, no-extension files).

```go
import "net/http"

func detectImage(header []byte) (string, bool) {
    mime := http.DetectContentType(header)
    if strings.HasPrefix(mime, "image/") {
        return mime, true
    }
    return "", false
}
```

Integrate into the existing binary detection flow in `readFile`:

```go
// header is already read (first 512 bytes) for NUL-byte binary detection
if mime, ok := detectImage(header); ok {
    data, err := io.ReadAll(f) // read remainder
    data = append(header, data...)
    return &mcp.CallToolResult{
        Content: []mcp.Content{&mcp.ImageContent{Data: data, MIMEType: mime}},
    }, nil, nil
}
```

`http.DetectContentType` reliably detects PNG, JPEG, GIF, and WebP via their magic bytes. SVG is the exception — it's XML-based text, so `DetectContentType` returns `text/xml` or `text/plain`, not `image/svg+xml`. For SVG, fall back to extension-based detection:

```go
if !ok && strings.ToLower(filepath.Ext(path)) == ".svg" {
    mime, ok = "image/svg+xml", true
}
```

The existing `max-file-size` limit applies (10MB default). Base64 encoding is handled by the SDK's JSON marshaler.

### D14: str_replace — `replace_all` parameter

Add to `StrReplaceArgs`:

```go
type StrReplaceArgs struct {
    Path       string `json:"path" jsonschema:"file path"`
    OldStr     string `json:"old_str" jsonschema:"the string to find (must be unique unless replace_all is true)"`
    NewStr     string `json:"new_str" jsonschema:"replacement string (empty to delete)"`
    ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"replace all occurrences instead of requiring a unique match"`
}
```

When `replace_all` is true:
- Skip the uniqueness check
- Use `strings.ReplaceAll(content, args.OldStr, args.NewStr)`
- Return count of replacements: `"Replaced 15 occurrences in /path/to/file"`
- Still error if `old_str` is not found at all (count == 0)
- No context snippet (too many locations to show)

When `replace_all` is false: existing behavior unchanged.

**Note on `new_str` omitempty:** Remove the `omitempty` tag from `NewStr`. The field should always be present in the schema. An empty string explicitly means deletion. The current `omitempty` causes the JSON schema to mark it as optional, which is semantically correct (omitting = deletion) but potentially confusing. Without `omitempty`, the schema shows it as a required field, which is slightly wrong in the other direction. The better fix is to keep `omitempty` but update the tool description to clearly document the deletion behavior. This is a documentation fix, not a code change.

### D15: create_file — overwrite by default

Remove the `Overwrite` field from `CreateFileArgs`:

```go
type CreateFileArgs struct {
    Path    string `json:"path" jsonschema:"file path to create or overwrite"`
    Content string `json:"content" jsonschema:"file content"`
}
```

The handler simplifies — remove the existence check entirely. `os.WriteFile` already overwrites. Parent directory creation remains.

### D16: `--anthropic-compat` mode

When `--anthropic-compat` is set, instead of registering `view`, `str_replace`, and `create_file` as separate tools, register a single `str_replace_editor` tool with a `command` enum parameter.

**Schema:**

```go
type StrReplaceEditorArgs struct {
    Command   string `json:"command" jsonschema:"enum=view,str_replace,create"`
    Path      string `json:"path" jsonschema:"file path"`
    ViewRange []int  `json:"view_range,omitempty"`
    OldStr    string `json:"old_str,omitempty"`
    NewStr    string `json:"new_str,omitempty"`
    ReplaceAll bool  `json:"replace_all,omitempty"`
    FileText  string `json:"file_text,omitempty"`
}
```

The handler dispatches based on `command` to the existing handler logic (extracted into shared functions). The `bash` tool is unaffected and always registered separately.

**Implementation approach:** Extract the core logic of each file tool into standalone functions (e.g., `doView`, `doStrReplace`, `doCreateFile`). Both the split-tool handlers and the combined handler call these functions. This avoids code duplication.

**Config change:** Add `AnthropicCompat bool` to `Config`. In `RegisterAll`, branch on this flag to choose which registration path to use.

## Risks / Trade-offs

- **[Timeout unit break]** Existing clients passing `timeout: 30` (meaning 30 seconds) will get 30ms. → *Mitigation: Document clearly in changelog. The tool description includes the unit. Since boris is pre-1.0 with few users, this is an acceptable break.*

- **[create_file overwrite break]** Clients relying on the overwrite guard will silently overwrite files. → *Mitigation: Document in changelog. Models should read before writing as a matter of practice, not rely on tool-level guards.*

- **[Background task state]** `run_in_background` adds server-side state (running processes, buffered output). If a background task runs forever, it leaks resources. → *Mitigation: Background tasks inherit the session timeout. A maximum concurrent background task limit (e.g., 10) prevents runaway accumulation. Completed tasks are cleaned up after retrieval.*

- **[Streaming client support]** Progress notifications are optional in MCP — some clients may ignore them. → *Mitigation: Streaming is a progressive enhancement. The final tool result is self-contained. Agents that don't support notifications still get full output.*

- **[Shell selection]** Switching from `/bin/sh` to `/bin/bash` may subtly change behavior for edge cases (e.g., word splitting differences). → *Mitigation: Bash is a superset of POSIX sh. The fallback to `/bin/sh` ensures minimal containers still work.*

- **[Image base64 size]** A 10MB image becomes ~13MB after base64. Large images in tool responses may be expensive for LLM context. → *Mitigation: The existing `max-file-size` limit caps this. Agents that don't need images can use the existing binary detection message by not requesting image files.*
