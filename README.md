# rollbaz

Fast CLI for Rollbar triage without opening Rollbar.

## Install

```bash
go build ./cmd/rollbaz
```

## Configure Projects

```bash
./rollbaz project add my-service --token <ROLLBAR_PROJECT_TOKEN>
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

## Examples

```bash
# One-time setup
./rollbaz project add my-service --token <READ_TOKEN>
./rollbaz project use my-service

# Daily triage
./rollbaz active --limit 5
# sample: #274 Database timeout while processing webhook | active | env=production | occurrences=6 | last_seen=1731739381

./rollbaz recent --limit 5
# sample: #233 API request failed with upstream 503 | active | env=production | occurrences=83 | last_seen=1731739412

# Deep dive on one issue
./rollbaz show 274
# sample:
# main error: java.sql.SQLTransientConnectionException: Connection timeout
# title: Database timeout while processing webhook
# status: active | environment: production | occurrences: 6
# counter: 274 | item_id: 1234567890

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
