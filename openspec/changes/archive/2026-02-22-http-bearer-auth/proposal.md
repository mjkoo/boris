## Why

Boris exposes bash execution and file operations over HTTP on a local port. Without authentication, any process on the machine — including browser-based DNS rebinding attacks and SSRF from other local services — can connect and execute arbitrary commands. An opt-in bearer token provides defense-in-depth for users who want to secure the HTTP transport without requiring an external reverse proxy.

## What Changes

- Add `--token` CLI flag and `BORIS_TOKEN` environment variable to set a specific shared secret bearer token
- Add `--generate-token` CLI flag (bool) to auto-generate a random token on startup and print it to stderr
- `--generate-token` and `--token` are mutually exclusive; providing both is an error
- When a token is active (via either mechanism), require `Authorization: Bearer <token>` on all HTTP requests to `/mcp`
- Reject unauthenticated or incorrectly authenticated requests with `401 Unauthorized`
- The `/health` endpoint remains unauthenticated (it exposes no sensitive data and is needed for orchestration probes)
- STDIO transport ignores token settings entirely (no network surface)
- Token auth is opt-in: if neither `--token` nor `--generate-token` is set, behavior is unchanged (no authentication required)

## Capabilities

### New Capabilities
- `http-bearer-auth`: Opt-in bearer token authentication for the HTTP transport, enforced as HTTP middleware wrapping the `/mcp` endpoint

### Modified Capabilities
- `mcp-server`: The HTTP server setup gains optional authentication middleware that wraps the `/mcp` handler when a token is configured

## Impact

- **CLI**: New `--token` / `BORIS_TOKEN` and `--generate-token` flags added to the kong CLI struct
- **HTTP handler**: Authentication middleware inserted between the mux and the `/mcp` handler
- **No breaking changes**: Entirely opt-in; default behavior (no auth) is preserved
- **No new dependencies**: Uses standard `net/http` middleware and constant-time string comparison from `crypto/subtle`
