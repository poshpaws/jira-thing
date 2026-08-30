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
| `project` | `toil-check`, `toil-sync` | Jira project key (e.g. `PROJ`) |
| `toil_marker` | `toil-check`, `toil-sync` | Label that identifies toil tickets |
| `toil_team` | `toil-check`, `toil-sync` | Legacy team label filter (used when `use_team_field` is false) |
| `team` | `toil-check`, `toil-sync` | Jira Team field UUID (used when `use_team_field` is true) |
| `use_team_field` | `toil-check`, `toil-sync` | `true`: filter by Team field. `false` (default): filter by `toil_team` label |
| `editor` | `update` | Preferred editor binary (fallback when `$EDITOR` is unset). Supports arguments, e.g. `"code --wait"` |
| `confluence_space` | `toil-sync`, `conf upload` | Confluence space key (e.g. `ENG`) |
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

### `state` — move a ticket to a new workflow state

Opens a TUI to move a ticket through its Jira workflow. Available states are **always read live from the ticket** via the Jira transitions API — workflows are heavily customised per board (a personal board might have 4 states, a client board 12), so nothing is hardcoded.

```bash
jira-thing state [TICKET-KEY]
```

If `TICKET-KEY` is omitted, a TUI first lists your open tasks (same list as `my-tasks`) so you can pick one. It then fetches that ticket's available transitions and shows a second TUI to pick the target state.

| Keys | Action |
|---|---|
| `↑`/`↓` | Navigate |
| `enter` | Select |
| `q` / `esc` / `ctrl+c` | Cancel |

**Example:**

```bash
jira-thing state PROJ-101
#   PROJ-101 — choose new state
#   Move to
#   In Review
#   Done
#   Blocked
# (enter on "Done")
# PROJ-101 moved to Done
```

If a ticket has no available transitions (e.g. already in a terminal state with no outgoing edges), the command reports that and exits without opening a TUI.

---

### `menu` — interactive menu for everything else

Opens a persistent home-screen TUI covering the full set of ticket operations jira-thing supports: search, describe, edit fields, comment, worklog, attach, subtasks, link/unlink, clone, delete, boards/sprints, projects/releases, and whoami. This is the jira-cli-parity surface — one place to reach every operation without memorising a flag for each.

```bash
jira-thing menu
```

Pick an action from the list; you'll be prompted for whatever it needs (ticket key, JQL, etc.) directly in the terminal. After an action finishes, press enter to return to the menu. Unlike the one-shot commands, a failed action (bad key, invalid JQL) reports the error and drops you back at the menu instead of exiting the program.

| Keys | Action |
|---|---|
| `↑`/`↓` | Navigate |
| `enter` | Select |
| `q` / `esc` / `ctrl+c` | Quit the menu |

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

Converts a local markdown file to Confluence storage format, launches the space browser TUI to choose a parent page, creates (or updates) the page, and uploads any referenced local files as attachments.

```bash
jira-thing conf upload <file.md> [-title "Page Title"]
```

| Flag | Default | Description |
|---|---|---|
| `-title` | Filename without extension | Title for the Confluence page |

**Requires config:** `confluence_space`, `confluence_url`, `confluence_base_page_id`.

#### Space browser TUI

After converting the markdown, the space browser launches so you can choose where to put the page. It starts at the page specified by `confluence_base_page_id` in your config.

| Key | Action |
|---|---|
| `↑` / `↓` | Navigate the page list |
| `enter` / `→` | Drill into a child page |
| `backspace` / `←` | Go back to the parent |
| `s` | Select the current page as the parent and proceed |
| `q` / `esc` | Cancel the upload |

A breadcrumb trail at the top shows your position in the hierarchy.

#### Create vs update behaviour

If a page with the same title already exists under the parent you selected, the page body is **updated in place** (version bumped) rather than creating a duplicate. This is deliberate — Confluence does not allow page deletion, only archiving, so update-in-place is the correct workflow for republishing changed documents.

Attachments follow the same pattern: existing attachments with the same filename are replaced with the new version; new filenames are added.

#### What gets converted

| Markdown | Confluence storage format |
|---|---|
| Headings (`#`, `##`, etc.) | `<h1>`, `<h2>`, etc. |
| Bold, italic, strikethrough | `<strong>`, `<em>`, `<s>` |
| Inline code | `<code>` |
| Links to external URLs | `<a href="...">` |
| Links to local files | `<ac:link>` attachment macro (file uploaded automatically) |
| Images (external URL) | `<ac:image>` with `<ri:url>` |
| Images (local file) | `<ac:image>` with `<ri:attachment>` (file uploaded automatically) |
| Fenced code blocks | `<ac:structured-macro ac:name="code">` with language parameter |
| Tables | `<table>` with `<th>` / `<td>` |
| Lists, blockquotes, horizontal rules | Standard XHTML equivalents |

#### What gets uploaded as attachments

Local files referenced via image syntax (`![alt](path)`) or link syntax (`[text](path)`) with recognised extensions are automatically uploaded as Confluence page attachments:

- **Images:** `.png`, `.jpg`, `.jpeg`, `.gif`, `.webp`, `.svg`, `.bmp`, `.ico`
- **Documents:** `.pdf`, `.doc`, `.docx`, `.xls`, `.xlsx`, `.ppt`, `.pptx`, `.csv`
- **Other:** `.zip`, `.gz`, `.tar`, `.json`, `.yaml`, `.yml`, `.xml`, `.txt`, `.log`, `.drawio`

Duplicate filenames are deduplicated — if the same image is referenced in multiple places, it is uploaded once.

#### Diagram handling

**Draw.io files:** If a `.drawio` file is referenced directly, or if a `.drawio` sibling file exists alongside a referenced `.svg` or `.png` (e.g. `flow.svg` with `flow.drawio` in the same directory), the `.drawio` file is uploaded and embedded using Confluence's native draw.io macro. This gives full-fidelity rendering with editable diagrams.

**SVG conversion:** For SVG files without a `.drawio` sibling, if `rsvg-convert` is installed (part of librsvg), SVGs are automatically converted to PNG before upload. Confluence Cloud has unreliable SVG rendering, so PNG conversion is recommended. The page XHTML is rewritten to reference the PNG filename. If `rsvg-convert` is not available, SVGs are uploaded as-is with a warning.

Install `rsvg-convert`:
```bash
brew install librsvg          # macOS
sudo apt install librsvg2-bin # Debian/Ubuntu
```

See [Optional dependencies](#optional-dependencies) for all platforms.

**Priority order for image references:**
1. `.drawio` sibling exists → upload `.drawio`, embed with draw.io macro
2. `.svg` with no `.drawio` sibling → convert to PNG via `rsvg-convert` (if available)
3. `.png`, `.jpg`, etc. → upload and embed as `<ac:image>` directly

#### Attachment upload resilience

Confluence Cloud occasionally returns HTTP 500 errors when uploading attachments immediately after page creation (transaction rollback race condition). `conf upload` handles this with:

- A 2-second settle delay between page creation and attachment uploads
- Up to 3 retries with 3-second backoff on HTTP 5xx errors
- Non-5xx errors (400, 403, etc.) fail immediately without retry

#### Path safety

Local file paths referenced in markdown are validated against the markdown file's directory. Paths that traverse outside the base directory (e.g. `../../../etc/passwd`) are rejected with a warning. Absolute paths are allowed but logged.

#### Example — first upload

```bash
jira-thing conf upload docs/architecture.md -title "Architecture Overview"
# Converted docs/architecture.md → 2847 bytes of storage XHTML, 3 attachment(s) found
# Select a parent page for the upload:
#   ECP Service Documentation › Vulnerability Management Service
# (navigate with arrows, press s to select)
# Creating page "Architecture Overview" under "Vulnerability Management Service"...
# Created page: Architecture Overview (ID: 12345678)
# Uploading 3 attachment(s)...
#   Attached: system-overview.png (ID: att456)
#   Attached: data-flow.drawio (ID: att457)
#   Attached: report.pdf (ID: att458)
# Done: https://yourorg.atlassian.net/wiki/spaces/ENG/pages/12345678
```

#### Example — re-upload (update in place)

```bash
jira-thing conf up docs/architecture.md -title "Architecture Overview"
# Converted docs/architecture.md → 3102 bytes of storage XHTML, 3 attachment(s) found
# Select a parent page for the upload:
# Page "Architecture Overview" already exists under "Vulnerability Management Service" — updating (version 1 → 2)...
# Updated page: Architecture Overview (ID: 12345678)
# Uploading 3 attachment(s)...
#   Updated: system-overview.png (ID: att456)
#   Updated: data-flow.drawio (ID: att457)
#   Updated: report.pdf (ID: att458)
# Done: https://yourorg.atlassian.net/wiki/spaces/ENG/pages/12345678
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

Reads a markdown file, extracts tasks, and creates a Jira subtask for each one under the specified parent ticket. Each subtask inherits the parent's project, priority, labels, and components. In heading mode, the full body text between headings is captured as the subtask description (converted to ADF for rich rendering in Jira).

```bash
jira-thing subtask <PARENT-KEY> -f <file.md> [--heading regex] [--section name] [--dry-run]
```

| Flag | Description |
|---|---|
| `-f` | Path to markdown file containing the tasks (required) |
| `--heading` | Extract tasks from headings matching this regex (see heading mode below) |
| `--section` | Extract list items only from the named section heading (see section mode below) |
| `--dry-run` | Show the parsed tasks without creating anything |

#### Three extraction modes

The parser has three modes to handle different document structures:

**Default mode** — extracts every list item in the file (bullet, numbered, checkbox). Best for simple task lists or ShapeUp briefs where the tasks are the only list content.

```bash
jira-thing st PROJ-42 -f tasks.md
```

**Heading mode** (`--heading`) — extracts tasks from headings matching a regex pattern. The matched prefix is stripped from the summary, and the full markdown body between headings becomes the subtask description (converted to ADF). Best for implementation plans where each task is its own section with detailed guidance.

```bash
jira-thing st PROJ-42 -f docs/implementation-plan.md --heading "^Task \d+:"
```

This finds headings like `### Task 1: Cross-Account IAM Role Module` and creates a subtask with:
- **Summary:** `Cross-Account IAM Role Module` (prefix stripped)
- **Description:** Everything between that heading and the next `### Task N:` heading — objective, implementation guidance, test requirements, demo criteria — all rendered as rich ADF in Jira.

**Section mode** (`--section`) — extracts list items only from under a specific heading, stopping at the next heading of equal or higher level. Best for documents where tasks live in a named section alongside other content.

```bash
jira-thing st PROJ-42 -f docs/shapeup.md --section "Initial Task List"
```

#### Inherited fields

Each subtask automatically inherits from the parent ticket:
- **Project** — same Jira project
- **Priority** — same priority level
- **Labels** — same labels
- **Components** — same components

The issue type is set to `Sub-task` and the parent link is set automatically.

#### Supported markdown formats

All three modes handle standard markdown list syntax:

```markdown
- Bullet list item
1. Numbered list item
- [ ] Unchecked checkbox
- [x] Checked checkbox (still created — useful for re-importing)
  - Nested items are flattened into the top-level list
```

In default and section modes, nested items are flattened (Jira subtasks cannot have sub-subtasks). Headings, paragraphs, code blocks, and other non-list content are ignored.

#### Example — heading mode with dry run

```bash
jira-thing st PROJ-123 -f docs/implementation-plan.md --heading "^Task \d+:" --dry-run
# Fetching parent ticket PROJ-123...
#
# Parent: PROJ-123 (21 subtask(s) from docs/implementation-plan.md)
#
#    1. Cross-Account IAM Role Module (AFT — IAG + BA)
#    2. Spike — SSM Custom Inventory PutInventory (Foundation)
#    3. Spike — SSM Distributor Packages (Foundation)
#    4. Project Scaffolding and pyproject.toml Setup
#    5. Account Discovery Lambda
#    ...
#   21. Migrate Scan Baselines to CIS Level 1 + RHEL 10 and Windows Server 2025 Support
#
# Dry run — no subtasks created.
```

#### Example — default mode (simple task list)

```bash
jira-thing st PROJ-42 -f tasks.md
# Fetching parent ticket PROJ-42...
#
# Parent: PROJ-42 (8 subtask(s) from tasks.md)
#
#    1. Set up Terraform module for new Lambda
#    2. Implement DynamoDB schema
#    3. Build EventBridge rule for scheduling
#    ...
#
# Creating 8 subtask(s)...
#   Created: PROJ-43  Set up Terraform module for new Lambda
#   Created: PROJ-44  Implement DynamoDB schema
#   ...
# Done: 8 of 8 subtask(s) created under PROJ-42
```

#### Example — section mode (ShapeUp brief)

```bash
jira-thing st PROJ-42 -f docs/shapeup.md --section "Initial Task List" --dry-run
# Fetching parent ticket PROJ-42...
#
# Parent: PROJ-42 (6 subtask(s) from docs/shapeup.md)
#
#    1. Create API endpoint for user preferences
#    2. Build React settings panel
#    ...
#
# Dry run — no subtasks created.
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

### `serve-mcp` — MCP server for AI agent integration

Starts a Model Context Protocol (MCP) server on stdio, exposing jira-thing's Jira and Confluence operations as tools for AI agents (Kiro, Claude, Copilot, etc.).

```bash
jira-thing serve-mcp
```

This is not run directly — it's launched by the AI agent via the MCP configuration. The server uses the same credentials from your OS keychain as the CLI.

**Available MCP tools:**

| Tool | Description |
|---|---|
| `describe_ticket` | Fetch a ticket's full details (summary, status, priority, assignee, description) |
| `search_tickets` | Search tickets using JQL |
| `my_tasks` | List open tasks assigned to you |
| `last_comment` | Fetch the most recent comment on a ticket |
| `add_comment` | Add a markdown comment to a ticket |
| `create_ticket` | Create a new ticket with project, summary, description, priority, labels |
| `update_ticket` | Edit summary, description, priority, labels, or assignee on an existing ticket |
| `list_transitions` | List the workflow states a ticket can currently move to |
| `transition_ticket` | Move a ticket to a new workflow state by name, optionally setting a comment, resolution, or assignee at the same time |
| `list_fields` | List every field on the instance, including custom field IDs |
| `add_attachment` | Upload a local file as an attachment on a ticket |
| `create_subtask` | Create a subtask under a ticket, inheriting project/priority/labels/components |
| `list_link_types` | List the issue link types configured on the instance |
| `link_tickets` | Link two tickets together (e.g. Blocks, Relates) |
| `unlink_tickets` | Remove the link between two tickets |
| `add_remote_link` | Attach a remote web link to a ticket |
| `whoami` | Show the current authenticated Jira user |
| `delete_ticket` | Permanently delete a ticket, optionally cascading to subtasks |
| `clone_ticket` | Clone a ticket, optionally overriding summary/priority/assignee |
| `add_worklog` | Log time spent against a ticket |
| `list_boards` | List Agile boards, optionally filtered to a project |
| `list_sprints` | List the sprints on a board |
| `list_sprint_issues` | List the issues in a sprint |
| `add_to_sprint` | Move tickets into a sprint |
| `list_projects` | List every accessible project |
| `list_versions` | List a project's releases/versions |
| `confluence_browse` | List child pages under a Confluence page |
| `confluence_get_page` | Fetch a page by ID or by space + title |
| `confluence_create_page` | Create or update a Confluence page from markdown |
| `confluence_update_page` | Update an existing page's content from markdown |

**Kiro MCP configuration** (`~/.kiro/settings/mcp.json` or `.kiro/settings/mcp.json`):

```json
{
  "mcpServers": {
    "jira-thing": {
      "command": "jira-thing",
      "args": ["serve-mcp"],
      "disabled": false
    }
  }
}
```

If `jira-thing` is not on your PATH, use the full path to the binary:

```json
{
  "mcpServers": {
    "jira-thing": {
      "command": "/usr/local/bin/jira-thing",
      "args": ["serve-mcp"],
      "disabled": false
    }
  }
}
```

---

## Command aliases

Most commands have short aliases for quick typing:

| Command | Alias |
|---|---|
| `template` | `te` |
| `create` | `cr` |
| `my-tasks` | `mt` |
| `state` | `sta` |
| `menu` | `m` |
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
