# Boris development task runner
# Prerequisites: just (https://just.systems), Go, Docker (for test-env)
# Run `just` with no arguments to see available recipes.

# List available recipes
default:
    @just --list

# Run tests with race detector
test:
    go test -race ./...

# Run Go static analysis
vet:
    go vet ./...

# Build boris binary for local platform
build:
    go build -o boris ./cmd/boris

# Run goreleaser snapshot build
snapshot:
    goreleaser release --snapshot --clean

# Build and run the Docker test environment (Ctrl-C to stop)
test-env:
    docker build -f docker/test-env.dockerfile -t boris-test-env .
    docker run --rm -p 8080:8080 boris-test-env

# Launch Claude Code with boris as the sole tool provider (test-env must be running)
test-claude:
    claude --tools "" --strict-mcp-config --mcp-config '{"mcpServers":{"boris":{"type":"http","url":"http://localhost:8080/mcp"}}}'
