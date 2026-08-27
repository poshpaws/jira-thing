# jira-thing

[jirathing.com](https://jirathing.com)

A CLI tool for Jira and Confluence. Clone tickets from reusable templates, manage comments and attachments, track toil, sync pages to Confluence, and upload markdown documents — all from the terminal.

[![Buy Me A Coffee](https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png)](https://buymeacoffee.com/6jnngb4chbz)

## Installation

### Pre-built binaries

Download the latest release for your platform from the [Releases](../../releases) page:

| Platform | File |
|---|---|
| macOS (Apple Silicon) | `jira-thing-darwin-arm64` |
| macOS (Intel) | `jira-thing-darwin-amd64` |
| Linux (x86-64) | `jira-thing-linux-amd64` |
| Linux (ARM64) | `jira-thing-linux-arm64` |
| Windows (x86-64) | `jira-thing-windows-amd64.exe` |
| Windows (ARM64) | `jira-thing-windows-arm64.exe` |

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

**Windows:** rename to `jira-thing.exe` and place on your `PATH`.

### Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/poshpaws/jira-thing.git
cd jira-thing
make build
```

### Optional dependencies

| Tool | Purpose | Install |
|---|---|---|
| `rsvg-convert` | Convert SVG images to PNG for reliable Confluence rendering | See below |

`rsvg-convert` is part of the **librsvg** library. Install it with your package manager:

```bash
# macOS (Homebrew)
brew install librsvg

# Debian / Ubuntu
sudo apt install librsvg2-bin

# Fedora / RHEL
sudo dnf install librsvg2-tools

# Arch
sudo pacman -S librsvg
```

Verify it's available:

```bash
rsvg-convert --version
```

If `rsvg-convert` is not installed, SVG files are uploaded to Confluence as-is. Confluence Cloud has unreliable SVG rendering (garbled images, zero-pixel dimensions), so installing librsvg is recommended if your markdown contains SVG diagrams.

## Authentication

On first use, `jira-thing` prompts for your Jira credentials and stores them securely in the OS keychain (macOS Keychain, Windows Credential Manager, or Linux Secret Service via D-Bus).

> **Linux note:** requires a running Secret Service daemon — GNOME Keyring or KWallet. On headless servers install and unlock `gnome-keyring` or use `secret-tool` to verify D-Bus is available.

```
Jira base URL (e.g. https://yourorg.atlassian.net): https://yourorg.atlassian.net
Jira email: you@example.com
Jira API token: ••••••••••••••••
Credentials stored securely in keyring.
```

**Generating an API token:** Go to [https://id.atlassian.com/manage-profile/security/api-tokens](https://id.atlassian.com/manage-profile/security/api-tokens) and create a token. Use your Atlassian account email as the username.

The same credentials are used for both Jira and Confluence API calls (Atlassian Cloud shares authentication across products).

Credentials are only prompted once. To update or remove them:

```bash
jira-thing clear-auth
```

## Configuration

`jira-thing` reads its configuration from `~/.config/jira-thing/jira-thing.json`. All fields are optional — only the features you use need their corresponding config values.

```json
{
  "project": "PROJ",
  "toil_marker": "TOIL_LABEL",
  "toil_team": "Team-Name",
  "team": "c4cb9231-6ac0-44e7-bd76-64331a96af81",
  "use_team_field": true,
  "editor": "zed -w",
  "confluence_space": "MYSPACE",
  "confluence_url": "https://yourorg.atlassian.net/wiki",
  "confluence_base_page_id": "12345678",
  "ticket_hanger": "Toil Ticket Hangar"
}
```

| Field | Used by | Description |
|---|---|---|
| `project` | `toil-check`, `toil-sync` | Jira project key (e.g. `CRSS`) |
| `toil_marker` | `toil-check`, `toil-sync` | Label that identifies toil tickets |
| `toil_team` | `toil-check`, `toil-sync` | Legacy team label filter (used when `use_team_field` is false) |
| `team` | `toil-check`, `toil-sync` | Jira Team field UUID (used when `use_team_field` is true) |
| `use_team_field` | `toil-check`, `toil-sync` | `true`: filter by Team field. `false` (default): filter by `toil_team` label |
| `editor` | `update` | Preferred editor binary (fallback when `$EDITOR` is unset). Supports arguments, e.g. `"code --wait"` |
| `confluence_space` | `toil-sync`, `conf upload` | Confluence space key (e.g. `ICSCET`) |
| `confluence_url` | `conf upload`, `conf browse` | Confluence base URL (e.g. `https://yourorg.atlassian.net/wiki`) |
| `confluence_base_page_id` | `conf upload`, `conf browse` | Numeric ID of the root page for the space browser TUI |
| `ticket_hanger` | `toil-sync` | Title of the parent Confluence page for toil ticket child pages |

## Commands

### `template` — capture a ticket as a template

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

**Template search path** (when `-t` is not specified): current directory → binary directory → `$XDG_CONFIG_HOME/jira-thing/` → `~/.config/jira-thing/`.

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

**Example:**

```bash
jira-thing mt
# Found 4 open task(s):
#   PROJ-101      In Progress     High      updated: 2026-04-25  Fix login redirect
#   PROJ-98       To Do           Medium    updated: 2026-04-23  Update API docs
```

---

### `update` — add a comment to a ticket

Posts a new comment on an existing Jira ticket. Opens `$EDITOR` to compose the comment, or reads from stdin with `-stdin`.

```bash
jira-thing update <TICKET-KEY> [-stdin]
```

| Flag | Description |
|---|---|
| `-stdin` | Read comment text from stdin instead of opening `$EDITOR` |

Set your preferred editor via the `EDITOR` environment variable. Editors with arguments (e.g. `code --wait`, `nano -w`) are fully supported.

> **GitHub-flavoured markdown is supported.** Comment text is parsed as markdown and converted to Atlassian Document Format (ADF) before posting — headings, bold/italic, bullet and numbered lists, code blocks, blockquotes, tables, strikethrough, and links all render properly in the Jira web UI. Write comments as you would a GitHub PR comment or README.
>
> **Do not use old Jira wiki markup** (`h1.`, `*bold*`, `{code}`, `[link|url]`, etc.) — the parser is GFM-only and does not understand it. Wiki markup will be posted **as literal text**, not rendered. Use standard markdown instead: `#` for headings, `**bold**`, fenced ` ``` ` code blocks, `[text](url)` links.

**Example — markdown formatting:**

```bash
jira-thing update PROJ-42 -stdin << 'EOF'
## Root cause

Race condition in cache invalidation (`internal/cache/store.go:88`).

- Fix deployed to staging
- Monitoring for 24h before prod rollout

| Env | Status |
|---|---|
| Staging | ✅ Deployed |
| Prod | ⏳ Pending |
EOF
```

**Example — editor:**

```bash
jira-thing update PROJ-42
# (editor opens — write your comment, save, exit)
# Comment added to PROJ-42
```

**Example — stdin / pipe:**

```bash
echo "Deployed to staging." | jira-thing update PROJ-42 -stdin
```

---

### `last-comment` — show the last comment on a ticket

Fetches the most recent comment on a Jira ticket and renders it as formatted markdown in the terminal.

```bash
jira-thing last-comment <TICKET-KEY>
```

---

### `describe` — dump full ticket as rendered markdown

Fetches a Jira ticket and renders its key, summary, status, priority, assignee, and description as formatted markdown in the terminal.

```bash
jira-thing describe <TICKET-KEY>
```

**Example:**

```bash
jira-thing de PROJ-42
```

---

### `attach` — attach a file to a ticket

Uploads a local file as an attachment on an existing Jira ticket.

```bash
jira-thing attach <TICKET-KEY> <file-path>
```

**Example:**

```bash
jira-thing at PROJ-42 screenshot.png
# Attached screenshot.png to PROJ-42
# URL: https://yourorg.atlassian.net/browse/PROJ-42
```

---

### `point-check` — check sprint tickets have story points

Finds all tickets assigned to you in the current sprint and reports which ones are missing story points.

```bash
jira-thing point-check
```

---

### `toil-check` — list toil tickets from the last week

Queries Jira for toil tickets updated in the last week, filtered by the project, toil marker, and team from your config.

```bash
jira-thing toil-check
```

**Requires config:** `project`, `toil_marker`, and either `toil_team` or `team` + `use_team_field`.

---

### `toil-sync` — sync toil tickets to Confluence

Queries open toil tickets, presents an interactive TUI for selection, then creates or updates a child Confluence page for each selected ticket under the configured hanger page.

```bash
jira-thing toil-sync
```

**Requires config:** `project`, `toil_marker`, team config, `confluence_space`, `ticket_hanger`.

The TUI lets you select tickets with enter/space, then press `s` to sync. Each ticket gets a child page with an embedded Jira macro and a notes section. Notes are preserved across syncs.

---

### `conf browse` — browse Confluence space tree

Launches an interactive TUI that traverses the Confluence page hierarchy starting from the configured base page. Useful for finding page IDs or exploring the space structure.

```bash
jira-thing conf browse
```

**Requires config:** `confluence_base_page_id`, and optionally `confluence_url` and `confluence_space` for the URL output.

**TUI controls:**

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate the page list |
| `enter` / `→` | Drill into a child page |
| `backspace` / `←` | Go back to the parent page |
| `s` | Select the current page and exit |
| `q` / `esc` | Cancel and exit |

A breadcrumb trail at the top shows your current position in the hierarchy. On exit, the selected page's title, ID, and URL are printed.

---

### `conf upload` — upload markdown to Confluence

Converts a local markdown file to Confluence storage format, launches the space browser TUI to choose a parent page, creates the page, and uploads any referenced local files as attachments.

```bash
jira-thing conf upload <file.md> [-title "Page Title"]
```

| Flag | Default | Description |
|---|---|---|
| `-title` | Filename without extension | Title for the Confluence page |

**Requires config:** `confluence_space`, `confluence_url`, `confluence_base_page_id`.

**What gets converted:**

| Markdown | Confluence storage format |
|---|---|
| Headings (`#`, `##`, etc.) | `<h1>`, `<h2>`, etc. |
| Bold, italic, strikethrough | `<strong>`, `<em>`, `<s>` |
| Links | `<a href>` for external URLs; `<ac:link>` attachment macro for local files |
| Images | `<ac:image>` with `<ri:attachment>` for local files; `<ri:url>` for external URLs |
| Code blocks | `<ac:structured-macro ac:name="code">` with language parameter |
| Tables | `<table>` with `<th>` / `<td>` |
| Lists, blockquotes, horizontal rules | Standard XHTML equivalents |

**What gets uploaded as attachments:**

Local files referenced via image syntax (`![alt](path)`) or link syntax (`[text](path)`) with recognised extensions are automatically uploaded:

- **Images:** `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.svg`, `.bmp`, `.ico`
- **Documents:** `.pdf`, `.doc`, `.docx`, `.xls`, `.xlsx`, `.ppt`, `.pptx`, `.csv`
- **Other:** `.zip`, `.gz`, `.tar`, `.json`, `.yaml`, `.yml`, `.xml`, `.txt`, `.log`, `.drawio`

**SVG handling:** If `rsvg-convert` is installed (part of librsvg — `brew install librsvg`), SVG files are automatically converted to PNG before upload for reliable rendering in Confluence. The page XHTML is rewritten to reference the PNG filename. If `rsvg-convert` is not available, SVGs are uploaded as-is with a warning. See [Optional dependencies](#optional-dependencies) for install instructions on other platforms.

**Example:**

```bash
jira-thing conf upload docs/architecture.md -title "Architecture Overview"
# Converted docs/architecture.md → 2847 bytes of storage XHTML, 3 attachment(s) found
# Select a parent page for the upload:
# (space browser TUI opens — navigate and press s to select)
# Creating page "Architecture Overview" under "Engineering Docs"...
# Created page: Architecture Overview (ID: 12345678)
#   Attached: system-overview.png (ID: att456)
#   Attached: data-flow.png (ID: att457)
#   Attached: report.pdf (ID: att458)
# Done: https://yourorg.atlassian.net/wiki/spaces/MYSPACE/pages/12345678
```

---

### `diagnose` — test API connectivity

Tests your Jira API connection and credentials. Also provides sub-commands for field discovery.

```bash
jira-thing diagnose                      # Test connectivity
jira-thing diagnose -find-field <search> # Find a field's customfield ID by name
jira-thing diagnose -list-fields         # List all fields on the instance
jira-thing diagnose -team <TICKET-KEY>   # Print a ticket's Team field value and ID
```

---

### `subtask` — create subtasks from a markdown task list

Reads a markdown file, extracts all list items (bullet, numbered, or checkbox), and creates a Jira subtask for each one under the specified parent ticket. Each subtask inherits the parent's project, priority, labels, and components.

```bash
jira-thing subtask <PARENT-KEY> -f tasks.md [--dry-run]
```

| Flag | Description |
|---|---|
| `-f` | Path to markdown file containing the task list (required) |
| `--dry-run` | Show the parsed tasks without creating anything |

**Supported markdown formats:**

```markdown
- Bullet list item
1. Numbered list item
- [ ] Unchecked checkbox
- [x] Checked checkbox (still created — useful for re-importing)
```

Nested items are flattened into the top-level list. Only list items are extracted — headings, paragraphs, and other content are ignored. This makes it safe to point at a full ShapeUp brief or design doc and have it pull just the task list.

**Example — dry run first:**

```bash
jira-thing st PROJ-42 -f docs/shapeup.md --dry-run
# Parent: PROJ-42 (8 subtask(s) from docs/shapeup.md)
#
#    1. Set up Terraform module for new Lambda
#    2. Implement DynamoDB schema
#    3. Build EventBridge rule for scheduling
#    ...
# Dry run — no subtasks created.
```

**Example — create for real:**

```bash
jira-thing st PROJ-42 -f docs/shapeup.md
# Fetching parent ticket PROJ-42...
# Parent: PROJ-42 (8 subtask(s) from docs/shapeup.md)
#
#    1. Set up Terraform module for new Lambda
#    2. Implement DynamoDB schema
#    ...
# Creating 8 subtask(s)...
#   Created: PROJ-43  Set up Terraform module for new Lambda
#   Created: PROJ-44  Implement DynamoDB schema
#   ...
# Done: 8 of 8 subtask(s) created under PROJ-42
```

---

### `check-update` — check for newer releases

Checks GitHub for a newer release of `jira-thing`.

```bash
jira-thing check-update
```

---

### `clear-auth` — remove stored credentials

Deletes all stored Jira credentials from the OS keychain.

```bash
jira-thing clear-auth
```

---

## Command aliases

Most commands have short aliases for quick typing:

| Command | Alias |
|---|---|
| `template` | `te` |
| `create` | `cr` |
| `my-tasks` | `mt` |
| `update` | `up` |
| `last-comment` | `lc` |
| `attach` | `at` |
| `describe` | `de` |
| `toil-check` | `tc`, `toil` |
| `toil-sync` | `ts` |
| `point-check` | `pc` |
| `conf browse` | `conf br` |
| `conf upload` | `conf up` |
| `subtask` | `st` |
| `diagnose` | `diag` |
| `check-update` | `cu` |

## Typical workflow

```bash
# 1. Capture a well-configured ticket as a template
jira-thing te PROJ-42 -o templates/task.json

# 2. Create new tickets from that template
jira-thing cr -t templates/task.json

# 3. Check what's on your plate
jira-thing mt

# 4. Add a progress update to a ticket
jira-thing up PROJ-42

# 5. Find tickets you haven't touched in a while
jira-thing mt -notupdated

# 6. Upload a design doc to Confluence
jira-thing conf up docs/design.md -title "Q3 Design Doc"

# 7. Browse the Confluence space to find a page ID
jira-thing conf br

# 8. Create subtasks from a ShapeUp brief's task list
jira-thing st PROJ-42 -f docs/shapeup.md
```

## Agent Skill

SKILL.md provided but not had a lot of testing — please feedback via issues.

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
