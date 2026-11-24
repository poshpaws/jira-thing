# jira-thing

[jirathing.com](https://jirathing.com)

A CLI tool for Jira and Confluence. Clone tickets from reusable templates, manage comments and attachments, track toil, sync pages to Confluence, and upload markdown documents — all from the terminal.

[![Buy Me A Coffee](https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png)](https://buymeacoffee.com/6jnngb4chbz)

## Installation

### Homebrew (macOS and Linux)

```bash
brew install poshpaws/tap/jira-thing
brew upgrade jira-thing        # later releases
```

The formula installs the same signed, notarised release binaries as below. `jira-thing self-update` refuses to replace a Homebrew-managed binary and points you at `brew upgrade` instead.

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

## Profiles

A profile is a separate set of stored credentials, plus optional config overrides, for working with more than one Jira site or account.

```bash
jira-thing -profile work me                # prompts for the work site's credentials the first time
jira-thing -profile work list -a me        # -profile goes before the command
export JIRA_THING_PROFILE=work             # or select it for the whole shell
jira-thing -profile work serve-mcp         # an MCP server for the work site
jira-thing -profile work clear-auth        # clears only that profile's credentials
```

- With no profile, jira-thing uses the credentials it always has; nothing changes for existing setups.
- A profile's credentials are stored in the same keychain entry under keys ending `@NAME`. Names are 1–32 letters, digits, `-` or `_`.
- `diagnose` shows the active profile and where its credentials came from.

**CI and other places without a keychain:** set `JIRA_URL`, `JIRA_EMAIL` and `JIRA_API_TOKEN`. When all three are set they're used instead of the keychain, whatever the profile. Setting only some of them is an error, so a stored URL is never paired with another account's token.

## Configuration

`jira-thing` reads its configuration from `~/.config/jira-thing/jira-thing.json`. All fields are optional — only the features you use need their corresponding config values.

```json
{
  "project": "PROJ",
  "board": 42,
  "story_points_field": "customfield_10016",
  "default_priority": "Medium",
  "default_components": ["Backend"],
  "default_labels": ["triage"],
  "templates_dir": "~/work/jira-templates",
  "toil_marker": "TOIL_LABEL",
  "toil_team": "Team-Name",
  "team": "c4cb9231-6ac0-44e7-bd76-64331a96af81",
  "use_team_field": true,
  "editor": "zed -w",
  "confluence_space": "MYSPACE",
  "confluence_url": "https://yourorg.atlassian.net/wiki",
  "confluence_base_page_id": "12345678",
  "ticket_hanger": "Toil Ticket Hangar",
  "profiles": {
    "work": { "project": "OPS", "board": 7, "story_points_field": "customfield_10021" }
  }
}
```

| Field | Used by | Description |
|---|---|---|
| `project` | `toil-check`, `toil-sync`, `epic list`, `sprint` | Jira project key (e.g. `PROJ`) |
| `board` | `sprint`, `create -sprint` | Agile board ID for sprint commands. Default: the project's only sprint board. `create -sprint` only uses it for tickets in `project` |
| `profiles` | everything | Per-profile overrides: with `-profile work`, the keys under `"work"` replace the matching top-level ones. See **Profiles** above |
| `default_priority` | `create`, `create_ticket`, `create_tickets_batch` | Priority name for new tickets that don't set one |
| `default_components` | `create`, `create_ticket`, `create_tickets_batch` | Component names for new tickets that don't set any |
| `default_labels` | `create`, `create_ticket`, `create_tickets_batch` | Labels for new tickets that don't set any. Not merged: a template or flag with its own labels replaces these |
| `templates_dir` | `create`, `template list`, `create_ticket` | Directory holding named templates (`create -T NAME` reads `NAME.json`), also searched first for `ticket_template.json`. Default for named templates: `~/.config/jira-thing/templates`. `~/` is expanded |
| `story_points_field` | `edit -points`, `create -points`, `point-check` | Story point custom field ID, tried before `customfield_10016` and `customfield_10308`. Find it with `diagnose -find-field "story point"` |
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

#### Named templates

`create -T NAME` starts from a named template instead of a file path, from any directory. A named template is `NAME.json` in the templates directory: `templates_dir` from config, else `~/.config/jira-thing/templates/`. Save one there with `-o`:

```bash
jira-thing template PROJ-42 -o ~/.config/jira-thing/templates/bug.json
jira-thing create -T bug -summary "Login fails on Safari" -y
```

Two are built in, so `-T` works with no files at all:

| Name | Fields |
|---|---|
| `standard` | A `Task` in the configured `project` |
| `toil` | `standard` plus the labels `toil-check` looks for: `toil_marker`, and `toil_team` unless `use_team_field` is true. Only exists when `toil_marker` is set. With `use_team_field`, set the Team field with `-custom` |

A file with the same name replaces a built-in. Names use letters, digits, `-` and `_` only. `template list` shows what's available and where each one comes from:

```bash
jira-thing template list [-json]
# Templates in /Users/me/.config/jira-thing/templates (use with: create -T NAME)
#   bug                  /Users/me/.config/jira-thing/templates/bug.json
#   standard             built-in
#   toil                 built-in
```

---

### `create` — create a ticket from a template

Loads a template file and creates a new Jira ticket with its fields pre-filled. If the template already carries `summary`/`description` (and optionally `parent`), the ticket is created with **zero prompts**. Anything still missing is prompted for interactively, unless `-non-interactive`/`-y` is set — then a missing summary is a fatal error instead of a blocking prompt. This non-interactive path is what scripts, CI, and agents should use.

```bash
jira-thing create [-t template.json] [-y] [-description-file body.md] [flags]
jira-thing create -f tickets.json [-dry-run]   # batch: create every entry in a JSON array
```

| Flag | Default | Description |
|---|---|---|
| `-t` | `ticket_template.json` | Path to the template file |
| `-T` | — | Named template to start from (see **Named templates** above). Can't be combined with `-t` |
| `-no-template` | `false` | Ignore templates on the search path and build the ticket from flags (plus config defaults) alone. Can't be combined with `-t` or `-T` |
| `-f` | — | Path to a JSON array of templates; creates all of them non-interactively and reports how many succeeded |
| `-non-interactive`, `-y` | `false` | Fail with a clear error instead of prompting for a missing summary |
| `-description-file` | — | Path to a markdown file used as the description, overriding both the template and the prompt. `-` reads it from stdin, which also turns on `-y` (stdin can't answer prompts), so pass `-summary` too |
| `-project` | — | Project key (e.g. `PROJ`) or numeric ID — overrides the template. A default template on the search path is **still used** for its other fields (labels, components, priority…); pass `-no-template` to build from flags alone. With no default template on the search path, the ticket is built from flags alone anyway |
| `-summary` | — | Ticket summary — overrides the template and skips the prompt |
| `-type` | — | Issue type name (e.g. `Task`, `Bug`, `Story`) — overrides the template |
| `-priority` | — | Priority name (e.g. `High`, `Medium`, `Low`) — overrides the template |
| `-labels` | — | Comma-separated labels — overrides the template |
| `-components` | — | Comma-separated component names — overrides the template |
| `-assignee` | — | `me`, `unassign`, a name or email (must match one user), or a Jira account ID |
| `-epic` | — | Epic/parent ticket key to link this ticket to — overrides the template |
| `-sprint` | — | Sprint to add the new ticket to right after the create: `current`, `next` or a sprint ID. Resolved before anything is created, so a bad selector (or `prev`, a closed sprint) creates nothing |
| `-board` | `board` in config | Board `-sprint` is looked up on. The configured `board` is only used when the ticket is in the configured `project`; otherwise the ticket's project must have exactly one sprint board |
| `-points` | — | Story points (e.g. `3`, `0.5`), set on the new ticket's edit screen right after the create, because most create screens don't carry the field. Uses `story_points_field` from config, else the usual defaults (same as `edit -points`) |
| `-fix-version` | — | Fix version name; repeatable or comma-separated. The version must already exist in the project (see `release list`) |
| `-custom` | — | Any other field as `"Name=value"` or `"customfield_NNNNN=value"`; repeatable. See **Custom fields** below |
| `-dry-run` | `false` | Print the resolved payload (method, URL, JSON body exactly as it will be POSTed) and exit — no ticket is created. Also prints the follow-up steps that run right after the create: the epic/parent link, the sprint add and the story points. With `-f`, previews every entry in the batch, continuing past per-ticket resolution errors |
| `-json` | `false` | Print `{"key", "url", "status"}` as a single line of JSON instead of styled text |
| `-dedupe-by-summary` | `false` | Skip creating if an open ticket in the same project has the same summary (see below). Also works with `-f` |

These first-class flags mean a ticket can be created end-to-end with no template file at all (`-project` and `-summary` are the minimum) — `-t`/`-f` and flags can also be combined, where the flags always win over whatever the template holds.

> **Markdown in, not Jira wiki markup.** `-description-file` and a template's `description` field are both parsed as GitHub-flavoured markdown and converted to Atlassian Document Format (ADF) before posting — same rule as `update`'s comment body (see below). Old Jira wiki markup (`h1.`, `*bold*`, `{code}`) is **not** understood and will post as literal text.

**Reference fields accept bare strings.** `project`, `issuetype` and `priority` can be written as plain strings in a template or batch entry — `"project": "PROJ"` is sent as `{"key": "PROJ"}` (an all-digit value such as `"10315"` as `{"id": "10315"}`), `"issuetype": "Bug"` as `{"name": "Bug"}`. The full object form still works unchanged.

**Defaults from config.** A field is taken from flags first, then the template (or batch entry), then `default_priority`, `default_components` or `default_labels` in config. A default only fills a field nothing else set; it never merges with one that is set.

**Template search path** (when neither `-t`, `-T` nor `-no-template` is given): `templates_dir` from config (if set) → current directory → binary directory → `$XDG_CONFIG_HOME/jira-thing/` → `~/.config/jira-thing/`. A template found this way is named on stderr (`Using template: <path>`), except with `-json`, since the command line doesn't show it.

**Follow-up steps.** The epic/parent link, `-sprint` and `-points` are applied after the ticket exists. In a `-f` batch file (and `create_tickets_batch`), an entry does the same with `"parent"`, `"sprint"` (`"current"`, `"next"` or a sprint ID, as a string or number) and `"points"` (a number); a bad `sprint` or `points` fails only that entry, before it is created. If one of them fails, the ticket is still created; the command names it, says which step failed, and exits non-zero so a script notices.

A template can set `"parent": "PROJ-10"` to link the new ticket to an epic/parent immediately after creation — same `parent`-then-`Epic Link` fallback as `epic add`, no separate call needed. `-epic` does the same from the command line.

**Duplicate guard.** Re-running a create that may already have succeeded can leave duplicates. With `-dedupe-by-summary`, jira-thing first looks for a ticket in the same project with the same summary (ignoring case and surrounding whitespace) that isn't in a Done status. If it finds one, nothing is created and the command exits 0 naming that ticket, with `"status": "exists"` under `-json`.

- If the check can't complete (search failure, more than 100 similar open tickets), nothing is created and the command exits non-zero. In a batch, that entry is reported as an error and the batch continues.
- With `-f`, repeated entries in the same file are caught too, and the closing tally counts what was skipped.
- With `-dry-run`, the check still runs, since it only reads. A duplicate is reported instead of the payload.
- **Immediate retries are not protected.** Jira's search doesn't show a new ticket straight away (about 3 seconds when measured, and it can vary), so a retry from a separate run inside that window still creates a duplicate. Wait several seconds before retrying. Repeats within one `-f` batch aren't affected.
- It's a best-effort check, not a lock: two simultaneous runs can both create.
- A skipped create doesn't apply `-epic`/`parent` to the existing ticket. If an earlier run created the ticket but failed to link it, use `epic add`.

**Example — interactive:**

```bash
jira-thing create -t templates/bug.json
# Enter ticket summary: Fix login redirect on mobile
# Enter ticket description: Users are redirected to /home instead of the original URL after login on iOS Safari.
# Created ticket: PROJ-99
# URL: https://yourorg.atlassian.net/browse/PROJ-99
```

**Example — non-interactive, fully declarative:**

```json
{
  "project": {"key": "PROJ"},
  "issuetype": {"name": "Task"},
  "summary": "Fix login redirect on mobile",
  "description": "Users are redirected to /home instead of the original URL after login on iOS Safari.",
  "parent": "PROJ-10"
}
```

```bash
jira-thing create -t templates/bug.json -y
# Created ticket: PROJ-99
# URL: https://yourorg.atlassian.net/browse/PROJ-99
```

**Example — no template, all flags:**

```bash
jira-thing create -y -project PROJ -summary "Fix login redirect" -type Bug -priority High \
  -labels backend,urgent -assignee me -epic PROJ-10 -description-file body.md
```

**Example — dry run / scripted:**

```bash
jira-thing create -t templates/bug.json -y -dry-run    # preview the payload, create nothing
jira-thing create -t templates/bug.json -y -json       # {"key":"PROJ-99","url":"https://...","status":"created"}
```

---

### `edit` — edit fields on an existing ticket

Changes fields on an existing ticket non-interactively. Only the flags you pass are touched — everything else on the ticket is left alone. Unlike `update` (which only adds a comment), `edit` mutates the ticket itself via `PUT /rest/api/3/issue/{key}`.

```bash
jira-thing edit <TICKET-KEY> [flags]
```

| Flag | Description |
|---|---|
| `-summary` | New summary/title |
| `-description-file` | Path to a markdown file for the new description (replaces the existing one), or `-` to read it from stdin |
| `-priority` | New priority name (e.g. High, Medium, Low) |
| `-labels` | Comma-separated labels — replaces all existing labels |
| `-label` | Add one label, or remove one with `-label -name`; repeatable, other labels are kept. Can't be combined with `-labels` |
| `-component` | Add or remove (`-name`) one component; repeatable |
| `-fix-version` | Add or remove (`-name`) one fix version; repeatable |
| `-assignee` | `me`, `unassign`, a name or email (must match one user), or a Jira account ID |
| `-parent` | Epic/parent ticket key to link this ticket to |
| `-type` | New issue type, matched case-insensitively. Only types the ticket's edit screen allows; an unknown name lists them. Types with a different workflow or screen need **Move** in Jira |
| `-points` | Story points, e.g. `3` or `0.5`. Written to `story_points_field` from config if it's on the ticket's edit screen, else the first of `customfield_10016`, `customfield_10308` that is |
| `-custom` | Any other field as `"Name=value"` or `"customfield_NNNNN=value"`; repeatable, empty value clears it. See **Custom fields** below |
| `-dry-run` | Print the request (method, URL, and the JSON body with its `fields` and `update` parts) and exit — nothing is changed |
| `-json` | Print `{"key", "url", "status"}` as a single line of JSON instead of styled text |

At least one flag is required. `-description-file` is parsed as markdown (not Jira wiki markup) — see the note under `update` below.

**Example:**

```bash
jira-thing edit PROJ-99 -priority High -labels bug,urgent -assignee me
# Edited ticket: PROJ-99
# URL: https://yourorg.atlassian.net/browse/PROJ-99

jira-thing edit PROJ-99 -priority High -dry-run   # preview, change nothing
jira-thing edit PROJ-99 -priority High -json      # {"key":"PROJ-99","url":"https://...","status":"edited"}
jira-thing edit PROJ-99 -type Task -points 0.5    # Bug → Task, half a point
jira-thing edit PROJ-99 -label reviewed -label -stale -fix-version v2.1   # add, remove, add
git log --oneline main..HEAD | jira-thing edit PROJ-99 -description-file -   # description from a pipe
```

#### Custom fields

`-custom` on `create` and `edit` sets any field by its name (case-insensitive) or ID, converting the value by the field's type:

| Field type | Value | Sent as |
|---|---|---|
| Number | `3`, `0.5` | a number |
| Text (single line) | `Platform` | a string |
| Text (paragraph) | markdown | ADF |
| Select list | `Red` | `{"value": "Red"}` |
| User | `me`, a name, an email, an account ID | `{"accountId": …}` |
| Date | `2026-12-31` | checked, then a string |
| Version, component, priority | `v2.0` | `{"name": "v2.0"}` |
| Sprint | `1234` (sprint ID) | a number |
| Team | team ID | a string |
| Lists of the above, labels | `a, b` | an array |

```bash
jira-thing edit PROJ-99 -custom "Story point estimate=3" -custom "Start date=2026-11-20"
jira-thing create -y -project PROJ -summary "Spike" -custom "Team=8f2c…" -custom "Reviewers=me, ada"
jira-thing edit PROJ-99 -custom "customfield_10021=" -dry-run    # clear a field
```

A name that matches several fields fails with their IDs; use the ID. Find fields with `diagnose -find-field "part of the name"`. Cascading selects and other unusual types aren't supported and say so. Templates still never copy custom fields; only `-custom` sets them.

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

### `transition` — move a ticket to a new state, non-interactively

The scriptable complement to `state`: no TUI, so it works from scripts, CI, and agents. Available states are read live from the ticket, same as `state`.

```bash
jira-thing transition <TICKET-KEY> [STATE] [flags]
```

Omit `STATE` to list the states available from the ticket's current status instead of moving it.

| Flag | Description |
|---|---|
| `-comment` | Optional comment (markdown) to add as part of the transition |
| `-resolution` | Optional resolution name (e.g. `Fixed`, `Won't Do`) |
| `-assignee` | Optional assignee to set during the transition: `me`, `unassign`, a name, an email, or a Jira account ID |
| `-dry-run` | Print the resolved payload and exit — nothing is changed |

State names match case-insensitively. An unrecognised state prints the available list instead of a bare error.

**Example:**

```bash
jira-thing transition PROJ-101          # no STATE given — lists available states
#   In Review
#   Done
#   Blocked

jira-thing tr PROJ-101 done -resolution Fixed
# Moved PROJ-101 to Done
```

---

### `assign` — assign a ticket by name, email or account ID

```bash
jira-thing assign <TICKET-KEY> <me|unassign|default|NAME|EMAIL|ACCOUNT_ID> [-dry-run] [-json]
```

| Assignee | Meaning |
|---|---|
| `me` | The authenticated user |
| `unassign` | Clear the assignee |
| `default` | The project's default assignee |
| a name or email | Looked up among active users; an exact name or email match wins over partial ones |
| an account ID | Used as-is, with no lookup |

A name that matches several people opens a picker in a terminal. In scripts (stdin or stdout not a terminal) it fails instead, listing each candidate's account ID to retry with. `-assignee` on `create`, `edit` and `transition` accepts names and emails the same way, without the picker.

**Example:**

```bash
jira-thing assign PROJ-101 "ada lovelace"
# Assigned PROJ-101 to Ada Lovelace <ada@example.com> (5b10ac8d82e05b22cc7d4ef5)

jira-thing as PROJ-101 unassign -json
# {"key":"PROJ-101","url":"https://...","status":"assigned","accountId":""}
```

---

### `menu` — interactive menu for everything else

Opens a persistent home-screen TUI covering the full set of ticket operations jira-thing supports: search, describe, edit fields, comment, worklog, attach, subtasks, link/unlink, clone, delete, boards/sprints, projects/releases, and whoami. This is the jira-cli-parity surface — one place to reach every operation without memorising a flag for each.

```bash
jira-thing menu
```

Pick an action from the list; you'll be prompted for whatever it needs directly in the terminal. After an action finishes, press enter to return to the menu. Unlike the one-shot commands, a failed action (bad key, invalid JQL) reports the error and drops you back at the menu instead of exiting the program. **Create Ticket** first asks for an optional sprint (`current`, `next` or an ID) and story points, the same as `create -sprint` and `-points`, then runs the usual create prompts.

Every action that needs a ticket key (Update Fields, Change State, Add Comment, Clone, Delete, Link/Unlink, etc.) opens a picker pre-populated with your open tasks instead of a bare text prompt — pick one with `enter`, or select the first row (**✎ Type a ticket key manually…**) to type any key, including tickets not assigned to you.

| Keys | Action |
|---|---|
| `↑`/`↓` | Navigate |
| `enter` | Select |
| `q` / `esc` / `ctrl+c` | Quit the menu |

**My Tasks** and **Search** open the ticket list in an interactive table with quick-action keys on the highlighted row, so you don't need to retype the key into a follow-up prompt. Navigation supports vim-style keys (`j`/`k`, `g`/`G` for top/bottom) in addition to arrows, courtesy of the underlying table widget.

| Key | Action |
|---|---|
| `v` | View the ticket's full details |
| `o` | Open the ticket in the browser |
| `m` | Transition the ticket to a new state |
| `c` | Copy the ticket's URL to the clipboard |
| `ctrl+k` | Copy the ticket key to the clipboard |
| `ctrl+r` | Refresh the list (re-runs the same search) |

Clipboard copy uses `pbcopy` (macOS), `clip` (Windows), or `xclip` (Linux — must be installed separately; there's no dependency-free universal fallback).

---

### `list` — search with jira-cli-style filter flags

Searches tickets using a JQL query built from flags, mirroring jira-cli's `issue list` ergonomics. Flags combine with AND; add `-jql` for anything the flags don't cover.

```bash
jira-thing list [flags]
```

| Flag | Description |
|---|---|
| `-a` | Assignee: `me`, `x` (unassigned), `~x` (assigned to someone), a name, or `~name` (not that person) |
| `-r` | Reporter: same shorthand as `-a` |
| `-y` | Priority name, or `~name` to exclude |
| `-s` | Status name, or `~name` to exclude |
| `-t` | Issue type name, or `~name` to exclude |
| `-R` | Resolution name; `x` for unresolved, `~x` for resolved |
| `-p` | Project key |
| `-C` | Component (repeatable; matches any) |
| `-l` | Label (repeatable: `-l backend -l urgent`) |
| `--created` | Created on or after: relative (`-7d`), keyword (`day`/`week`/`month`/`year`, meaning the start of the current one), or absolute (`2026-01-01`) |
| `--created-before` | Created before, same formats |
| `--updated` | Same format as `--created`, filtered on the updated date |
| `-w` | Only tickets you're watching |
| `-jql` | Raw JQL, AND-ed in alongside the flags above |
| `--order-by` | Sort field (default `created`) |
| `--reverse` | Sort ascending instead of descending |
| `-limit` | Most tickets to return (default 100, max 1000; pages through Jira's results) |
| `-json` | Print matching tickets as a JSON array instead of styled text — `[]` on no matches, never prose, so scripts don't need a special case. With `-columns`, each object holds just those columns |
| `-plain` | Tab-aligned table with no colours, for reading or `cut`/`awk` |
| `-csv` | CSV. Cells starting with `=`, `+`, `-` or `@` get a leading `'` so a spreadsheet won't run ticket text as a formula |
| `-columns` | Comma-separated columns for `-plain`/`-csv`/`-json`: `key`, `summary`, `status`, `priority`, `type`, `assignee`, `reporter`, `resolution`, `labels`, `components`, `created`, `updated`, `url`. Alone, it implies `-plain`. Default: `key,type,status,priority,assignee,updated,summary` |
| `-no-headers` | Leave out the header row in `-plain` and `-csv` |

A `-jql` with its own `ORDER BY` and no other flags is sent unchanged; otherwise the flags are AND-ed with it and `--order-by` sets the sort.

**Examples:**

```bash
# High priority issues assigned to me, in progress, with the backend label
jira-thing list -a me -yHigh -s"In Progress" -lbackend

# Issues created this week, unassigned
jira-thing list --created week -a x

# Tickets I'm watching in project PROJ, oldest first
jira-thing list -w -p PROJ --reverse

# Scripted: JSON array of key/url/summary/status/priority/updated
jira-thing list -a me -s"In Progress" -json

# Unresolved bugs in the api component, as CSV for a spreadsheet
jira-thing list -t Bug -R x -C api -csv -columns key,status,assignee,summary

# Keys only, for a loop
jira-thing list -p PROJ -created-before -90d -R x -columns key -no-headers -limit 500
```

---

### `epic` — epic management

Jira has two incompatible epic models depending on how a project is configured: team-managed projects link issues via the generic `parent` field, company-managed projects use a classic `Epic Link` custom field (ID varies per instance). Every `epic` subcommand that touches issue-to-epic links tries `parent` first and falls back to `Epic Link` automatically — you don't need to know which model your project uses.

```bash
jira-thing epic list [-p PROJECT]                        # list every epic in a project (default: config's "project")
jira-thing epic describe <EPIC-KEY>                      # list issues under a specific epic
jira-thing epic create -p PROJ -n NAME -s SUMMARY        # create an epic
jira-thing epic add <EPIC-KEY> <ISSUE-1> [ISSUE-2 ...]   # add issues to an epic
jira-thing epic remove <ISSUE-1> [ISSUE-2 ...]           # remove issues from their epic
jira-thing epic explore [-p PROJECT]                     # browse epics and their issues side by side
```

`epic list` defaults to the `project` key set in `~/.config/jira-thing/jira-thing.json`; pass `-p` to override, or set the config value if neither is present and the command errors out.

`epic create`'s `-n` (epic name) only takes effect on instances with a classic `Epic Name` field — team-managed projects don't have one, so it's silently omitted there.

---

### `sprint` — sprints and their issues (alias: `sp`)

```bash
jira-thing sprint list                          # sprints on the board, oldest first
jira-thing sprint list -state active,future     # only open sprints
jira-thing sprint list -current                 # issues in the active sprint
jira-thing sprint list -prev -jql "assignee = currentUser()"
jira-thing sprint list -sprint 1234 -json       # issues in sprint 1234, as JSON
jira-thing sprint add current PROJ-1 PROJ-2     # move tickets into the active sprint
jira-thing sprint add next PROJ-3 -dry-run      # show the request, move nothing
jira-thing sprint explore                       # browse sprints and their issues side by side
```

The board comes from `-b BOARD`, then `"board"` in config, then the only sprint board in `-p PROJECT` (default: config's `"project"`). A project with several sprint boards fails with a list of them; pass `-b` or set `"board"`. "Sprint board" means any board that isn't kanban: company-managed scrum boards and team-managed boards alike.

`-current` is the active sprint (the latest started, if several are active), `-prev` the last closed sprint and `-next` the first future one. `sprint add` takes `current`, `next` or a sprint ID, and sends more than 50 keys in batches. `-json` prints `{sprintId, sprintName, keys, status}`.

---

### Explorer — epics or sprints beside their issues

```bash
jira-thing epic explore              # epics in the configured project
jira-thing epic explore -p PROJ
jira-thing sprint explore            # active and future sprints, opening on the active one
jira-thing sprint explore -all       # include closed sprints
```

A full-screen view with the epics or sprints on the left and the selected one's issues on the right. Issues load as you move through the list and are kept until you reload. The menu has **Epic Explorer** and **Sprint Explorer** entries too.

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Switch between the list and the issues |
| `↑`/`↓`, `j`/`k`, `g`/`G` | Move |
| `enter`, `→` | Open the selected epic or sprint's issues |
| `esc`, `←` | Back to the list (quits from the list) |
| `v` / `o` / `m` | View, open in the browser, or move the issue to another state |
| `c` / `ctrl+k` | Copy the issue's URL or key |
| `r` | Reload the open epic or sprint |
| `?` | Help |
| `q` | Quit |

After `v`, `o`, `m`, `c` or `ctrl+k`, the action runs and you're asked to press enter, then the explorer reopens on the same epic or sprint. The ticket table (my-tasks, search) also has `?` help now.

---

### `board list`, `project list`, `release list` — what's there

```bash
jira-thing board list                    # every Agile board you can see
jira-thing board list -p PROJ            # one project's boards (find the ID for "board" in config)
jira-thing project list                  # every project you can browse
jira-thing release list                  # config's project, newest release date first
jira-thing release list -p PROJ -unreleased
jira-thing release list -all -json       # include archived releases, as JSON
```

All three page through Jira's results, so long lists aren't cut off at 50. `release list` hides archived releases unless you pass `-all`, and labels each one `unreleased`, `overdue`, `released` or `archived`. Each command prints the raw Jira records with `-json`.

---

### `worklog add` — log time (alias: `wl`)

```bash
jira-thing worklog add PROJ-101 "2h 30m"
jira-thing wl add PROJ-101 1d 4h -comment "Migration **dry run** with ops"
jira-thing wl add PROJ-101 45m -dry-run       # check the duration, log nothing
```

The duration uses Jira's units (`w`, `d`, `h`, `m`, decimals allowed) and is checked before anything is sent. It can be quoted or given as separate words. `-comment` takes markdown. `-json` prints `{key, url, status, timeSpent}`.

---

### `delete` — permanently delete a ticket

```bash
jira-thing delete PROJ-101                 # shows the ticket and asks first
jira-thing delete PROJ-101 -dry-run        # what would go, including subtasks
jira-thing delete PROJ-101 -cascade -y     # with its subtasks, no prompt
```

Deleting cannot be undone. Without `-y`, `delete` shows the ticket and asks; in a script (no terminal) it refuses unless you pass `-y`. A ticket with subtasks is refused unless you pass `-cascade`, and the error lists them. `-json` prints `{key, summary, subtasks, status}`.

---

### `clone` — copy a ticket

```bash
jira-thing clone PROJ-101                                 # "CLONE - <summary>"
jira-thing clone PROJ-101 -summary "Patch web-02" -assignee ada
jira-thing clone PROJ-101 -replace web-01:web-02 -replace prod:staging -dry-run
jira-thing clone PROJ-101 -labels q4,ops -priority High -json
```

The copy inherits the project, type, priority, labels, components and description. `-replace find:replace` (split at the first colon, repeatable) swaps text in the summary and in the description's text, leaving the source untouched. `-labels` replaces the inherited labels; `-assignee` accepts names and emails like `assign`.

---

### `me` — who am I

```bash
jira-thing me          # your email (or account ID, if your profile hides it)
jira-thing me -json    # {accountId, displayName, emailAddress}
```

---

### `watch`, `unwatch`, `watchers`

```bash
jira-thing watch PROJ-101                 # you start watching
jira-thing watch PROJ-101 "ada lovelace"  # someone else (needs permission to manage watchers)
jira-thing unwatch PROJ-101 -dry-run
jira-thing watchers PROJ-101 -json
```

The user defaults to you and is resolved like `assign`: a name, email or account ID, with a picker in a terminal when a name matches several people. `watch` and `unwatch` print `{key, url, status, accountId, displayName}` with `-json`. The menu's **Watch / Unwatch** flips whether you're watching the picked ticket.

---

### `open` — open a ticket in the browser

```bash
jira-thing open PROJ-101   # opens the ticket
jira-thing open            # opens the Jira base URL
```

---

### `completion` — shell completion

Prints a completion script for bash or zsh, covering command names, aliases and the sub-commands of `epic`, `sprint`, `board`, `project`, `release`, `worklog`, `conf` and `completion`. Flags are not completed.

```bash
jira-thing completion bash >> ~/.bashrc
jira-thing completion zsh > "${fpath[1]}/_jira-thing"
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
jira-thing last-comment <TICKET-KEY> [-plain] [-raw]
```

| Flag | Description |
|---|---|
| `-plain` | Force plain text output even on a terminal (auto-detected anyway when piping to another command — no ANSI escape codes end up in the output either way) |
| `-raw` | Print the stored comment body as raw ADF JSON instead of converting it to markdown |

---

### `describe` — dump full ticket as rendered markdown

Fetches a Jira ticket and renders its key, summary, status, priority, assignee, linked tickets, and description as formatted markdown in the terminal. Linked tickets appear under a **Linked Tickets** heading as `<relation> <KEY>: <summary> (<status>)` — e.g. `is blocked by PROJ-9: Upstream (Done)` — and the section is omitted when the ticket has no links. The latest comment follows under **Comments**, if the ticket has any. The table and explorer `v` (view) key shows the same.

```bash
jira-thing describe <TICKET-KEY> [-comments N] [-plain] [-raw]
```

| Flag | Description |
|---|---|
| `-comments` | How many of the latest comments to show, oldest first, under **Comments (latest N of TOTAL)**. Default `1`; `0` for none; max 100. The section is left out when the ticket has no comments |
| `-plain` | Force plain text output even on a terminal (auto-detected anyway when piped) |
| `-raw` | Print the stored issue fields as raw JSON, including the ADF description, instead of rendering markdown. Comments are left out unless `-comments` is given (so existing scripts keep the bare issue); with it, prints `{issue, comments, commentTotal}` |

**Example:**

```bash
jira-thing de PROJ-42
jira-thing de PROJ-42 | cat     # auto-plain — no escape codes to strip
jira-thing de PROJ-42 -raw      # raw JSON fields, e.g. to inspect the stored ADF
jira-thing de PROJ-42 -comments 5   # with the latest five comments
```

---

### `link-types` — list the instance's issue link types

Link types are configured per Jira instance (`Blocks` isn't guaranteed to exist everywhere), so they are always read live.

```bash
jira-thing link-types [-json]
```

| Flag | Description |
|---|---|
| `-json` | Print the types as a JSON array of `{id, name, inward, outward}` |

---

### `link` — link two tickets

Creates a formal Jira issue link. Argument order sets the direction: the link type's **outward** text reads from `OUTWARD-KEY` to `INWARD-KEY`, so `jira-thing link PROJ-100 PROJ-200 -type Blocks` means *PROJ-100 blocks PROJ-200*.

```bash
jira-thing link <OUTWARD-KEY> <INWARD-KEY> -type <name> [-dry-run] [-json]
```

| Flag | Description |
|---|---|
| `-type` | Link type name, matched case-insensitively against `link-types`. If omitted or unknown, the command exits non-zero and prints the live list |
| `-dry-run` | Show the request payload without creating the link |
| `-json` | Print `{outward, inward, type, status}` as JSON instead of styled text |

**Example:**

```bash
jira-thing link PROJ-101 PROJ-42 -type Relates
jira-thing ln PROJ-100 PROJ-200 -type blocks -json
```

---

### `unlink` — remove a link between two tickets

Removes the link connecting the two tickets, whichever link type it is.

```bash
jira-thing unlink <KEY> <OTHER-KEY>
```

---

### `remote-link` — attach a web link to a ticket

```bash
jira-thing remote-link <KEY> -url <url> [-title <title>]
```

| Flag | Description |
|---|---|
| `-url` | The URL to link (required) |
| `-title` | Display text for the link. Default: the URL |

---

### `attach` — attach files to a ticket

Uploads one or more local files, or piped stdin content under an explicit name, as attachments on an existing Jira ticket.

```bash
jira-thing attach <TICKET-KEY> <file-path>
jira-thing attach <TICKET-KEY> -f file1 -f file2    # multiple files in one call
jira-thing attach <TICKET-KEY> -stdin -name notes.md
```

| Flag | Description |
|---|---|
| `-f` | File to attach (repeatable: `-f a.png -f b.png`) |
| `-stdin` | Read attachment content from stdin |
| `-name` | Attachment filename when using `-stdin` (required with `-stdin`) |

**Example:**

```bash
jira-thing at PROJ-42 screenshot.png
# Attached screenshot.png to PROJ-42
# URL: https://yourorg.atlassian.net/browse/PROJ-42

jira-thing at PROJ-42 -f before.png -f after.png
echo "build log excerpt" | jira-thing at PROJ-42 -stdin -name build.log
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

Converts a local markdown file to Confluence storage format and publishes it. By default launches the space browser TUI to choose a parent page — `-page-id`/`-parent-id` skip that entirely for non-interactive use (scripts, agents, anywhere without a TTY).

```bash
jira-thing conf upload <file.md> [-title "Page Title"]
jira-thing conf upload <file.md> -page-id 1514045707        # update this exact page, no TUI
jira-thing conf upload <file.md> -parent-id 12345678         # create under this exact parent, no TUI
```

| Flag | Default | Description |
|---|---|---|
| `-title` | Filename without extension | Title for the Confluence page |
| `-page-id` | — | Update this exact page by ID. Skips the title lookup and the parent picker entirely — can't create a duplicate under the wrong parent. Keeps the page's existing title unless `-title` is also given |
| `-parent-id` | — | Create under this exact parent page ID, skipping the picker. Ignored if `-page-id` is also given |

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

### `self-update` — download and install the latest release

Downloads the latest GitHub release matching your OS/arch, verifies it against a signed checksums manifest, and replaces the running binary. Refuses to downgrade and refuses a `dev` build (nothing to compare against — download a tagged release instead).

```bash
jira-thing self-update
```

The download is verified fully in a temp file before anything touches the install location, so a bad download never risks the binary you're running. If the final install step fails — most commonly because you don't own the directory the binary lives in (e.g. installed by a different user, or into a system path) — the error says so explicitly and suggests installing somewhere you own, or re-running with elevated privileges; the original binary is left untouched either way.

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

Generated from the tool registrations in `internal/mcpserver` — run `make gen-docs` after adding or changing a tool (`go test ./...` fails if this table drifts from the code).

<!-- mcp-tools:start -->
| Tool | Description |
|---|---|
| `add_attachment` | Upload a local file as an attachment on a Jira ticket. The file must be accessible on the filesystem where jira-thing is running. |
| `add_comment` | Add a markdown comment to a Jira ticket. The markdown is converted to Atlassian Document Format automatically. |
| `add_remote_link` | Attach a remote web link (e.g. a URL to external documentation or a related resource) to a Jira ticket. |
| `add_to_epic` | Add issues to an epic. Tries the "parent" field then falls back to the classic "Epic Link" custom field per issue. |
| `add_to_sprint` | Move tickets into a sprint. More than 50 keys are sent in batches. |
| `add_worklog` | Log time spent against a Jira ticket. |
| `assign_ticket` | Assign a Jira ticket to someone, by name, email or account ID, or unassign it |
| `check_story_points` | Check the current user's open-sprint tickets for missing story points. Checks a few common story-point field names/IDs; if this instance uses a different custom field, use list_fields to find its ID and inspect tickets with describe_ticket instead. |
| `clone_ticket` | Clone an existing Jira ticket, optionally overriding summary, priority, assignee or labels, and replacing text in the summary and description. |
| `confluence_browse` | List child pages under a Confluence page. Use to navigate the page hierarchy. |
| `confluence_create_page` | Create a new Confluence page under a parent page. Content is provided as markdown and converted to Confluence storage format. |
| `confluence_export_page` | Export a Confluence page as markdown, given its numeric page ID. Also resolves every embedded document the page references — images, attached files, view-file/multimedia macros, draw.io diagrams — downloading them into an attachments/ subdirectory when output_dir is given. |
| `confluence_get_page` | Fetch a Confluence page by space key and title, or by page ID. Returns the page's storage-format body. |
| `confluence_update_page` | Update an existing Confluence page's content. Content is provided as markdown and converted to Confluence storage format. |
| `create_epic` | Create an epic. Jira has two incompatible epic models depending on how the project is configured (team-managed vs company-managed); this tool sets the classic "Epic Name" field when the instance has one, and is a no-op on team-managed projects where it doesn't exist. |
| `create_subtask` | Create a subtask under an existing Jira ticket. Inherits project, priority, labels, and components from the parent. |
| `create_ticket` | Create a new Jira ticket with the specified fields |
| `create_tickets_batch` | Create several Jira tickets in one call. Each entry is a ticket object, e.g. {"project":"PROJ","issuetype":"Task","summary":"...","description":"markdown","labels":["x"],"parent":"PROJ-10"}. Entries are created independently: one failing does not stop the rest, and the result reports each one. An entry may also carry "sprint" ("current", "next" or a sprint ID) and "points" (a number), applied right after its create. |
| `delete_ticket` | Permanently delete a Jira ticket. This cannot be undone: run with dry_run first and confirm with the user. |
| `describe_ticket` | Fetch a Jira ticket's full details including summary, status, priority, assignee, linked tickets, and description, with its latest comment (or the latest N) |
| `diagnose_connection` | Check jira-thing's Jira connection: the configured URL and email, the API token's format (no part of the token is returned), and an authenticated API call. Use when other tools fail with auth or connection errors; the result suggests a fix for 401/403 failures. |
| `fetch_template` | Fetch a Jira ticket's reusable fields (project, issue type, priority, labels, components, description, assignee) as a JSON template, for use as the base of create_tickets_batch entries. By default the assignee becomes a marker meaning "whoever creates from this template". Saving a template as the CLI's default is left to the user (jira-thing template KEY), so an agent can't change what later creates use. |
| `last_comment` | Fetch the most recent comment on a Jira ticket, rendered as markdown |
| `link_tickets` | Link two Jira tickets together (e.g. mark one as blocking another). The link_type must match one of the types returned by list_link_types — call that tool first if unsure. |
| `list_boards` | List Agile (Scrum/Kanban) boards visible to the current user, optionally filtered to a project. |
| `list_epic_issues` | List the issues under an epic. Tries the "parent" field (team-managed projects) then falls back to the classic "Epic Link" custom field (company-managed projects). |
| `list_epics` | List every epic in a project. |
| `list_fields` | List every field known to the Jira instance, including custom fields with their customfield_XXXXX IDs. Custom fields are configured per instance, so their IDs must always be looked up here rather than assumed — the same field name can map to a different ID on different boards. |
| `list_link_types` | List the issue link types configured on this Jira instance (e.g. Blocks, Relates, Duplicate). Link types are instance-configurable, so always call this before link_tickets rather than assuming a type name is valid. |
| `list_projects` | List every Jira project the current user can access |
| `list_sprint_issues` | List the issues in a sprint (up to 100). Defaults to the current sprint. |
| `list_sprints` | List the sprints on a board, oldest first. |
| `list_templates` | List the named ticket templates create_ticket can start from (its template argument): the user's files in the templates directory, and the built-ins "standard" (a Task in the configured project) and "toil" (adds the toil labels, when toil_marker is configured). A file overrides a built-in of the same name. Templates are read-only to agents. |
| `list_toil_tickets` | List toil tickets for the project/team configured in ~/.config/jira-thing/jira-thing.json (project, toil_marker label, and team filter). Fails if those aren't configured — this tool is specific to instances that have set up toil tracking. |
| `list_transitions` | List the workflow states a Jira ticket can currently move to. Boards customise workflows heavily (a personal board may have 4 states, a client board 12), so always call this before transition_ticket rather than assuming a state name is valid. |
| `list_versions` | List a project's releases/versions, newest release date first. Archived releases are hidden unless include_archived is set. |
| `list_watchers` | List who is watching a Jira ticket |
| `my_tasks` | List open Jira tasks assigned to the current user |
| `remove_from_epic` | Remove issues from their epic. Tries the "parent" field then falls back to the classic "Epic Link" custom field per issue. |
| `search_tickets` | Search Jira tickets with JQL and/or filters (AND-ed together). Returns key, status, priority, summary and updated date, or the chosen columns. |
| `search_users` | Find active Jira users by name or email, returning their account IDs. Use it to pick an assignee when a name matches several people. |
| `sync_toil` | Sync open toil tickets to Confluence: one child page per ticket under the configured ticket_hanger page (keeping any notes already written on it), then rewrite the hanger page's index. Needs the toil settings plus confluence_space and ticket_hanger in ~/.config/jira-thing/jira-thing.json. |
| `transition_ticket` | Move a Jira ticket to a new workflow state. The target_status must match one of the states returned by list_transitions for this ticket — call that tool first if unsure what states are available. |
| `unlink_tickets` | Remove the link between two Jira tickets, whichever link type currently connects them. |
| `unwatch_ticket` | Stop watching a Jira ticket, for yourself or someone else |
| `update_ticket` | Edit fields on an existing Jira ticket (summary, description, priority, labels, assignee, parent, issue type, story points). Only the fields you provide are changed; omitted fields are left untouched. To change workflow state, use transition_ticket instead. |
| `watch_ticket` | Start watching a Jira ticket, for yourself or someone else |
| `whoami` | Show the current authenticated Jira user (account ID, display name, email) |

<!-- mcp-tools:end -->

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
| `edit` | `ed` |
| `my-tasks` | `mt` |
| `state` | `sta` |
| `transition` | `tr` |
| `assign` | `as` |
| `menu` | `m` |
| `list` | `ls` |
| `sprint` | `sp` |
| `worklog` | `wl` |
| `open` | `o` |
| `update` | `up` |
| `last-comment` | `lc` |
| `attach` | `at` |
| `describe` | `de` |
| `link-types` | `lt` |
| `link` | `ln` |
| `unlink` | `ul` |
| `remote-link` | `rl` |
| `toil-check` | `tc`, `toil` |
| `toil-sync` | `ts` |
| `point-check` | `pc` |
| `conf browse` | `conf br` |
| `conf upload` | `conf up` |
| `subtask` | `st` |
| `diagnose` | `diag` |
| `check-update` | `cu` |
| `self-update` | `su` |

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
