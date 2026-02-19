# rollbaz

**Generated:** 2026-02-19 | **Branch:** main

## OVERVIEW

Go CLI for fast Rollbar incident triage. Primary workflows are listing active/recent issues, showing issue details by project counter, and rendering either concise human output or machine-readable JSON.

## STRUCTURE

```
rollbaz/
├── cmd/rollbaz/main.go          # CLI entrypoint
├── internal/cli/                # Cobra commands and command wiring
├── internal/app/                # Presentation-agnostic use-case layer
├── internal/rollbar/            # HTTP client and API DTOs
├── internal/config/             # Local config store for project tokens
├── internal/output/             # Human and JSON rendering helpers
├── internal/summary/            # Main-error extraction from payloads
├── internal/redact/             # Token and sensitive value redaction
├── internal/domain/             # Small domain types/newtypes
├── scripts/coveragecheck/       # Coverage gate helper
├── .github/workflows/ci.yml     # CI quality and security gates
└── .golangci.yml                # Linter policy
```

## WHERE TO LOOK

| Task | Location | Notes |
| --- | --- | --- |
| Add CLI command | `internal/cli/root.go` | Keep business logic out of handlers |
| Add triage behavior | `internal/app/service.go` | Stable contracts for future TUI |
| Add Rollbar endpoint | `internal/rollbar/client.go` | Keep redaction and error wrapping |
| Add config behavior | `internal/config/store.go` | Maintain strict file perms |
| Change output format | `internal/output/` | Human + JSON renderers |
| Improve extraction | `internal/summary/extract.go` | Prefer deterministic path order |
| Add tests | `internal/*/*_test.go` | Follow existing direct table-driven style |

## CODE MAP

| Symbol | Type | Location | Role |
| --- | --- | --- | --- |
| `Service` | struct | `internal/app/service.go` | App-layer use-cases for active/recent/show |
| `Client` | struct | `internal/rollbar/client.go` | Rollbar API transport and parsing |
| `Store` | struct | `internal/config/store.go` | Project token config read/write/select |
| `IssueSummary` | struct | `internal/app/service.go` | TUI-friendly list model |
| `IssueDetail` | struct | `internal/app/service.go` | TUI-friendly detail model |
| `RenderIssueListHuman` | fn | `internal/output/issues.go` | Compact list display |
| `RenderJSON` | fn | `internal/output/issues.go` | Stable pretty-JSON output |

## CONVENTIONS

- Strictly avoid logging or returning raw secrets. Always redact token-bearing strings.
- Keep CLI handlers thin; business logic belongs in `internal/app`.
- Keep rendering separate from use-case logic to allow CLI and TUI reuse.
- Use small typed structs for API payloads and view models; avoid `map[string]any` except final output assembly.
- Keep network timeouts explicit and conservative.

## ANTI-PATTERNS

- No `any`/unsafe casts to bypass type issues.
- No broad defensive branches that don't match existing project style.
- No token output in errors, logs, or rendered payloads.
- No skipping tests for edge cases affecting correctness or security posture.

## TESTING

- Run the full gate locally when touching behavior/security:

```bash
go test ./...
go test -race ./...
go test ./internal/... -coverprofile=coverage.out && go run ./scripts/coveragecheck -min 85 -file coverage.out
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
```

- Financial software mindset: include edge cases, empty payloads, parse failures, and redaction checks.

## NOTES

- Project token storage is plaintext local config with strict permissions; treat it as sensitive at rest.
- App-layer view models are intended to be consumed by a future TUI without API/client refactors.
