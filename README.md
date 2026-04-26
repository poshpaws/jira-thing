# jira-thing

A CLI tool for cloning Jira tickets via reusable JSON templates. Capture an existing ticket's structure once, then stamp out new tickets from it interactively.

## Installation

### Pre-built binaries

Download the latest release for your platform from the [Releases](../../releases) page:

| Platform | File |
|---|---|
| macOS (Apple Silicon) | `jira-thing-darwin-arm64` |
| macOS (Intel) | `jira-thing-darwin-amd64` |
| Windows | `jira-thing-windows-amd64.exe` |

Make the binary executable (macOS/Linux):

```bash
chmod +x jira-thing-darwin-arm64
mv jira-thing-darwin-arm64 /usr/local/bin/jira-thing
```

### Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/poshpaws/jira-thing.git
cd jira-thing
make build
```

## Authentication

On first use, `jira-thing` will prompt for your Jira credentials and store them securely in the OS keychain (macOS Keychain, Windows Credential Manager, or Linux Secret Service).

```
Jira base URL (e.g. https://yourorg.atlassian.net): https://yourorg.atlassian.net
Jira email: you@example.com
Jira API token: ••••••••••••••••
Credentials stored securely in keyring.
```

**Generating an API token:** Go to [https://id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens) and create a token. Use your Atlassian account email as the username.

Credentials are only prompted once. To update or remove them:

```bash
jira-thing clear-auth
```

## Commands

### `template` — capture a ticket as a template

NOTE: you need a template for the create ticket to work.


Fetches an existing Jira issue and saves its reusable fields (project, issue type, priority, labels, components, assignee) as a local JSON file.

```bash
jira-thing template <TICKET-KEY> [-o output.json]
```

| Flag | Default | Description |
|---|---|---|
| `-o` | `ticket_template.json` | Path to write the template file |

**Example:**

```bash
jira-thing template PROJ-42
# Fetching PROJ-42...
# Template saved to ticket_template.json
# {
#   "assignee": { ... },
#   "issuetype": { "name": "Task" },
#   "project": { "key": "PROJ" },
#   ...
# }
```

Save to a specific file:

```bash
jira-thing template PROJ-42 -o templates/bug.json
```

---

### `create` — create a ticket from a template

Loads a template file, prompts for a summary and description, then creates a new Jira ticket with the template's fields pre-filled.

```bash
jira-thing create [-t template.json]
```

| Flag | Default | Description |
|---|---|---|
| `-t` | `ticket_template.json` | Path to the template file |

**Example:**

```bash
jira-thing create -t templates/bug.json
# Enter ticket summary: Fix login redirect on mobile
# Enter ticket description: Users are redirected to /home instead of the original URL after login on iOS Safari.
# Created ticket: PROJ-99
# URL: https://yourorg.atlassian.net/browse/PROJ-99
```

---

### `my-tasks` — list your open tasks

Lists all unresolved Jira issues assigned to you, ordered by most recently updated.

```bash
jira-thing my-tasks [-notupdated]
```

| Flag | Description |
|---|---|
| `-notupdated` | Show only tasks with no activity in the last 3 business days (ordered oldest-first) |

**Example — all open tasks:**

```bash
jira-thing my-tasks
# Found 4 open task(s):
#
#   PROJ-101      In Progress     High      updated: 2026-04-25  Fix login redirect on mobile
#   PROJ-98       To Do           Medium    updated: 2026-04-23  Update API docs
#   PROJ-87       In Review       High      updated: 2026-04-21  Refactor auth middleware
#   PROJ-72       To Do           Low       updated: 2026-04-18  Clean up legacy scripts
```

**Example — stale tasks only:**

```bash
jira-thing my-tasks -notupdated
# Found 2 stale (no updates in 3+ business days) task(s):
#
#   PROJ-87       In Review       High      updated: 2026-04-21  Refactor auth middleware
#   PROJ-72       To Do           Low       updated: 2026-04-18  Clean up legacy scripts
```

---

### `update` — add a comment to a ticket

Adds a comment to an existing Jira ticket. Opens `$EDITOR` to compose the comment, or reads from stdin with `-stdin`.

```bash
jira-thing update <TICKET-KEY> [-stdin]
```

| Flag | Description |
|---|---|
| `-stdin` | Read comment text from stdin instead of opening `$EDITOR` |

**Example — via editor:**

```bash
jira-thing update PROJ-42
# Opens $EDITOR, then posts the saved text as a comment
# Comment added to PROJ-42
# URL: https://yourorg.atlassian.net/browse/PROJ-42
```

**Example — via stdin (useful in scripts):**

```bash
echo "Deployed to staging" | jira-thing update PROJ-42 -stdin
# Comment added to PROJ-42
# URL: https://yourorg.atlassian.net/browse/PROJ-42
```

---

### `clear-auth` — remove stored credentials

Deletes all stored Jira credentials from the OS keychain.

```bash
jira-thing clear-auth
# Credentials cleared.
```

Run this if you change your API token, switch Jira accounts, or want to reset authentication.

---

## Typical workflow

```bash
# 1. Capture a well-configured ticket as a template
jira-thing template PROJ-42 -o templates/task.json

# 2. Create new tickets from that template
jira-thing create -t templates/task.json

# 3. Check what's on your plate
jira-thing my-tasks

# 4. Add a progress update to a ticket
jira-thing update PROJ-42

# 5. Find tickets you haven't touched in a while
jira-thing my-tasks -notupdated
```

## Template file format

Templates are plain JSON. The `template` command captures these fields from an existing ticket:

| Field | Description |
|---|---|
| `project` | The Jira project |
| `issuetype` | Issue type (Task, Bug, Story, etc.) |
| `priority` | Priority level |
| `labels` | Label list |
| `components` | Component list |
| `assignee` | Assignee account |

You can edit the JSON directly to change defaults before running `create`.

**Example `ticket_template.json`:**

```json
{
  "project": { "key": "PROJ" },
  "issuetype": { "name": "Task" },
  "priority": { "name": "Medium" },
  "labels": ["backend"],
  "components": [{ "name": "API" }],
  "assignee": { "accountId": "712020:abc123..." }
}
```

## Development

```bash
make build    # compile binary
make test     # run all tests
make vet      # run go vet
make tidy     # sync go.mod
```

Tests use `httptest` for HTTP mocking and a mock keyring interface — no real Jira connection or OS keychain required.
