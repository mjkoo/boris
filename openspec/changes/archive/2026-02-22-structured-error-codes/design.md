## Context

Boris tool errors use `toolErr(msg, args...)` which produces `isError: true` responses with plain `fmt.Errorf` strings. There are ~47 call sites across 7 files. The messages are inconsistent: some expose raw Go errors (`%v`), some lack context for self-correction, and none carry machine-readable codes. The MCP spec doesn't define a structured error code field on `CallToolResult` — only `isError` (bool) and `content` (text). Any structure must live in the text itself.

## Goals / Non-Goals

**Goals:**
- Every `isError: true` response carries a machine-readable error code
- Error messages consistently help LLMs self-correct
- Codes are documented and stable — clients can match on them
- Migration is safe: compile-time enforcement prevents missing codes, tests catch regressions

**Non-Goals:**
- Custom MCP protocol extensions (no extra JSON fields beyond what the spec defines)
- Changing which conditions produce errors vs success (error semantics stay the same)
- Changing bash non-zero exit code handling (stays as normal output)
- i18n or user-facing error localization

## Decisions

### 1. Codes embedded in text via `[CODE]` prefix

**Choice**: Prefix the `content` text with `[CODE]` — e.g., `[PATH_NOT_FOUND] /foo does not exist.`

**Alternatives considered**:
- *Separate `structuredContent` field*: The MCP SDK's `CallToolResult` has a `StructuredContent any` field, but the spec says `structuredContent` and `content` are mutually exclusive and structured content is for typed tool outputs, not errors. Using it for errors would violate the spec.
- *JSON-encoded error object in text*: Wrapping errors in `{"code": "...", "message": "..."}` wastes tokens and the LLM parses bracketed codes just as easily from natural text.
- *No prefix, just better messages*: Loses the machine-readable aspect. Clients and tests can't match on a stable code.

The `[CODE]` prefix is simple, parseable, token-efficient, and doesn't require MCP protocol changes.

### 2. Error codes as string constants

**Choice**: Define codes as `const` strings in `tools.go`:

```go
const (
    ErrInvalidInput  = "INVALID_INPUT"
    ErrPathNotFound  = "PATH_NOT_FOUND"
    ErrAccessDenied  = "ACCESS_DENIED"
    ErrFileTooLarge  = "FILE_TOO_LARGE"
    ErrIO            = "IO_ERROR"
    // tool-specific codes...
)
```

**Alternatives considered**:
- *Typed enum with iota*: Adds marshaling complexity for no benefit — codes appear in text, not in a typed field.
- *Untyped string literals at call sites*: No compile-time checking, easy to typo. Constants give us grep-ability and refactor safety.

### 3. `toolErr` takes code as first parameter

**Choice**: Change signature from `toolErr(msg, args...)` to `toolErr(code, msg, args...)`:

```go
func toolErr(code string, msg string, args ...any) (*mcp.CallToolResult, any, error) {
    r := &mcp.CallToolResult{}
    text := fmt.Sprintf("[%s] %s", code, fmt.Sprintf(msg, args...))
    r.SetError(fmt.Errorf("%s", text))
    return r, nil, nil
}
```

**Alternatives considered**:
- *Builder pattern / options struct*: Over-engineered for a two-field call. Every call site needs code + message, nothing more.
- *Separate functions per code (`toolErrPathNotFound`, etc.)*: Proliferates functions, harder to grep. A single function with a code constant is cleaner.

The signature change breaks all existing call sites at compile time, which is a feature — the compiler forces us to add codes everywhere, preventing gaps.

### 4. All-at-once migration

**Choice**: Update all ~47 call sites in a single change, not incrementally.

**Rationale**: The `toolErr` signature change is intentionally breaking — after the change, the code won't compile until every call site is updated. This guarantees completeness. An incremental approach would require a compatibility shim (old signature alongside new), adding complexity for no safety benefit.

### 5. Test helper for code assertions

**Choice**: Add a helper alongside the existing `isErrorResult` and `resultText`:

```go
func hasErrorCode(r *mcp.CallToolResult, code string) bool {
    return isErrorResult(r) && strings.HasPrefix(resultText(r), "["+code+"]")
}
```

Existing tests already assert `isErrorResult(r)` and check `resultText(r)` content. Adding code checks is additive — just append a `hasErrorCode` assertion to each error test case.

## Risks / Trade-offs

**[Prefix parsing fragility]** → Clients matching on `[CODE]` rely on text format staying stable. Mitigation: document the format as a stable contract in the spec. The format is trivial to parse (bracket-delimited first token) and unlikely to need changing.

**[Message quality is subjective]** → Rewriting ~47 messages for "LLM self-correction quality" involves judgment calls. Mitigation: follow the rules from ERRORS.md (state what happened, include the value, suggest the fix). Review in PR.

**[Compile-time break scope]** → Changing `toolErr` signature touches every tool file at once. Mitigation: this is mechanical (add code constant as first arg), the compiler catches any missed sites, and all existing tests validate the result.
