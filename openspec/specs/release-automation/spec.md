### Requirement: release-please configuration
The project SHALL include a `release-please-config.json` that configures release-please for Go release type with the root package (`.`). The config SHALL set `bump-minor-pre-major` and `bump-patch-for-minor-pre-major` to `true` for pre-v1.0 versioning semantics. The project SHALL include a `.release-please-manifest.json` initialized as an empty object (`{}`).

#### Scenario: Config files exist and are valid
- **WHEN** the repository is checked out
- **THEN** `release-please-config.json` SHALL exist at the repository root with `release-type` set to `"go"` and `packages` containing a `"."` entry
- **AND** `.release-please-manifest.json` SHALL exist at the repository root

#### Scenario: Pre-v1.0 version bumps
- **WHEN** a breaking change commit (e.g., `feat!:`) is merged before v1.0.0
- **THEN** release-please SHALL bump the minor version (not major)

### Requirement: GoReleaser configuration
The project SHALL include a `.goreleaser.yaml` that builds the `boris` binary from `./cmd/boris` for linux/{amd64,arm64} and darwin/{amd64,arm64} with `CGO_ENABLED=0` for static linking. Windows is excluded because Boris depends on Unix-only syscalls (`syscall.Kill`, `syscall.SIGTERM`, `syscall.SIGKILL`, `SysProcAttr{Setpgid}`) for process group isolation.

#### Scenario: Static cross-platform builds
- **WHEN** goreleaser runs with the configuration
- **THEN** it SHALL produce statically linked binaries for all 4 target platform/arch combinations (linux/{amd64,arm64}, darwin/{amd64,arm64})
- **AND** binaries SHALL be built with `-trimpath` and `-s -w` ldflags for reproducibility and size reduction

#### Scenario: Version injection via ldflags
- **WHEN** goreleaser builds a binary
- **THEN** it SHALL inject the release version into `main.version` via `-X main.version={{.Version}}`

#### Scenario: Archive format
- **WHEN** goreleaser creates release archives
- **THEN** all archives SHALL use `tar.gz` format
- **AND** archive names SHALL follow the pattern `boris_<version>_<os>_<arch>`

#### Scenario: Changelog delegation to release-please
- **WHEN** goreleaser runs
- **THEN** goreleaser's own changelog generation SHALL be disabled
- **AND** goreleaser SHALL use `release.mode: keep-existing` to preserve release-please's release notes

### Requirement: Release workflow
The project SHALL include a GitHub Actions workflow (`.github/workflows/release.yml`) triggered on pushes to the `main` branch that orchestrates the full release pipeline.

#### Scenario: No release needed
- **WHEN** a push to main does not trigger a release-please release (e.g., release PR is still open)
- **THEN** the workflow SHALL run release-please only
- **AND** the goreleaser job SHALL be skipped

#### Scenario: Release created
- **WHEN** release-please creates a new release (release PR was merged)
- **THEN** the workflow SHALL run goreleaser to build artifacts
- **AND** goreleaser SHALL attach all built archives and a checksums file to the GitHub Release
- **AND** the workflow SHALL use only the built-in `GITHUB_TOKEN` (no PATs required)

### Requirement: Checksum file
GoReleaser SHALL produce a `checksums.txt` file using SHA-256 that covers all release archives.

#### Scenario: Checksums published
- **WHEN** a release is created
- **THEN** a `checksums.txt` file SHALL be attached to the GitHub Release
- **AND** it SHALL contain SHA-256 hashes for every archive in the release
