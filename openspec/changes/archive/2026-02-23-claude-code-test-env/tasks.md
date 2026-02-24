## 1. Docker Test Environment

- [x] 1.1 Create `docker/test-env.dockerfile` with multi-stage build: builder stage compiles boris from source, runtime stage (`debian:bookworm-slim`) installs git/bash/coreutils, clones boris repo at `2cc83a9` into `/workspace`, copies binary, sets entrypoint to boris in HTTP mode on port 8080 with workdir `/workspace`
- [x] 1.2 Verify the image builds and the health endpoint responds: `docker build -f docker/test-env.dockerfile .` then `docker run --rm -p 8080:8080 <image>` and `curl localhost:8080/health`

## 2. Justfile

- [x] 2.1 Create `justfile` at project root with `default` recipe that runs `just --list`
- [x] 2.2 Add `test` recipe: `go test -race ./...`
- [x] 2.3 Add `vet` recipe: `go vet ./...`
- [x] 2.4 Add `build` recipe: `go build -o boris ./cmd/boris`
- [x] 2.5 Add `snapshot` recipe: `goreleaser release --snapshot --clean`
- [x] 2.6 Add `test-env` recipe: build Docker image from `docker/test-env.dockerfile`, run container in foreground with `--rm -p 8080:8080` (Ctrl-C to stop)
- [x] 2.7 Add `test-claude` recipe: run `claude --tools "" --strict-mcp-config --mcp-config '{"mcpServers":{"boris":{"type":"http","url":"http://localhost:8080/mcp"}}}'`

## 3. Documentation

- [x] 3.1 Add test environment usage section to README or justfile doc comments covering: prerequisites (Docker, just, claude), workflow (`just test-env` in one terminal → `just test-claude` in another → exercise → Ctrl-C to stop)
