## 1. Project Setup & Dependencies

- [x] 1.1 Add dependencies: `github.com/modelcontextprotocol/go-sdk`, `github.com/alecthomas/kong`, `github.com/bmatcuk/doublestar/v4`
- [x] 1.2 Create package directory structure: `internal/session/`, `internal/pathscope/`, `internal/tools/`

## 2. Session State

- [x] 2.1 Implement `internal/session/session.go`: `Session` struct with `Cwd() string` and `SetCwd(string)` methods, protected by `sync.Mutex`
- [x] 2.2 Write tests for `Session`: concurrent access, initial cwd, set/get round-trip

## 3. Path Scoping

- [x] 3.1 Implement `internal/pathscope/pathscope.go`: `Resolver` struct with `NewResolver(allowDirs []string, denyPatterns []string) (*Resolver, error)` and `Resolve(baseCwd string, path string) (string, error)` method — canonicalize via `filepath.EvalSymlinks` + `filepath.Abs`, check allow list (prefix match), check deny list (doublestar glob match, deny > allow)
- [x] 3.2 Write tests for `Resolver`: no allow dirs (everything allowed), single allow dir, multiple allow dirs, path outside allow, deny overrides allow, deny with `**/` glob, deny with simple glob, symlink resolution, relative path resolution, clear error messages for both denial types

## 4. Bash Tool

- [x] 4.1 Implement `internal/tools/bash.go`: `BashArgs` struct, handler function that executes `cd <cwd> && <command> ; echo '<sentinel>' ; pwd` via `/bin/sh -c`, collects stdout/stderr, parses sentinel to extract new cwd, strips sentinel lines from output, returns `stdout`, `stderr`, `exit_code` as text content
- [x] 4.2 Implement timeout with process group kill: set `SysProcAttr{Setpgid: true}`, use `time.AfterFunc` to `syscall.Kill(-pgid, syscall.SIGKILL)` on timeout, include partial output and timeout indication in response
- [x] 4.3 Write tests for bash tool: simple command execution, non-zero exit code, stderr capture, cwd tracking across calls (cd then pwd), sentinel stripping, timeout kills process and returns partial output, missing sentinel preserves cwd, initial workdir from session

## 5. View Tool

- [x] 5.1 Implement `internal/tools/view.go`: `ViewArgs` struct with `Path` and `ViewRange`, handler that resolves path via session cwd + pathscope resolver, detects file vs directory
- [x] 5.2 Implement file reading: line number prefixes, `view_range` support (1-indexed, validate bounds), large file truncation at 2000 lines with message, binary file detection (check first 512 bytes for NUL), max file size check, symlink following
- [x] 5.3 Implement directory listing: 2 levels deep, exclude hidden files (`.` prefix) and `node_modules`, format as tree-style listing
- [x] 5.4 Write tests for view tool: read entire file with line numbers, read line range, invalid view_range errors, large file truncation, binary file detection, directory listing with exclusions, relative path resolution, path scoping enforcement, file not found error, max file size error

## 6. str_replace Tool

- [x] 6.1 Implement `internal/tools/str_replace.go`: `StrReplaceArgs` struct with `Path`, `OldStr`, `NewStr`, handler that resolves path via session cwd + pathscope resolver, reads file, counts occurrences of `old_str`, replaces if exactly 1 match, preserves file permissions, returns context snippet around replacement
- [x] 6.2 Write tests for str_replace: successful replacement with context snippet, string not found error, multiple occurrences error with count, deletion (empty new_str), file permissions preserved, path scoping enforcement, file not found error

## 7. create_file Tool

- [x] 7.1 Implement `internal/tools/create_file.go`: `CreateFileArgs` struct with `Path`, `Content`, `Overwrite`, handler that resolves path via session cwd + pathscope resolver, creates parent dirs (0755), writes file (0644), checks overwrite flag, checks max file size
- [x] 7.2 Write tests for create_file: create new file, refuse overwrite when not allowed, overwrite when allowed, parent directory creation, file permissions 0644, max file size enforcement, path scoping enforcement

## 8. Tool Registration

- [x] 8.1 Implement `internal/tools/tools.go`: `RegisterAll(server *mcp.Server, resolver *pathscope.Resolver, session *session.Session, cfg Config)` function that registers all tools via `mcp.AddTool`, conditionally skip bash when `NoBash` is set, pass max file size and default timeout to handlers

## 9. CLI & Server Wiring

- [x] 9.1 Implement kong CLI struct in `cmd/boris/main.go`: define all flags with `kong` tags and `env` bindings, `--transport` as enum (`http`/`stdio`), `--allow-dir` and `--deny-dir` as `[]string` slices, `--max-file-size` with size parsing
- [x] 9.2 Wire up main function: parse CLI via kong, create session (with resolved workdir), create pathscope resolver (from allow/deny flags), create MCP server with `mcp.NewServer`, call `tools.RegisterAll`, branch on transport flag
- [x] 9.3 Implement HTTP transport path: create `mcp.NewStreamableHTTPHandler`, register `GET /health` handler (returns 200 + JSON), create `http.ServeMux`, listen on configured port with log output
- [x] 9.4 Implement STDIO transport path: call `server.Run(ctx, &mcp.StdioTransport{})`, ignore port flag

## 10. Build

- [ ] ~~10.1 Create Makefile with `build` target (CGO_ENABLED=0, local OS/arch), `build-all` target cross-compiling for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, output to `dist/` directory~~ (deferred per user request)

## 11. Integration Testing

- [x] 11.1 Write an integration test that starts boris in-process using `mcp.InMemoryTransport`, connects an MCP client, and exercises the tool lifecycle: create_file → view → str_replace → view (verify change) → bash (run a command in the workspace)
