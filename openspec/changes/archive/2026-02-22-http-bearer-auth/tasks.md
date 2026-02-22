## 1. CLI Flags

- [x] 1.1 Add `Token string` and `GenerateToken bool` fields to the `CLI` struct with kong tags, env vars (`BORIS_TOKEN`), and help text
- [x] 1.2 Add `Validate()` method on `CLI` to enforce mutual exclusivity of `--token` and `--generate-token`
- [x] 1.3 Write tests for CLI validation: both flags error, each flag alone works, neither works

## 2. Token Generation

- [x] 2.1 Implement `generateToken()` function that returns 32 bytes from `crypto/rand` hex-encoded to 64 characters
- [x] 2.2 Write test for `generateToken()` verifying length, hex format, and uniqueness across calls

## 3. Auth Middleware

- [x] 3.1 Implement `bearerAuthMiddleware(token string, next http.Handler) http.Handler` that checks `Authorization: Bearer <token>` using `crypto/subtle.ConstantTimeCompare`, returns 401 JSON on failure
- [x] 3.2 Write tests for middleware: valid token passes through, missing header returns 401, wrong token returns 401, malformed auth scheme returns 401

## 4. Server Integration

- [x] 4.1 Wire token resolution in `main()`: resolve token from `--token` or generate via `--generate-token`, print generated token to stderr
- [x] 4.2 Update `runHTTP()` to accept an optional token and wrap the `/mcp` handler with auth middleware when a token is present
- [x] 4.3 Write integration test: start HTTP server with token, verify `/mcp` requires auth and `/health` does not
- [x] 4.4 Write integration test: start HTTP server without token, verify `/mcp` is accessible without auth
