## 1. release-please Configuration

- [x] 1.1 Create `release-please-config.json` at repo root with `release-type: "go"`, `bump-minor-pre-major: true`, `bump-patch-for-minor-pre-major: true`, and `packages: { ".": { "changelog-path": "CHANGELOG.md" } }`
- [x] 1.2 Create `.release-please-manifest.json` at repo root as an empty object (`{}`)

## 2. GoReleaser Configuration

- [x] 2.1 Create `.goreleaser.yaml` at repo root with: `version: 2`, build from `./cmd/boris`, `CGO_ENABLED=0`, targets linux/{amd64,arm64} + darwin/{amd64,arm64} + windows/amd64, ldflags injecting `main.version`, `-s -w -trimpath`, tar.gz archives (zip for windows), `checksums.txt` with sha256, `changelog.disable: true`, `release.mode: keep-existing`

## 3. GitHub Actions Workflows

- [x] 3.1 Create `.github/workflows/release.yml` with two jobs: (1) `release-please` job using `googleapis/release-please-action@v4` outputting `release_created` and `tag_name`, (2) `goreleaser` job conditioned on `release_created`, using `actions/checkout@v4` with `fetch-depth: 0`, `actions/setup-go@v5` with `go-version: stable`, and `goreleaser/goreleaser-action@v7` with `args: release --clean`. Workflow triggered on push to `main`, permissions for `contents: write` and `pull-requests: write`.
- [x] 3.2 Create `.github/workflows/ci.yml` triggered on pull requests and pushes to main, with jobs for: (1) `test` — checkout, setup-go, `go test -race ./...`, (2) `snapshot` — checkout with `fetch-depth: 0`, setup-go, goreleaser snapshot build (`release --snapshot --clean`) on PRs only

## 4. Verification

- [x] 4.1 Run `goreleaser check` to validate `.goreleaser.yaml` syntax
- [x] 4.2 Run `goreleaser release --snapshot --clean` locally to verify cross-platform builds succeed
- [x] 4.3 Verify built binaries report correct version info (spot check one binary with `./boris --version` or similar)
