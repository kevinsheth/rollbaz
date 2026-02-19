# rollbaz

Fast CLI for Rollbar triage without opening Rollbar.

## Install

```bash
go build ./cmd/rollbaz
```

## Configure Projects

Use a Rollbar project token with read access.

```bash
./rollbaz project add my-service --token '<ROLLBAR_PROJECT_TOKEN>'
./rollbaz project list
./rollbaz project use my-service
```

Tokens are stored in your user config directory with strict file permissions (`0600`).

## Core Commands

```bash
./rollbaz                 # default: list active issues for active project
./rollbaz active --limit 20
./rollbaz recent --limit 20
./rollbaz show 274
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
./rollbaz project add my-service --token '<READ_TOKEN>'
./rollbaz project use my-service

# Daily triage
./rollbaz active --limit 5
./rollbaz recent --limit 5

# filtered triage
./rollbaz active --env production --status active --since 2026-02-19T00:00:00Z --min-occurrences 5

# Deep dive on one issue
./rollbaz show 274

# LLM-readable output
./rollbaz show 274 --format json
./rollbaz active --format json --limit 20
./rollbaz recent --format json --limit 20
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
