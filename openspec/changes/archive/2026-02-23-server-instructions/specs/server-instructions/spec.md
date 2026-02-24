## ADDED Requirements

### Requirement: Server instructions include initial working directory
The server SHALL build an instructions string that includes the initial working directory (the resolved absolute path from `--workdir`). This string SHALL be provided to `mcp.NewServer()` via `ServerOptions.Instructions` so that it appears in the MCP initialize response.

#### Scenario: Instructions contain working directory
- **WHEN** boris is started with `--workdir=/workspace`
- **THEN** the MCP initialize response contains an `instructions` field that includes "Working directory: /workspace"

#### Scenario: Default working directory in instructions
- **WHEN** boris is started without `--workdir` (defaults to ".")
- **THEN** the instructions contain the resolved absolute path of the current directory

### Requirement: Server instructions include allowed directories when configured
When one or more `--allow-dir` flags are set, the instructions string SHALL include the canonicalized allowed directory list.

#### Scenario: Instructions include allowed directories
- **WHEN** boris is started with `--allow-dir=/workspace --allow-dir=/data`
- **THEN** the instructions include "Allowed directories: /workspace, /data"

#### Scenario: No allowed directories omits the line
- **WHEN** boris is started without any `--allow-dir` flags
- **THEN** the instructions do not contain an "Allowed directories" line

### Requirement: Server instructions include denied patterns when configured
When one or more `--deny-dir` flags are set, the instructions string SHALL include the deny pattern list.

#### Scenario: Instructions include denied patterns
- **WHEN** boris is started with `--deny-dir='**/.env' --deny-dir='**/.git'`
- **THEN** the instructions include "Denied patterns: **/.env, **/.git"

#### Scenario: No denied patterns omits the line
- **WHEN** boris is started without any `--deny-dir` flags
- **THEN** the instructions do not contain a "Denied patterns" line

### Requirement: Instructions built once at startup
The instructions string SHALL be built once during server startup and stored in the server configuration. The same instructions SHALL be used for all sessions (HTTP mode) and the single session (STDIO mode).

#### Scenario: All HTTP sessions receive the same instructions
- **WHEN** two clients connect to the HTTP server
- **THEN** both receive identical instructions in their initialize responses
