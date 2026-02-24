## 1. Path Scoping Accessors

- [x] 1.1 Add `AllowDirs() []string` method to `Resolver` in `internal/pathscope/pathscope.go`
- [x] 1.2 Add `DenyPatterns() []string` method to `Resolver` in `internal/pathscope/pathscope.go`
- [x] 1.3 Add tests for `AllowDirs()` and `DenyPatterns()` in `internal/pathscope/pathscope_test.go`

## 2. Instructions Builder

- [x] 2.1 Add `buildInstructions(workdir string, resolver *pathscope.Resolver) string` function in `cmd/boris/main.go`
- [x] 2.2 Add `instructions` field to `serverConfig` struct, populated at startup via `buildInstructions`
- [x] 2.3 Add unit tests for `buildInstructions` covering: workdir only, workdir + allow dirs, workdir + deny patterns, all three

## 3. Server Options Plumbing

- [x] 3.1 Add `serverOpts` field (type `*mcp.ServerOptions`) to `serverConfig`, built from instructions string
- [x] 3.2 Update `runHTTP` to pass `cfg.serverOpts` instead of `nil` to `mcp.NewServer()`
- [x] 3.3 Update `runSTDIO` to pass `cfg.serverOpts` instead of `nil` to `mcp.NewServer()`

## 4. Integration Test

- [x] 4.1 Add integration test that initializes an MCP server and verifies the initialize response contains the expected instructions string
