# rollbaz

Fast CLI to resolve a Rollbar item counter and print a concise summary.

## Usage

```bash
go build ./cmd/rollbaz
ROLLBAR_ACCESS_TOKEN=your_token ./rollbaz --item 269
```

Required:
- `ROLLBAR_ACCESS_TOKEN`
- `--item <counter>`

## Quality Gates

```bash
prek run --all-files
```
