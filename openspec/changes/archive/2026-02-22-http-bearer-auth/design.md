## Context

Boris serves MCP tools (including bash execution) over HTTP+SSE on a local port. Currently, any process that can reach the port can issue tool calls with no authentication. The proposal introduces opt-in bearer token auth via `--token <value>` or `--generate-token` flags, applied only to the HTTP transport.

The HTTP handler is set up in `runHTTP()` in `cmd/boris/main.go`. It uses `http.NewServeMux` with two routes: `/mcp` (the MCP streamable HTTP handler) and `GET /health`. The CLI is parsed by kong into a `CLI` struct.

## Goals / Non-Goals

**Goals:**
- Opt-in bearer token authentication on the `/mcp` HTTP endpoint
- Two mechanisms: explicit `--token <value>` and auto-generated `--generate-token`
- Mutual exclusivity enforced at startup
- Constant-time token comparison to prevent timing attacks
- Zero impact on STDIO transport or unauthenticated deployments

**Non-Goals:**
- OAuth2, session tokens, or any stateful auth mechanism
- Token rotation or expiry
- Authentication on the `/health` endpoint
- Rate limiting or brute-force protection (deployment-layer concern)
- TLS termination (deployment-layer concern)

## Decisions

### 1. Standard HTTP middleware wrapping the mux handler

The auth check is implemented as an `http.Handler` middleware that wraps the `/mcp` route handler. When a token is configured, the middleware extracts the `Authorization: Bearer <token>` header and compares it using `crypto/subtle.ConstantTimeCompare`. On mismatch or missing header, it returns `401 Unauthorized` with a JSON error body. On match, it delegates to the inner handler.

**Rationale**: Standard `net/http` middleware is the idiomatic Go approach. It keeps auth logic separate from MCP handler logic and is trivially testable. Using `crypto/subtle.ConstantTimeCompare` prevents timing side-channels.

**Alternative considered**: Implementing auth inside the `mcp.NewStreamableHTTPHandler` factory function. Rejected because the factory returns an `*mcp.Server`, not an HTTP response — there's no clean way to short-circuit with a 401 from inside it.

### 2. Token generation uses `crypto/rand`

`--generate-token` produces a 32-byte random value encoded as hex (64 characters), generated via `crypto/rand`. The token is printed to stderr on startup (stderr because stdout may be used for structured output in some integrations).

**Rationale**: `crypto/rand` is the standard for security-sensitive random generation in Go. 32 bytes (256 bits) of entropy is more than sufficient for a bearer token. Hex encoding is simple, unambiguous, and shell-safe.

### 3. Kong validation for mutual exclusivity

The `CLI` struct adds `Token string` and `GenerateToken bool` fields. A `Validate()` method on the `CLI` struct enforces that both cannot be set simultaneously — kong calls this automatically after parsing. This keeps validation co-located with the CLI definition.

**Rationale**: Kong supports `Validate()` methods on CLI structs natively. This is cleaner than post-parse validation in `main()`.

### 4. Token ignored in STDIO mode

When `--transport=stdio`, the token flags are accepted but have no effect. No warning is emitted — this keeps the configuration surface simple for container deployments that set env vars globally.

**Rationale**: Emitting a warning would be noisy in environments where `BORIS_TOKEN` is set as a blanket env var across both HTTP and STDIO instances. The token simply has no enforcement point in STDIO mode.

## Risks / Trade-offs

- **No brute-force protection** → Mitigation: deployment-layer rate limiting or firewall rules. The token's 256-bit entropy makes brute force infeasible regardless.
- **Token visible in process listing** (`--token` on command line) → Mitigation: recommend `BORIS_TOKEN` env var in documentation. `--generate-token` avoids this entirely since the token is never on the command line.
- **No TLS** → Mitigation: token is only meaningful defense-in-depth for localhost. Network-exposed deployments should use a reverse proxy with TLS. This is documented, not solved.
