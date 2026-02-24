## Context

Boris has comprehensive unit and integration tests but no way to exercise the server through a real Claude Code session. Developers manually build, run, and configure Claude Code each time they want to test end-to-end — a tedious process that discourages manual QA. Common developer commands (`go test -race ./...`, `goreleaser release --snapshot --clean`) are also not codified anywhere.

## Goals / Non-Goals

**Goals:**
- One-command setup for a Claude Code test environment backed by boris against a known codebase
- Codify common developer workflows in a justfile
- Keep it simple — no complex orchestration, just Docker + just

**Non-Goals:**
- Automated integration test suite (existing tests cover tool correctness)
- Replacing CI — the justfile targets are for local development convenience
- Supporting non-Docker runtimes (Podman, etc.) — may work but not a goal

## Decisions

### Docker image: multi-stage build with self-referential workspace

The Dockerfile uses a multi-stage build:
1. **Builder stage**: Go builder that compiles boris from the local source
2. **Runtime stage**: Minimal image (e.g., `debian:bookworm-slim`) with git, bash, and common CLI tools that the boris bash tool expects. Clones `https://github.com/mjkoo/boris` at commit `2cc83a9` into `/workspace`. Copies the built binary and runs it in HTTP mode on port 8080.

**Rationale**: Multi-stage keeps the image small. Using the boris repo itself as the workspace is self-referential and fun — we know the exact file contents, directory structure, and git state at that commit, which makes it easy to verify tool behavior manually.

**Alternative considered**: Mount a local directory as the workspace. Rejected because the point is a *known, reproducible* state — local directories change.

### Test environment runs in foreground

`just test-env` builds the image and runs the container in the foreground with `docker build` + `docker run --rm -p 8080:8080`. Server logs stream to the terminal. Ctrl-C stops and removes the container automatically (`--rm`).

**Rationale**: No named containers, no detached mode, no separate stop command. The developer sees logs in real-time and stops when done. Simplest possible lifecycle.

**Alternative considered**: Detached container with separate `test-env-stop` command. Rejected — adds state management (named containers, checking if running) for no benefit.

### Ad-hoc Claude Code session via CLI flags

`just test-claude` launches Claude Code with flags that configure boris as the sole tool provider without any persistent configuration:

```
claude --tools "" --strict-mcp-config --mcp-config '{"mcpServers":{"boris":{"type":"http","url":"http://localhost:8080/mcp"}}}'
```

- `--tools ""` disables all built-in tools (Bash, Read, Edit, etc.)
- `--strict-mcp-config` ignores any persistent MCP configs (`.mcp.json`, `~/.claude.json`)
- `--mcp-config '...'` provides the boris server config inline as JSON

**Rationale**: Fully ad-hoc — no persistent state, no `claude mcp add`, no cleanup needed. The session uses only boris tools and nothing else.

**Alternative considered**: Printing `claude mcp add` instructions for manual setup. Rejected — requires manual cleanup afterwards and is more error-prone.

### Justfile structure

A single `justfile` at the project root. Targets use the `just` conventions: `default` recipe lists all targets via `just --list`, recipes are simple wrappers around the underlying commands with short doc comments.

**Rationale**: `just` is a simple command runner with no dependencies beyond the binary. The justfile is self-documenting and the underlying commands still work without `just` installed.

## Risks / Trade-offs

- **Docker build time**: Building boris from source in Docker takes ~30s on first run. → Mitigated by Docker layer caching; rebuilds after code changes only recompile.
- **Fixed commit goes stale**: The pinned commit (`2cc83a9`) won't reflect new tools/features added later. → Acceptable trade-off for reproducibility. Can bump the commit periodically.
- **`just` not installed**: Developers without `just` can still run the underlying commands directly. → Document this in the justfile header comment.
