## Context

Boris currently passes `nil` as the `*mcp.ServerOptions` argument to `mcp.NewServer()`. The MCP SDK supports an `Instructions` field on `ServerOptions` that gets included in the initialize response — clients surface this as model context. Boris has the relevant information (working directory, path scoping config) already resolved at startup but doesn't expose it to connected models.

## Goals / Non-Goals

**Goals:**
- Provide the model with the server's initial working directory and path constraints at connection time, without requiring a tool call.
- Keep the instructions string concise and useful — just what the model needs to construct correct paths.

**Non-Goals:**
- Dynamic instructions that update as the session CWD changes (CWD tracking is already handled by the pwd sentinel in bash tool results).
- Exposing tool-level documentation in instructions (tool descriptions already handle this).
- Making the instructions content configurable via flags.

## Decisions

### Build instructions in `serverConfig`, pass via `ServerOptions`

Add an `instructions` string field to `serverConfig`, built once at startup. Both `runHTTP` and `runSTDIO` pass `&mcp.ServerOptions{Instructions: cfg.instructions}` instead of `nil`.

**Rationale**: The instructions content depends on startup config (workdir, resolver) which is already captured in `serverConfig`. Building it once avoids repeating the logic per transport. The `ServerOptions` struct is the SDK's intended mechanism.

**Alternative considered**: Building the string inside the `getServer` factory (per-session). Rejected because the instructions are static — same workdir and path config for every session.

### Add accessor methods to `Resolver`

Add `AllowDirs() []string` and `DenyPatterns() []string` methods that return the stored slices. These are used by the instructions builder to include canonicalized paths.

**Rationale**: The resolver already canonicalizes allow dirs at construction time. Exposing the canonical values is more accurate than using the raw CLI inputs. Accessor methods are a minimal API surface.

**Alternative considered**: Passing the raw `cli.AllowDir`/`cli.DenyDir` slices directly. Rejected because the resolver may have resolved symlinks or made paths absolute, and we want the instructions to reflect the actual enforced paths.

### Instructions format

The instructions string follows a simple plaintext format:

```
Working directory: /path/to/workdir
Allowed directories: /path/one, /path/two
Denied patterns: **/.env, **/.git
```

The "Allowed directories" and "Denied patterns" lines are only included when the respective lists are non-empty.

**Rationale**: Concise, unambiguous, and easy for the model to parse. No markdown or structured format needed — this is a short context hint, not documentation.

## Risks / Trade-offs

- **Information disclosure**: The instructions expose filesystem paths to the connected model. This is intentional — the model needs this to function — but operators should be aware the workdir and scoping config are visible in the MCP initialize response. This is no different from what a model could discover via a `pwd` bash command.
- **Static vs dynamic**: Instructions are set at server start and don't reflect CWD changes during a session. This is acceptable because the bash tool's pwd sentinel already tracks CWD per-command, and the instructions are specifically for the *initial* working directory context.
