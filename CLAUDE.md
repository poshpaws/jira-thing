# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go mod tidy                             # Sync dependencies
go build -o jira-thing .              # Build binary
go test ./...                          # Run all tests
go test ./internal/api/...            # Run a single package's tests
go test -run TestFetchIssue ./...      # Run a specific test by name
go vet ./...                           # Lint

# CLI usage (after build)
./jira-thing template <KEY> [-o file.json]  # Fetch ticket and save as template
./jira-thing create [-t template.json]      # Create ticket from template
./jira-thing clear-auth                     # Clear stored credentials
```

## Architecture

This is a Go CLI tool (`jira-thing`) for cloning Jira tickets via templates.

**Packages:**
- `main.go` — CLI entry point; subcommand dispatch; `mustConnect`, `fatal`, display helpers
- `internal/api` — `FetchIssue`, `CreateIssue`, `SearchIssues`; shared `executeRequest`/`newAuthRequest` helpers eliminate duplication; `SearchQuery` struct groups search params
- `internal/auth` — credential load/store via OS keyring; `Keyring` interface allows test injection; split into `readCredentials` + `storeCredentials`
- `internal/template` — `Build`/`Save`/`Load` JSON templates

**Flow:** CLI → `api` (with creds from `auth`) → JSON template (via `template`)

Templates are stored as local JSON files (see `ticket_template.json`). No database.

**Testing:** HTTP is mocked with `net/http/httptest`. Keyring is injected via the `Keyring` interface (unexported `backend` var in `internal/auth`). Template tests use `t.TempDir()` for filesystem isolation.

**Docs:** See `docs/architecture.md` for package diagram, data-flow diagram (Mermaid), and design decisions.


## Git discipline
Commit the current working state before starting each new change:
`git add -A && git commit -m "checkpoint: <what exists now>"`

## Quality Target — CodeScene

All code MUST maintain a CodeScene code health score ≥9.0:

- **Function length**: Maximum 30 lines per function.
- **Cyclomatic complexity**: Keep below 10, aim for ≤5.
- **Nesting depth**: Maximum 3 levels.
- **File length**: Maximum 300 lines; split if exceeded.
- **No code duplication**: Zero tolerance for duplicate blocks >6 lines.
- **No bumpy road**: Group related logic, don't alternate conditionals and logic.
- **Single responsibility**: Each function and file does one thing.
- **Excess Number of Function Arguments**: try to reduce arguments , make dataclasses to assist

## documentation 
All code must be documented in a docs directory , Where appropriate, class diagrams should be rendered with mermaid. Function headers should be documented with at least a small description of what they're doing. 
