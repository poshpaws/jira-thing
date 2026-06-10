# Copilot Instructions

## Build, Test, and Lint

```bash
make build      # compile binary
make test       # run all tests with coverage
make lint       # go vet + gofmt + staticcheck
make security   # gosec + govulncheck
make all        # lint → security → test → clean → build
```

Run a single test:
```bash
go test -run TestThreeBusinessDaysAgo ./...
go test -run TestRunMyTasks ./internal/api/...
```

Requires Go 1.24+. External lint tools (`staticcheck`, `gosec`, `govulncheck`) must be installed separately via `make tools`.

## Architecture

The codebase is a single-binary Go CLI (`module jira-thing`) with three internal packages:

- **`main.go`** — command dispatch (`template`, `create`, `my-tasks`, `update`, `last-comment`, `clear-auth`), flag parsing, and top-level wiring
- **`render.go`** — converts Jira's Atlassian Document Format (ADF) node trees to Markdown, then renders with `charmbracelet/glamour`
- **`internal/api/`** — thin Jira Cloud REST API v3 client (`/rest/api/3/issue`, `/rest/api/3/search/jql`); all requests use Basic Auth (email + API token)
- **`internal/auth/`** — OS keyring credential store via `go-keyring`; prompts interactively on first use
- **`internal/template/`** — builds, saves, and loads JSON ticket templates; extracts standard fields plus any `customfield_*` keys from a fetched issue

Data flow for the primary workflow:
`template` command → `api.FetchIssue` → `template.Build` (strips to reusable fields) → JSON file on disk → `template.Load` → `api.CreateIssue`

## Key Conventions

### Testability via package-level var injection
Externally-visible side effects are abstracted as package-level `var` so tests can replace them without build tags:

| Variable | Package | Purpose |
|---|---|---|
| `getCredentialsFn` | `main` | swapped in tests to avoid OS keyring |
| `osExit` | `main` | replaced with a panic-based sentinel to test `fatal()` calls |
| `backend` | `internal/auth` | replaced with a mock `Keyring` implementation |
| `readPassword` | `internal/auth` | replaced to avoid terminal prompts |
| `CandidatePathsFunc` | `internal/template` | overrides template search path in tests |

### Testing `os.Exit`
Tests use `captureExit` (in `main_test.go`), which replaces `osExit` with a function that panics with an `exitSignal` struct, recovered via `defer`. Never call `os.Exit` directly — always use the `osExit` var.

### ADF (Atlassian Document Format)
All text sent to Jira (descriptions, comments) must be wrapped in ADF structure. Use `buildDescription(text string)` in `main.go` — it wraps plain text into the required `doc → paragraph → text` node tree. Avoid constructing raw ADF manually elsewhere.

### Template fields
`template.Build` captures `project`, `issuetype`, `priority`, `labels`, `components`, `assignee`, and any `customfield_*` keys. The `summary` and `description` fields are intentionally excluded from templates and always provided at create time.

### HTTP testing
Use `httptest.NewServer` for all API-level tests — no real Jira connection is needed. Mock credentials with `mockCreds(srv.URL)` (returns a cleanup func via the deferred-restore pattern).

### Module path
The Go module is named `jira-thing` (not a URL). Imports look like `jira-thing/internal/api`.
