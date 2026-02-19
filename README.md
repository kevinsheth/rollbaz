# rollbaz

Fast CLI for Rollbar triage without opening Rollbar.

## Install

Download a prebuilt binary from GitHub Releases:

1. Open `https://github.com/kevinsheth/rollbaz/releases`
2. Download the archive matching your OS/arch (for example `rollbaz_v0.1.0_darwin_arm64.tar.gz`)
3. Extract it and move `rollbaz` onto your `PATH`

Or install from source:

```bash
go install github.com/kevinsheth/rollbaz/cmd/rollbaz@latest
rollbaz --help

# local build
go build ./cmd/rollbaz
./rollbaz --help
```

## Configure Projects

Use a Rollbar project token with read access.

```bash
rollbaz project add my-service --token '<ROLLBAR_PROJECT_TOKEN>'
rollbaz project list
rollbaz project use my-service
```

Tokens are stored in your user config directory with strict file permissions (`0600`).

## Core Commands

```bash
rollbaz                 # default: list active issues for active project
rollbaz active --limit 20
rollbaz recent --limit 20
rollbaz show 274
```

Use `--format json` on list and show commands for LLM-friendly machine output.

List filters (for `rollbaz`, `active`, and `recent`):

```bash
--env <environment>
--status <status>
--since <RFC3339-or-unix-seconds>
--until <RFC3339-or-unix-seconds>
--min-occurrences <count>
--max-occurrences <count>
```

Human output uses pretty terminal tables and UTC timestamps (`RFC3339`).
In interactive terminals, commands show a short progress indicator while data is loading.
Tables auto-size to terminal width and keep a consistent layout across commands.
Long fields are truncated in human mode to keep output readable.

## Examples

```bash
# One-time setup
rollbaz project add my-service --token '<READ_TOKEN>'
rollbaz project use my-service

# Daily triage
rollbaz active --limit 5
rollbaz recent --limit 5

# filtered triage
rollbaz active --env production --status active --since 2026-02-19T00:00:00Z --min-occurrences 5

# Deep dive on one issue
rollbaz show 274

# LLM-readable output
rollbaz show 274 --format json
rollbaz active --format json --limit 20
rollbaz recent --format json --limit 20
```

## Token Resolution

Token precedence:
1. `--token`
2. configured `--project` token
3. active configured project token
4. `ROLLBAR_ACCESS_TOKEN`

## Quality Gates

```bash
go test ./...
go test -race ./...
go test ./internal/... -coverprofile=coverage.out && go run ./scripts/coveragecheck -min 85 -file coverage.out
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
```

## Releasing

Releases are automated with Release Please + GoReleaser.

1. Merge feature PRs into `main`.
2. When you are ready to release, run the `release-please` workflow manually.
3. It opens/updates a Release PR.
4. Merge the Release PR.
5. Release Please creates a semver tag.
6. The `release` workflow runs GoReleaser and publishes cross-platform binaries to GitHub Releases.

If you need to force a specific version, run the `release-please` workflow manually and set `release_as`.

```bash
# optional: force a release version from GitHub Actions workflow_dispatch
# release_as: 1.2.3
```
