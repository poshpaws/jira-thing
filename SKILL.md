---
name: jira-thing
description: CLI tool for Jira and Confluence — tickets, comments, attachments, subtasks, Confluence publishing, and space browsing. Also available as an MCP server.
---

# jira-thing

Go CLI tool for Jira and Confluence. Credentials stored in OS keyring (same creds for both Jira and Confluence).

> **Why this file exists:** Some corporate environments disable local MCP servers by policy. When that happens, this SKILL.md gives AI agents (Kiro, Codex, etc.) full knowledge of jira-thing's capabilities so they can shell out to the CLI directly instead of calling MCP tools. The effect is the same — the agent can read tickets, add comments, create subtasks, and publish to Confluence — it just uses `jira-thing <command>` instead of an MCP tool call.

## When to use

- Fetch, search, create, or describe Jira tickets
- Add markdown comments to tickets (converted to ADF automatically)
- Show the last comment on a ticket
- List open tasks or find stale ones
- Attach files to tickets
- Create subtasks from a markdown task list or implementation plan
- Upload markdown documents to Confluence (with images, diagrams, and attachments)
- Browse the Confluence page hierarchy
- Sync TOIL tickets to Confluence
- Check sprint tickets for missing story points
- Diagnose API connectivity and credential problems

## Commands

### `template <TICKET-KEY> [-o output.json]` (alias: `te`)

Fetch an existing ticket and save its reusable fields as a local JSON template.

```bash
jira-thing template PROJ-123
jira-thing template PROJ-123 -o my-template.json
```

---

### `create [-t template.json]` (alias: `cr`)

Create a new Jira ticket from a template. Prompts for summary and description.

```bash
jira-thing create
jira-thing create -t /path/to/template.json
```

Template search path: current directory → binary directory → `$XDG_CONFIG_HOME/jira-thing/` → `~/.config/jira-thing/`.

---

### `update <TICKET-KEY> [-stdin]` (alias: `up`)

Add a markdown comment to a ticket. Opens `$EDITOR` by default; use `-stdin` for piped input.

```bash
jira-thing update PROJ-123
echo "Deployed to staging." | jira-thing update PROJ-123 -stdin
```

GFM markdown is converted to ADF — headings, lists, code blocks, tables, strikethrough, and links all render in Jira.

---

### `last-comment <TICKET-KEY>` (alias: `lc`)

Fetch and render the most recent comment as terminal markdown.

```bash
jira-thing last-comment PROJ-123
```

---

### `describe <TICKET-KEY>` (alias: `de`)

Dump a ticket's full details (key, summary, status, priority, assignee, description) as rendered markdown.

```bash
jira-thing describe PROJ-123
```

---

### `my-tasks [-notupdated]` (alias: `mt`)

List open tasks assigned to you. `-notupdated` shows only tasks idle for 3+ business days.

```bash
jira-thing my-tasks
jira-thing my-tasks -notupdated
```

---

### `attach <TICKET-KEY> <file>` (alias: `at`)

Upload a local file as an attachment on a ticket.

```bash
jira-thing attach PROJ-123 screenshot.png
```

---

### `point-check` (alias: `pc`)

Check current sprint tickets for missing story points.

```bash
jira-thing point-check
```

---

### `toil-check` (aliases: `toil`, `tc`)

List toil tickets from the last week, filtered by project, toil marker, and team from config.

```bash
jira-thing toil-check
```

---

### `toil-sync` (alias: `ts`)

Sync TOIL tickets to Confluence child pages under the configured hanger page. Interactive TUI for ticket selection.

```bash
jira-thing toil-sync
```

---

### `conf browse` (alias: `conf br`)

Browse the Confluence page hierarchy starting from the configured base page. Useful for finding page IDs.

```bash
jira-thing conf browse
```

TUI controls: `↑/↓` navigate, `enter/→` drill in, `backspace/←` go up, `s` select, `q` quit.

---

### `conf upload <file.md> [-title "Title"]` (alias: `conf up`)

Convert a markdown file to Confluence storage format and publish it. Launches a space browser TUI to choose the parent page.

```bash
jira-thing conf upload docs/architecture.md -title "Architecture Overview"
```

Key behaviours:
- **Create or update:** If a page with the same title already exists under the chosen parent, it is updated in place (version bumped). No duplicates.
- **Attachments:** Local files referenced via `![](path)` or `[text](path)` are uploaded automatically. Duplicate filenames are deduplicated.
- **Draw.io support:** `.drawio` files (or `.svg`/`.png` with a `.drawio` sibling) are embedded with Confluence's native draw.io macro.
- **SVG handling:** SVGs without a `.drawio` sibling are converted to PNG via `rsvg-convert` if available.
- **Retry resilience:** Attachment uploads retry up to 3 times on HTTP 5xx errors.

Requires config: `confluence_space`, `confluence_url`, `confluence_base_page_id`.

---

### `subtask <PARENT-KEY> -f <file.md> [--heading regex] [--section name] [--dry-run]` (alias: `st`)

Create Jira subtasks from a markdown file. Each subtask inherits the parent's project, priority, labels, and components.

Three extraction modes:

**Default** — extracts all list items (bullet, numbered, checkbox):
```bash
jira-thing st PROJ-42 -f tasks.md
```

**Heading mode** — extracts tasks from headings matching a regex. The body between headings becomes the subtask description (converted to ADF):
```bash
jira-thing st PROJ-42 -f docs/implementation-plan.md --heading "^Task \d+:"
```

**Section mode** — extracts list items only from a named section:
```bash
jira-thing st PROJ-42 -f docs/shapeup.md --section "Initial Task List"
```

Always dry-run first:
```bash
jira-thing st PROJ-42 -f plan.md --heading "^Task \d+:" --dry-run
```

---

### `serve-mcp`

Start an MCP (Model Context Protocol) server on stdio for AI agent integration. Exposes all Jira and Confluence operations as MCP tools.

```bash
jira-thing serve-mcp
```

Not run directly — configured in your AI agent's MCP settings. Tools available: `describe_ticket`, `search_tickets`, `my_tasks`, `last_comment`, `add_comment`, `create_ticket`, `confluence_browse`, `confluence_get_page`, `confluence_create_page`, `confluence_update_page`.

---

### `diagnose` (alias: `diag`)

Test API connectivity. Also supports field lookups:

```bash
jira-thing diagnose
jira-thing diagnose -find-field team
jira-thing diagnose -list-fields
jira-thing diagnose -team PROJ-123
```

---

### `self-update` (alias: `su`)

Download and install the latest release from GitHub.

```bash
jira-thing self-update
```

---

### `check-update` (alias: `cu`)

Check GitHub for newer releases without installing.

```bash
jira-thing check-update
```

---

### `clear-auth`

Remove stored credentials from the OS keyring.

```bash
jira-thing clear-auth
```

---

## Configuration

Settings in `~/.config/jira-thing/jira-thing.json`:

```json
{
  "project": "PROJ",
  "toil_marker": "TOIL",
  "toil_team": "My Team",
  "team": "c4cb9231-6ac0-44e7-bd76-64331a96af81",
  "use_team_field": true,
  "editor": "zed -w",
  "confluence_space": "ENG",
  "confluence_url": "https://yourorg.atlassian.net/wiki",
  "confluence_base_page_id": "12345678",
  "ticket_hanger": "Toil Ticket Hangar"
}
```

| Field | Used by | Description |
|---|---|---|
| `project` | toil-check, toil-sync | Jira project key |
| `toil_marker` | toil-check, toil-sync | Label that marks toil tickets |
| `toil_team` | toil-check, toil-sync | Legacy team label filter (when `use_team_field` is false) |
| `team` | toil-check, toil-sync | Jira Team field UUID (when `use_team_field` is true) |
| `use_team_field` | toil-check, toil-sync | `true`: match Team field. `false` (default): match `toil_team` label |
| `editor` | update | Fallback editor when `$EDITOR` is unset |
| `confluence_space` | toil-sync, conf upload | Confluence space key |
| `confluence_url` | conf upload, conf browse | Confluence base URL (e.g. `https://yourorg.atlassian.net/wiki`) |
| `confluence_base_page_id` | conf upload, conf browse | Root page ID for the space browser TUI |
| `ticket_hanger` | toil-sync | Parent page title for toil ticket child pages |

### Finding the team UUID

```bash
jira-thing diagnose -find-field team         # find the customfield ID
jira-thing diagnose -team PROJ-123           # get the team UUID from a real ticket
```

Put the UUID in `team`, set `use_team_field: true`.

## Typical workflow

```bash
jira-thing te PROJ-42 -o templates/task.json   # capture a template
jira-thing cr -t templates/task.json           # create a ticket
jira-thing mt                                  # check your tasks
jira-thing up PROJ-42                          # add a progress comment
jira-thing mt -notupdated                      # find stale tasks
jira-thing conf up docs/design.md              # publish to Confluence
jira-thing conf br                             # browse Confluence pages
jira-thing st PROJ-42 -f plan.md --heading "^Task \d+:" --dry-run  # preview subtasks
jira-thing st PROJ-42 -f plan.md --heading "^Task \d+:"            # create them
```
