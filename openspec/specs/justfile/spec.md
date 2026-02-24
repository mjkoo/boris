## Requirements

### Requirement: Default target lists all available targets

The justfile SHALL have a `default` recipe that lists all available targets with descriptions.

#### Scenario: Running just with no arguments
- **WHEN** a developer runs `just` with no arguments
- **THEN** all available recipes are listed with their doc comments

### Requirement: Test target runs unit tests with race detector

The justfile SHALL have a `test` recipe that runs the Go test suite with the race detector enabled.

#### Scenario: Running tests
- **WHEN** a developer runs `just test`
- **THEN** `go test -race ./...` is executed

### Requirement: Vet target runs static analysis

The justfile SHALL have a `vet` recipe that runs Go's static analysis.

#### Scenario: Running vet
- **WHEN** a developer runs `just vet`
- **THEN** `go vet ./...` is executed

### Requirement: Build target compiles local binary

The justfile SHALL have a `build` recipe that compiles boris for the local platform.

#### Scenario: Local build
- **WHEN** a developer runs `just build`
- **THEN** `go build -o boris ./cmd/boris` is executed, producing a `boris` binary in the project root

### Requirement: Snapshot target runs goreleaser snapshot build

The justfile SHALL have a `snapshot` recipe that runs a local goreleaser snapshot build.

#### Scenario: Snapshot build
- **WHEN** a developer runs `just snapshot`
- **THEN** `goreleaser release --snapshot --clean` is executed

### Requirement: Test environment targets

The justfile SHALL have `test-env` and `test-claude` recipes for the Claude Code test environment.

#### Scenario: Starting the test environment
- **WHEN** a developer runs `just test-env`
- **THEN** the Docker image is built from `docker/test-env.dockerfile`
- **AND** the container runs in the foreground with port 8080 mapped and `--rm`
- **AND** server logs stream to the terminal
- **AND** Ctrl-C stops and removes the container

#### Scenario: Launching Claude Code against the test environment
- **WHEN** a developer runs `just test-claude`
- **THEN** Claude Code is launched with `--tools "" --strict-mcp-config --mcp-config` pointing to `http://localhost:8080/mcp`
