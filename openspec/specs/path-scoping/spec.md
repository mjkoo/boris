## ADDED Requirements

### Requirement: Path canonicalization
The path resolver SHALL canonicalize all paths before checking allow/deny lists. Canonicalization MUST resolve symbolic links (via `filepath.EvalSymlinks`) and convert to absolute paths (via `filepath.Abs`).

#### Scenario: Symlink resolved before scoping check
- **WHEN** a symlink inside an allowed directory points to a file outside the allowed directory
- **THEN** the resolver denies access because the resolved (real) path is outside the allow list

#### Scenario: Relative path made absolute
- **WHEN** a relative path `src/main.go` is passed with session cwd `/workspace`
- **THEN** the resolver canonicalizes it to `/workspace/src/main.go` before checking allow/deny lists

### Requirement: Allow list enforcement
When one or more `--allow-dir` flags are set, the resolver SHALL verify that the canonicalized path is under one of the allowed directories. If no `--allow-dir` flags are set, all paths SHALL be allowed (canonicalization only).

#### Scenario: Path inside allowed directory
- **WHEN** `--allow-dir=/workspace` is set and the path resolves to `/workspace/src/main.go`
- **THEN** the resolver allows access and returns the canonical path

#### Scenario: Path outside allowed directory
- **WHEN** `--allow-dir=/workspace` is set and the path resolves to `/etc/passwd`
- **THEN** the resolver returns an error indicating the path is outside allowed directories

#### Scenario: No allow list means everything allowed
- **WHEN** no `--allow-dir` flags are set
- **THEN** the resolver allows any path (canonicalization is still performed)

#### Scenario: Multiple allowed directories
- **WHEN** `--allow-dir=/src --allow-dir=/tests` is set
- **THEN** paths under either `/src` or `/tests` are allowed; paths outside both are denied

### Requirement: Deny list enforcement
The resolver SHALL check the canonicalized path against the deny list. Deny MUST take precedence over allow — a path that matches a deny entry is rejected even if it is inside an allowed directory.

#### Scenario: Deny overrides allow
- **WHEN** `--allow-dir=/workspace --deny-dir=/workspace/.env` is set and the path resolves to `/workspace/.env`
- **THEN** the resolver denies access

#### Scenario: Deny with glob pattern
- **WHEN** `--deny-dir='**/.env'` is set and the path resolves to `/workspace/config/.env`
- **THEN** the resolver denies access

#### Scenario: Deny with doublestar glob
- **WHEN** `--deny-dir='**/.git'` is set and the path resolves to `/workspace/project/.git/config`
- **THEN** the resolver denies access (the path is under a directory matching the deny glob)

### Requirement: Glob pattern support via doublestar
Deny entries SHALL support glob patterns using `bmatcuk/doublestar` syntax, including `**/` for matching across directory levels. Allow entries are absolute directory paths only (no globs).

#### Scenario: Doublestar pattern matches nested path
- **WHEN** deny contains `**/.secret` and the path is `/a/b/c/.secret`
- **THEN** the resolver denies access

#### Scenario: Simple glob pattern
- **WHEN** deny contains `/workspace/*.tmp` and the path is `/workspace/data.tmp`
- **THEN** the resolver denies access

### Requirement: File tools use the path resolver
All file tools (`view`, `str_replace`, `create_file`, `grep`) SHALL pass their path arguments through the resolver before performing any filesystem operations. If the resolver returns an error, the tool SHALL return that error to the caller without performing the operation.

#### Scenario: File tool respects deny
- **WHEN** `view` is called with a path that matches a deny entry
- **THEN** the tool returns an access denied error without reading the file

#### Scenario: Grep search root respects allow list
- **WHEN** `grep` is called with a `path` that resolves to a location outside the allowed directories
- **THEN** the tool returns an access denied error without performing any search

#### Scenario: Grep silently skips denied files during traversal
- **WHEN** `grep` is searching a directory and encounters a file matching a deny pattern
- **THEN** the file is silently skipped (no error, no result for that file)

### Requirement: Bash tool does not use the path resolver
The `bash` tool SHALL NOT pass commands through the path resolver. Bash containment is best-effort only — the working directory is set inside an allowed directory, but shell commands can access any path. The `--disable-tools bash` flag is the mechanism for strict file-only enforcement.

#### Scenario: Bash can access paths outside allow list
- **WHEN** `--allow-dir=/workspace` is set and a bash command runs `cat /etc/hostname`
- **THEN** the command executes and returns the file contents (bash is not scoped)

### Requirement: Clear error messages for denied paths
When a path is denied, the resolver SHALL return an error message that indicates whether the denial was due to the path being outside the allow list or matching a deny entry. The error SHALL include the resolved path.

#### Scenario: Error message for outside allow list
- **WHEN** a path resolves to `/etc/passwd` with `--allow-dir=/workspace`
- **THEN** the error message indicates the path is outside the allowed directories

#### Scenario: Error message for deny match
- **WHEN** a path resolves to `/workspace/.env` with `--deny-dir='**/.env'`
- **THEN** the error message indicates the path matched a deny pattern
