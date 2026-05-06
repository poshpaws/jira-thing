# Architecture

## Overview

`jira-thing` is a CLI tool for templating and creating Jira tickets. It is structured in three internal packages consumed by a single `main` package.

```
CLI (main) ──► internal/api      ──► Jira REST API (HTTPS)
           ──► internal/auth     ──► OS Keyring
           ──► internal/template ──► Local JSON files
```

## Package Responsibilities

| Package | Responsibility |
|---|---|
| `main` | Subcommand routing, user I/O, flag parsing |
| `main` (`render.go`) | ADF→markdown conversion; glamour terminal rendering |
| `internal/api` | HTTP requests to the Jira REST API |
| `internal/auth` | Credential load/store via the OS keyring |
| `internal/template` | Build, save, and load JSON ticket templates |

## Class Diagram

```mermaid
classDiagram
    class JiraConnection {
        +string BaseURL
        +string Email
        +string APIToken
    }

    class SearchQuery {
        +string JQL
        +[]string Fields
        +int MaxResults
    }

    class SearchResult {
        +[]map Issues
        +int Total
        +int MaxResults
    }

    class Comment {
        +map Author
        +map Body
        +string Created
    }

    class CommentResult {
        +[]Comment Comments
        +int Total
    }

    CommentResult "1" --> "*" Comment : contains

    class Keyring {
        <<interface>>
        +Get(service, key) string, error
        +Set(service, key, value) error
        +Delete(service, key) error
    }

    class systemKeyring {
        +Get(service, key) string, error
        +Set(service, key, value) error
        +Delete(service, key) error
    }

    systemKeyring ..|> Keyring : implements
    JiraConnection ..> SearchQuery : used with
    SearchQuery --> SearchResult : produces
```

## Data Flow

```mermaid
flowchart TD
    CLI["main.go\n(subcommand dispatch)"]

    CLI -->|"GetCredentials()"| Auth["internal/auth\nload / prompt / store"]
    CLI -->|"FetchIssue()"| API["internal/api\nHTTP client"]
    CLI -->|"CreateIssue()"| API
    CLI -->|"SearchIssues()"| API
    CLI -->|"AddComment()"| API
    CLI -->|"FetchLastComment()"| API
    CLI -->|"Build / Save / Load"| Tmpl["internal/template\nJSON serialisation"]
    CLI -->|"renderLastComment()"| Render["render.go\nADF→markdown + glamour"]

    Auth -->|OS keyring calls| KR[(OS Keyring)]
    API  -->|REST over HTTPS| Jira[(Jira Cloud)]
    Tmpl -->|ReadFile / WriteFile| FS[(Local JSON files)]
```

## Commands

| Command | Description |
|---|---|
| `template <KEY> [-o file]` | Fetch a ticket, extract reusable fields, write JSON template |
| `create [-t file]` | Load a template, prompt for summary/description, create ticket |
| `update <KEY> [-stdin]` | Add a comment via `$EDITOR` or stdin |
| `last-comment <KEY>` | Fetch and render the most recent comment as markdown |
| `my-tasks [-notupdated]` | List open tickets assigned to `currentUser()`; `-notupdated` filters to tickets idle for 3+ business days |
| `clear-auth` | Delete all stored credentials from the OS keyring |

## Key Design Decisions

- **No database** — templates are standalone JSON files on disk.
- **`Keyring` interface** — allows unit tests to inject an in-memory mock without touching the OS keyring.
- **`executeRequest` helper** — eliminates duplicated HTTP status-check + JSON-decode logic across all three API functions.
- **`SearchQuery` struct** — groups the four search parameters to keep `SearchIssues` within the single-responsibility / argument-count guidelines.
- **ADF→markdown conversion** (`render.go`) — Jira Cloud API v3 returns comment bodies as Atlassian Document Format (ADF), not raw markdown. `adfToMarkdown` recursively walks the ADF node tree to produce standard markdown, which is then rendered to styled terminal output via [glamour](https://github.com/charmbracelet/glamour).
- **Two-call last-comment fetch** — `FetchLastComment` first requests one comment to read `total`, then requests `startAt=total-1` to fetch the last without downloading all comments.
