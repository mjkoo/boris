## Why

Boris has no release automation. Builds are manual (`go build`), there are no git tags, no changelog, and no published artifacts. Users have no way to download a pre-built binary. We need an automated release pipeline that produces cross-platform static binaries from conventional commits with zero manual intervention beyond merging a PR.

## What Changes

- Add release-please configuration to automate version bumping and changelog generation from conventional commits
- Add GoReleaser configuration to cross-compile static binaries for linux/darwin/windows on amd64/arm64
- Add a GitHub Actions workflow that orchestrates the full pipeline: release-please creates the release, goreleaser builds and attaches artifacts
- Add a CI workflow for running tests and snapshot builds on PRs

## Capabilities

### New Capabilities
- `release-automation`: Automated release pipeline — release-please creates versioned GitHub releases from conventional commits, goreleaser cross-compiles and attaches static binaries, all orchestrated by GitHub Actions
- `ci-pipeline`: Continuous integration — run tests with race detector on PRs and pushes to main, plus goreleaser snapshot builds to catch build issues before merge

### Modified Capabilities
_(none — this is purely additive infrastructure, no existing behavior changes)_

## Impact

- **New files**: `.github/workflows/release.yml`, `.github/workflows/ci.yml`, `.goreleaser.yaml`, `release-please-config.json`, `.release-please-manifest.json`
- **No code changes**: The existing `var version = "dev"` with ldflags override in `cmd/boris/main.go` already supports version injection — goreleaser will use this as-is
- **Git conventions**: Repository already uses conventional commits (all 11 existing commits follow `feat:` / `fix:` prefixes) — no workflow change required
- **Dependencies**: No new Go dependencies. GitHub Actions are all third-party actions (googleapis/release-please-action@v4, goreleaser/goreleaser-action@v7, actions/checkout@v4, actions/setup-go@v5)
- **Secrets**: No PATs or additional secrets needed — the single-workflow architecture uses the built-in `GITHUB_TOKEN`
