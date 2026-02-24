## ADDED Requirements

### Requirement: Docker image builds boris and sets up known workspace

The test environment SHALL be defined by a Dockerfile at `docker/test-env.dockerfile` that uses a multi-stage build to compile boris from source and create a runtime image with the boris repo checked out at commit `2cc83a9` as the workspace.

#### Scenario: Image builds successfully from project root
- **WHEN** a developer runs `docker build -f docker/test-env.dockerfile .` from the project root
- **THEN** the image builds successfully with boris compiled from the current source

#### Scenario: Runtime image contains known workspace
- **WHEN** the image is run
- **THEN** the directory `/workspace` contains the boris repo at commit `2cc83a9`
- **AND** the boris binary is available and executable

#### Scenario: Runtime image includes required tools
- **WHEN** the image is run
- **THEN** `git`, `bash`, and common CLI tools (grep, find, etc.) are available in PATH

### Requirement: Container runs boris in HTTP mode

The container SHALL run boris in HTTP mode on port 8080 with `/workspace` as the working directory.

#### Scenario: Server starts in foreground and is healthy
- **WHEN** the container is run with `docker run --rm -p 8080:8080`
- **THEN** boris listens on port 8080 in HTTP mode with workdir `/workspace`
- **AND** server logs stream to the terminal
- **AND** `GET /health` returns HTTP 200

#### Scenario: All tools are available
- **WHEN** a client connects to the MCP endpoint
- **THEN** all boris tools (bash, view, str_replace, create_file, grep, find) are available

### Requirement: Ad-hoc Claude Code session with boris as sole tool provider

The test environment SHALL provide a command that launches Claude Code with all built-in tools disabled and boris configured as the only MCP server, without any persistent configuration changes.

#### Scenario: Launching Claude Code against the test environment
- **WHEN** a developer runs `just test-claude` while the test environment is running
- **THEN** Claude Code starts with all built-in tools disabled (`--tools ""`)
- **AND** boris is configured as the only MCP server via `--mcp-config` inline JSON
- **AND** any persistent MCP configurations are ignored (`--strict-mcp-config`)

#### Scenario: No persistent state after session ends
- **WHEN** the Claude Code session started by `just test-claude` is closed
- **THEN** no MCP server configuration has been added to the user's persistent Claude Code settings
