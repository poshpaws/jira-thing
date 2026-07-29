---
name: jira-thing
description: CLI tool for cloning, creating, updating, and listing Jira tickets via JSON templates.
---

# jira-thing

Go CLI tool that manages Jira tickets through local JSON templates. Credentials stored in OS keyring.

## When to use

- Clone an existing Jira ticket as a reusable template
- Create a new Jira ticket from a saved template
- Add a comment to an existing ticket (markdown supported — converted to ADF)
- Show the last comment on a ticket, rendered as markdown
- List open tasks assigned to you, including stale ones
- Find current-sprint tickets missing story points
- List or sync TOIL tickets (Confluence)
- Diagnose API connectivity and credential problems
- Clear stored Jira credentials

## Commands

### `template <TICKET-KEY> [-o output.json]` (alias: `te`)

Fetch an existing ticket from Jira and save its fields as a local JSON template.

```bash
jira-thing template PROJ-123
jira-thing template PROJ-123 -o my-template.json
```

Saves `ticket_template.json` in the current directory by default. Output includes project, issue type, priority, labels, components, and assignee — fields reused when creating new tickets.

---

### `create [-t template.json]` (alias: `cr`)

Create a new Jira ticket from a template. Prompts for summary and description interactively.

```bash
jira-thing create
jira-thing create -t /path/to/template.json
```

Without `-t`, searches for `ticket_template.json` in:
1. Current directory
2. Same directory as the `jira-thing` binary
3. `$XDG_CONFIG_HOME/jira-thing/`
4. `~/.config/jira-thing/`

Must run `template` first to generate a template file.

---

### `update <TICKET-KEY> [-stdin]` (alias: `up`)

Add a comment to an existing ticket. Opens `$EDITOR` by default; use `-stdin` for piped input.

```bash
jira-thing update PROJ-123
echo "Deployed to staging." | jira-thing update PROJ-123 -stdin
```

Requires `$EDITOR` to be set unless `-stdin` is used. **GitHub-flavoured markdown is supported** — comment text is parsed as GFM and converted to structured ADF, so headings, lists, code blocks, tables, strikethrough, and links render properly in the Jira web UI.

**Do not use old Jira wiki markup** (`h1.`, `*bold*`, `{code}`, `[link|url]`) — parser is GFM-only, won't convert it. Posted as literal text. Use `#`, `**bold**`, fenced code blocks, `[text](url)` instead.

---

### `last-comment <TICKET-KEY>` (alias: `lc`)

Show the most recent comment on a ticket, converted from ADF and rendered as markdown in the terminal.

```bash
jira-thing last-comment PROJ-123
```

---

### `my-tasks [-notupdated]` (alias: `mt`)

List open Jira tasks assigned to the current user, ordered by last updated (descending).

```bash
jira-thing my-tasks
jira-thing my-tasks -notupdated
```

`-notupdated` filters to tasks with no activity in the last 3 business days (stale tasks), ordered oldest-first.

---

### `point-check` (alias: `pc`)

List tickets in the current open sprint assigned to you and report which are missing story points.

```bash
jira-thing point-check
```

---

### `toil-check` (aliases: `toil`, `tc`)

List toil tickets from the last week. Filters on `project`, `toil_marker`, and a team match — see [Configuration](#configuration) for how the team match is chosen.

```bash
jira-thing toil-check
```

---

### `toil-sync` (alias: `ts`)

Sync TOIL tickets to Confluence. Uses the same team match as `toil-check`, plus `confluence_space` and `ticket_hanger`.

```bash
jira-thing toil-sync
```

---

### `diagnose` (alias: `diag`)

Test API connectivity and stored credentials. Use when commands fail with auth or network errors.

```bash
jira-thing diagnose
```

Also supports lookups used to fill in `~/.config/jira-thing/jira-thing.json` — see [Configuration](#configuration):

```bash
jira-thing diagnose -userid                        # print only your accountId
jira-thing diagnose -list-fields                    # list every field on the Jira instance
jira-thing diagnose -find-field team                # search the field registry by name
jira-thing diagnose -team PROJ-123                  # print PROJ-123's Team field value + resolved team ID
jira-thing diagnose -team PROJ-123 -team-field customfield_10005   # ...against a non-default field ID
```

---

### `check-update` (alias: `cu`)

Check GitHub for newer jira-thing releases.

```bash
jira-thing check-update
```

---

### `clear-auth`

Remove stored Jira credentials from the OS keyring.

```bash
jira-thing clear-auth
```

---

### `version`

Show the installed version.

```bash
jira-thing version
```

---

## Typical workflow

```bash
# 1. Clone an existing ticket as template
jira-thing template PROJ-123

# 2. Create a new ticket using that template
jira-thing create

# 3. Add a comment to an existing ticket
jira-thing update PROJ-456

# 4. Read the last comment back
jira-thing last-comment PROJ-456

# 5. Check your open tasks
jira-thing my-tasks
```

## Template search path

Templates are plain JSON files. Place `ticket_template.json` in one of the search path locations above to avoid specifying `-t` on every `create` invocation.

## Configuration

`toil-check`/`toil-sync` (and `update`'s `$EDITOR` fallback) read settings from `~/.config/jira-thing/jira-thing.json`:

```json
{
  "project": "CRSS",
  "toil_marker": "ECP_TOIL",
  "toil_team": "ECP_SEC_TEAM",
  "team": "c4cb9231-6ac0-44e7-bd76-64331a96af81",
  "use_team_field": true,
  "editor": "nvim",
  "confluence_space": "ECP",
  "ticket_hanger": "ECP-TOIL-Hanger"
}
```

| Field | Used by | Description |
|---|---|---|
| `project` | toil-check, toil-sync | Jira project key |
| `toil_marker` | toil-check, toil-sync | Label that marks a ticket as toil |
| `toil_team` | toil-check, toil-sync | **Legacy** team match: a second label to require (used when `use_team_field` is `false`, the default) |
| `team` | toil-check, toil-sync | Jira team UUID to match via the ticket's Team field (used when `use_team_field` is `true`) |
| `use_team_field` | toil-check, toil-sync | `false` (default): match `toil_team` as a label. `true`: match `team` against the Team field instead. Not every team applies the same labelling convention, so this switch exists per-user |
| `editor` | update | Fallback editor if `$EDITOR` is unset |
| `confluence_space` | toil-sync | Confluence space key to sync TOIL tickets into |
| `ticket_hanger` | toil-sync | Confluence page title/ID that links to synced children |

### Getting `team` (the Team field UUID)

The Team field's customfield ID varies per Jira instance — don't guess it. Confirm it, then pull a real ticket's team ID:

```bash
# 1. Confirm the real customfield ID for "Team" (don't assume it's customfield_10001)
jira-thing diagnose -find-field team

# 2. Fetch a ticket that has the Team field set, using the ID from step 1
jira-thing diagnose -team PROJ-123 -team-field customfield_10001
#   → prints the raw field value plus a resolved "team ID: <uuid> (name: ...)" line
```

Put that UUID in `team`, set `use_team_field: true`. If the JQL `"Team[Team]" = <uuid>` isn't supported on your Jira instance, leave `use_team_field: false` and use the legacy `toil_team` label match instead.
