---
name: jira-thing
description: CLI tool for cloning, creating, updating, and listing Jira tickets via JSON templates.
---

# jira-thing

Go CLI tool that manages Jira tickets through local JSON templates. Credentials stored in OS keyring.

## When to use

- Clone an existing Jira ticket as a reusable template
- Create a new Jira ticket from a saved template
- Add a comment to an existing ticket
- List open tasks assigned to you, including stale ones
- Clear stored Jira credentials

## Commands

### `template <TICKET-KEY> [-o output.json]`

Fetch an existing ticket from Jira and save its fields as a local JSON template.

```bash
jira-thing template PROJ-123
jira-thing template PROJ-123 -o my-template.json
```

Saves `ticket_template.json` in the current directory by default. Output includes project, issue type, priority, labels, components, and assignee — fields reused when creating new tickets.

---

### `create [-t template.json]`

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

### `update <TICKET-KEY> [-stdin]`

Add a comment to an existing ticket. Opens `$EDITOR` by default; use `-stdin` for piped input.

```bash
jira-thing update PROJ-123
echo "Deployed to staging." | jira-thing update PROJ-123 -stdin
```

Requires `$EDITOR` to be set unless `-stdin` is used.

---

### `my-tasks [-notupdated]`

List open Jira tasks assigned to the current user, ordered by last updated (descending).

```bash
jira-thing my-tasks
jira-thing my-tasks -notupdated
```

`-notupdated` filters to tasks with no activity in the last 3 business days (stale tasks), ordered oldest-first.

---

### `clear-auth`

Remove stored Jira credentials from the OS keyring.

```bash
jira-thing clear-auth
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

# 4. Check your open tasks
jira-thing my-tasks
```

## Template search path

Templates are plain JSON files. Place `ticket_template.json` in one of the search path locations above to avoid specifying `-t` on every `create` invocation.
