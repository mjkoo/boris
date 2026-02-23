## Context

Boris is a single-binary Go project (`github.com/mjkoo/boris`) at Go 1.25 with no CI/CD infrastructure. The codebase already follows conventional commits across all 11 existing commits and has a version injection point ready (`var version = "dev"` with ldflags override in `cmd/boris/main.go`). There are no git tags, no changelog, no published binaries.

The goal is a fully automated release pipeline: push conventional commits to main → release-please manages version bumps and changelogs → goreleaser cross-compiles and publishes static binaries to GitHub Releases.

## Goals / Non-Goals

**Goals:**
- Automated versioning and changelog from conventional commits (zero manual steps beyond merging the release PR)
- Cross-platform static binaries for linux/darwin (amd64, arm64) and windows/amd64
- CI that runs tests with race detector on every PR and push to main
- Snapshot builds on PRs to catch goreleaser config issues before merge

**Non-Goals:**
- Container image publishing (can be added later)
- Homebrew tap or other package manager distribution
- Code signing or notarization
- Monorepo/multi-module release coordination (boris is a single module)

## Decisions

### Single workflow architecture for release-please + goreleaser

**Decision**: Use a single GitHub Actions workflow with two jobs — release-please runs first, goreleaser conditionally runs when a release is created.

**Alternatives considered**:
- **Separate workflows** (release-please creates tag, tag-push triggers goreleaser): Requires a Personal Access Token because `GITHUB_TOKEN`-created events don't trigger other workflows. Adds secret management overhead for no benefit.

**Rationale**: The single-workflow approach uses only the built-in `GITHUB_TOKEN`, has no secret management requirements, and keeps the release pipeline in one file that's easy to understand.

### goreleaser `release.mode: keep-existing`

**Decision**: Configure goreleaser with `release.mode: keep-existing` so it attaches build artifacts to the GitHub Release that release-please already created, without overwriting the release notes.

**Rationale**: release-please generates curated release notes from conventional commits. goreleaser's default behavior would overwrite these. `keep-existing` preserves release-please's notes and only adds the binary artifacts.

### goreleaser `changelog.disable: true`

**Decision**: Disable goreleaser's changelog generation entirely.

**Rationale**: release-please manages `CHANGELOG.md`. goreleaser generating its own changelog would be redundant and potentially conflicting.

### Static binaries with CGO_ENABLED=0

**Decision**: Build all binaries with `CGO_ENABLED=0` for fully static linking.

**Rationale**: Boris is a pure Go binary with no CGO dependencies. Static binaries have zero runtime dependencies, work in scratch/distroless containers, and cross-compile trivially without a C toolchain.

### Target platforms: linux, darwin, windows (amd64 + arm64, excluding windows/arm64)

**Decision**: Build for 5 platform/arch combinations: linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64.

**Rationale**: These cover the vast majority of developer machines and CI environments. windows/arm64 is excluded as it's still niche and boris's primary use case (MCP server for coding agents) is overwhelmingly linux/darwin.

### Separate CI workflow

**Decision**: Create a dedicated `ci.yml` workflow for testing (separate from `release.yml`).

**Rationale**: CI concerns (test, lint, build verification) run on every PR and push. Release concerns only run on main pushes. Separating them keeps each workflow focused and makes failures easier to diagnose.

### Pre-v1.0 versioning strategy

**Decision**: Use `bump-minor-pre-major: true` and `bump-patch-for-minor-pre-major: true` in release-please config.

**Rationale**: While pre-v1.0, breaking changes bump minor (0.x.0) instead of major, and features bump patch (0.0.x) instead of minor. This avoids prematurely reaching v1.0 while the API is still stabilizing.

## Risks / Trade-offs

- **[Risk] First release version**: release-please starts with an empty manifest (`{}`). The first release PR will determine v0.1.0 based on commit history. → **Mitigation**: Review the first release PR to confirm the version and changelog look correct before merging.

- **[Risk] goreleaser schema changes**: goreleaser v2 is current but future versions may break config. → **Mitigation**: Pin goreleaser action to `~> v2` (semver range), which will receive compatible updates but not break on major version bumps.

- **[Risk] Binary size**: `-s -w` strips debug info but boris is already ~15MB. Cross-platform archives will add up. → **Mitigation**: Acceptable for now. Can add UPX compression later if needed.

- **[Trade-off] No container images**: Docker users must build their own image. → **Acceptable**: Container publishing is a separate concern and can be added as a future change without modifying the release pipeline.
