# jira-thing

A CLI tool for cloning Jira tickets via reusable JSON templates. Capture an existing ticket's structure once, then stamp out new tickets from it interactively.

## Installation

### Pre-built binaries

Download the latest release for your platform from the [Releases](../../releases) page:

| Platform | File |
|---|---|
| macOS (Apple Silicon) | `jira-thing-darwin-arm64` |
| macOS (Intel) | `jira-thing-darwin-amd64` |
| Linux (x86-64) | `jira-thing-linux-amd64` |
| Linux (ARM64) | `jira-thing-linux-arm64` |
| Windows | `jira-thing-windows-amd64.exe` |

**macOS:**

```bash
chmod +x jira-thing-darwin-arm64
sudo mv jira-thing-darwin-arm64 /usr/local/bin/jira-thing
```

**Linux:**

```bash
chmod +x jira-thing-linux-amd64
sudo mv jira-thing-linux-amd64 /usr/local/bin/jira-thing
```

**Linux (ARM64 — Raspberry Pi, AWS Graviton, etc.):**

```bash
chmod +x jira-thing-linux-arm64
sudo mv jira-thing-linux-arm64 /usr/local/bin/jira-thing
```

**Windows:** rename to `jira-thing.exe` and place on your `PATH`.

### Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/poshpaws/jira-thing.git
cd jira-thing
make build
```

## Authentication

On first use, `jira-thing` will prompt for your Jira credentials and store them securely in the OS keychain (macOS Keychain, Windows Credential Manager, or Linux Secret Service via D-Bus).

> **Linux note:** requires a running Secret Service daemon — GNOME Keyring or KWallet. On headless servers install and unlock `gnome-keyring` or use `secret-tool` to verify D-Bus is available.

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

Fetches an existing Jira issue and saves its reusable fields (project, issue type, priority, labels, components, assignee) as a local JSON file.

```bash
jira-thing template <TICKET-KEY> [-o output.json]
```
Note replace <TICKET-KEY> with the actual key of the ticket you want to capture, e.g. `PROJ-42`.

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

Posts a new comment on an existing Jira ticket. The existing description is **not** modified. Opens `$EDITOR` to compose the comment, or reads from stdin with `-stdin`.

```bash
jira-thing update <TICKET-KEY> [-stdin]
```

| Flag | Description |
|---|---|
| `-stdin` | Read comment text from stdin instead of opening `$EDITOR` |

Set your preferred editor via the `EDITOR` environment variable. Editors with arguments (e.g. `code --wait`, `nano -w`) are fully supported.

**Example — editor:**

```bash
export EDITOR="nano -w"
jira-thing update PROJ-42
# (nano opens — write your comment, save, exit)
# Updated PROJ-42
# URL: https://yourorg.atlassian.net/browse/PROJ-42
```

**Example — stdin:**

```bash
echo "Deployed to staging. Monitoring for 30 min." | jira-thing update PROJ-42 -stdin
```

**Example — heredoc (multi-line):**

```bash
jira-thing update PROJ-42 -stdin << 'EOF'
Root cause: race condition in cache invalidation.
Fix deployed to staging. Monitoring for 24h before prod rollout.
EOF
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
