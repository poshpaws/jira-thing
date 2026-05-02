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

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
