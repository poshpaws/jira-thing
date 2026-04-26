# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go mod tidy                             # Sync dependencies
go build -o jira-client .              # Build binary
go test ./...                          # Run all tests
go test ./internal/api/...            # Run a single package's tests
go test -run TestFetchIssue ./...      # Run a specific test by name
go vet ./...                           # Lint

# CLI usage (after build)
./jira-client template <KEY> [-o file.json]  # Fetch ticket and save as template
./jira-client create [-t template.json]      # Create ticket from template
./jira-client clear-auth                     # Clear stored credentials
```

## Architecture

This is a Go CLI tool (`jira-client`) for cloning Jira tickets via templates.

**Packages:**
- `main.go` — CLI entry point; manual subcommand dispatch using `flag` and `os.Args`
- `internal/api` — Jira REST API calls (`FetchIssue`, `CreateIssue`) using `net/http`
- `internal/auth` — credential management via OS keyring (`github.com/zalando/go-keyring`); prompts if missing; `Keyring` interface allows test injection
- `internal/template` — `Build`/`Save`/`Load` JSON templates from Jira issue fields

**Flow:** CLI → `api` (with creds from `auth`) → JSON template (via `template`)

Templates are stored as local JSON files (see `ticket_template.json`). No database.

**Testing:** HTTP is mocked with `net/http/httptest`. Keyring is injected via the `Keyring` interface (unexported `backend` var in `internal/auth`). Template tests use `t.TempDir()` for filesystem isolation.
