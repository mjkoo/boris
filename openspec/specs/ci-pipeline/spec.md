### Requirement: CI test workflow
The project SHALL include a GitHub Actions workflow (`.github/workflows/ci.yml`) that runs on pull requests and pushes to the `main` branch.

#### Scenario: Tests run on PR
- **WHEN** a pull request is opened or updated
- **THEN** the CI workflow SHALL run `go test -race ./...` to execute all tests with the race detector enabled

#### Scenario: Tests run on main push
- **WHEN** a commit is pushed to the `main` branch
- **THEN** the CI workflow SHALL run the same test suite

### Requirement: Go version matrix
The CI workflow SHALL use `actions/setup-go` with Go version `stable` to track the latest stable Go release.

#### Scenario: Go setup
- **WHEN** the CI workflow runs
- **THEN** it SHALL use `actions/setup-go@v5` with `go-version: stable`

### Requirement: Snapshot build verification
The CI workflow SHALL run goreleaser in snapshot mode on pull requests to verify that the goreleaser configuration produces valid builds without publishing.

#### Scenario: Snapshot build on PR
- **WHEN** a pull request is opened or updated
- **THEN** the CI workflow SHALL run `goreleaser release --snapshot --clean`
- **AND** the snapshot build SHALL NOT publish any artifacts or create any releases

#### Scenario: Snapshot build catches config errors
- **WHEN** the goreleaser configuration has an error (e.g., invalid build target)
- **THEN** the snapshot build SHALL fail the CI check before the PR can be merged
