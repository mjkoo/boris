## Why

There's no way to quickly spin up a real Claude Code session backed by boris as its sole tool provider. This makes it hard to manually QA features, reproduce bugs reported by users, and demo boris in action. Additionally, common developer workflows (run tests, build, snapshot release) aren't codified anywhere — each developer has to remember the incantations.

## What Changes

- Add a Dockerfile (`docker/test-env.dockerfile`) that builds boris from source, checks out the boris repo itself at a fixed commit (`2cc83a9`) as the workspace, and runs the server in HTTP mode
- Add a justfile with:
  - `default` — lists all available targets (justfile best practice)
  - `test` — run `go test -race ./...`
  - `vet` — run `go vet ./...`
  - `build` — local dev build (`go build -o boris ./cmd/boris`)
  - `snapshot` — local goreleaser snapshot build (`goreleaser release --snapshot --clean`)
  - `test-env` — build the Docker image and run boris in the foreground (Ctrl-C to stop)
- Document the test environment workflow in the README or the justfile itself

## Capabilities

### New Capabilities
- `test-environment`: Docker-based manual test environment for exercising boris through Claude Code against a known codebase
- `justfile`: Task runner codifying common developer workflows (test, build, snapshot, test environment lifecycle)

### Modified Capabilities

_(none — no changes to existing spec-level behavior)_

## Impact

- **New files**: `docker/test-env.dockerfile`, `justfile`, optional docker-compose or helper script
- **Dependencies**: Docker required for test environment; `just` required for task runner (both optional — underlying commands still work directly)
- **No changes to boris source code** — purely additive developer tooling
