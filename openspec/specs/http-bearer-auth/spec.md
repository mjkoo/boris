### Requirement: Bearer token authentication middleware
When a token is configured (via `--token` or `--generate-token`), the server SHALL require a valid `Authorization: Bearer <token>` header on all HTTP requests to the `/mcp` endpoint. The token comparison SHALL use constant-time comparison (`crypto/subtle.ConstantTimeCompare`) to prevent timing side-channel attacks. Requests with a missing, malformed, or incorrect `Authorization` header SHALL receive a `401 Unauthorized` response with a `Content-Type: application/json` header and a JSON body of `{"error": "unauthorized"}`. Requests with a valid token SHALL be forwarded to the MCP handler unmodified. The `/health` endpoint SHALL NOT require authentication regardless of token configuration.

#### Scenario: Valid bearer token grants access
- **WHEN** a token is configured and a client sends a request to `/mcp` with header `Authorization: Bearer <correct-token>`
- **THEN** the request is forwarded to the MCP handler normally

#### Scenario: Missing authorization header returns 401
- **WHEN** a token is configured and a client sends a request to `/mcp` without an `Authorization` header
- **THEN** the server responds with HTTP 401 and body `{"error": "unauthorized"}`

#### Scenario: Incorrect token returns 401
- **WHEN** a token is configured and a client sends a request to `/mcp` with header `Authorization: Bearer <wrong-token>`
- **THEN** the server responds with HTTP 401 and body `{"error": "unauthorized"}`

#### Scenario: Malformed authorization header returns 401
- **WHEN** a token is configured and a client sends a request to `/mcp` with header `Authorization: Basic <credentials>`
- **THEN** the server responds with HTTP 401 and body `{"error": "unauthorized"}`

#### Scenario: Health endpoint bypasses authentication
- **WHEN** a token is configured and a client sends a GET request to `/health` without an `Authorization` header
- **THEN** the server responds with HTTP 200 and the health check JSON

#### Scenario: No token configured allows unauthenticated access
- **WHEN** no token is configured (neither `--token` nor `--generate-token`)
- **THEN** all requests to `/mcp` are forwarded to the MCP handler without authentication checks

### Requirement: Explicit token via --token flag
The server SHALL accept a `--token` CLI flag (also configurable via `BORIS_TOKEN` environment variable) that sets a specific bearer token value. When provided, the token value SHALL be used directly for authentication.

#### Scenario: Token set via CLI flag
- **WHEN** boris is started with `--token=mysecret`
- **THEN** requests to `/mcp` require `Authorization: Bearer mysecret`

#### Scenario: Token set via environment variable
- **WHEN** boris is started with `BORIS_TOKEN=mysecret` and no `--token` flag
- **THEN** requests to `/mcp` require `Authorization: Bearer mysecret`

### Requirement: Auto-generated token via --generate-token flag
The server SHALL accept a `--generate-token` CLI flag (boolean). When set, the server SHALL generate a cryptographically random token (32 bytes from `crypto/rand`, hex-encoded to 64 characters) and print it to stderr on startup in the format `bearer token: <token>`. The generated token SHALL then be used for authentication identically to an explicit `--token` value.

#### Scenario: Token auto-generated on startup
- **WHEN** boris is started with `--generate-token`
- **THEN** a 64-character hex token is printed to stderr and used for `/mcp` authentication

### Requirement: Mutual exclusivity of token flags
The server SHALL reject startup if both `--token` and `--generate-token` are provided. The validation SHALL occur during CLI parsing and produce a clear error message.

#### Scenario: Both flags provided causes startup error
- **WHEN** boris is started with `--token=mysecret --generate-token`
- **THEN** the server exits with an error message indicating the flags are mutually exclusive

### Requirement: Token flags ignored in STDIO mode
When `--transport=stdio`, the `--token` and `--generate-token` flags SHALL be accepted without error but SHALL have no effect. No authentication middleware is applied in STDIO mode.

#### Scenario: STDIO mode with token configured
- **WHEN** boris is started with `--transport=stdio --token=mysecret`
- **THEN** the server starts normally on stdio without applying authentication
