# rollbaz

Fast CLI for Rollbar triage without opening Rollbar.

## Install

```bash
go build ./cmd/rollbaz
```

## Configure Projects

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

Human output uses pretty terminal tables and UTC timestamps (`RFC3339`).
In interactive terminals, commands show a short progress indicator while data is loading.
Long fields are truncated in human mode to keep tables readable.

## Examples

```bash
# One-time setup
./rollbaz project add my-service --token '<READ_TOKEN>'
./rollbaz project use my-service

# Daily triage
./rollbaz active --limit 5
# sample row: #274   active   production   6   2024-11-16T12:43:01Z   Database timeout while processing webhook

./rollbaz recent --limit 5
# sample row: #233   active   production   83  2024-11-16T12:43:32Z   API request failed with upstream 503

# Deep dive on one issue
./rollbaz show 274
# sample: Main Error: java.sql.SQLTransientConnectionException: Connection timeout
#         followed by a table with title/status/environment/occurrences/counter/item id

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
