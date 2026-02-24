## ADDED Requirements

### Requirement: Resolver exposes allow dirs via accessor
The `Resolver` SHALL provide an `AllowDirs()` method that returns the canonicalized allow directory list as a `[]string`. If no allow dirs were configured, it SHALL return an empty (or nil) slice.

#### Scenario: AllowDirs returns canonicalized paths
- **WHEN** a `Resolver` is created with `--allow-dir=./workspace` (relative path)
- **THEN** `AllowDirs()` returns the resolved absolute path with symlinks evaluated

#### Scenario: AllowDirs returns empty when unconfigured
- **WHEN** a `Resolver` is created with no allow dirs
- **THEN** `AllowDirs()` returns an empty or nil slice

### Requirement: Resolver exposes deny patterns via accessor
The `Resolver` SHALL provide a `DenyPatterns()` method that returns the deny pattern list as a `[]string`. If no deny patterns were configured, it SHALL return an empty (or nil) slice.

#### Scenario: DenyPatterns returns configured patterns
- **WHEN** a `Resolver` is created with `--deny-dir='**/.env' --deny-dir='**/.git'`
- **THEN** `DenyPatterns()` returns `["**/.env", "**/.git"]`

#### Scenario: DenyPatterns returns empty when unconfigured
- **WHEN** a `Resolver` is created with no deny patterns
- **THEN** `DenyPatterns()` returns an empty or nil slice
