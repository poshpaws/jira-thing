package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
)

// runEdit mutates fields on an existing Jira ticket non-interactively: only
// the flags given are changed, everything else on the ticket is left alone.
// This is the field-level counterpart to "update", which only adds comments.
func runEdit(args []string) {
	// Reorder args so flags (and their values) precede the positional
	// TICKET-KEY — flag.Parse stops at the first non-flag argument, and the
	// key is conventionally given first (e.g. "edit PROJ-1 -priority High").
	var reordered, positional []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			reordered = append(reordered, args[i], args[i+1])
			i++
		} else if strings.HasPrefix(args[i], "-") {
			reordered = append(reordered, args[i])
		} else {
			positional = append(positional, args[i])
		}
	}
	reordered = append(reordered, positional...)

	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	summary := fs.String("summary", "", "New summary/title")
	descriptionFile := fs.String("description-file", "", "Path to a markdown file for the new description")
	priority := fs.String("priority", "", "New priority name (e.g. High, Medium, Low)")
	labels := fs.String("labels", "", "Comma-separated labels (replaces all existing labels)")
	assignee := fs.String("assignee", "", `Who to assign: "me", "unassign", or a Jira account ID`)
	parent := fs.String("parent", "", "Epic/parent ticket key to link this ticket to")
	if err := fs.Parse(reordered); err != nil || fs.NArg() < 1 {
		fatal("usage: jira-thing edit <TICKET-KEY> [-summary S] [-description-file F] [-priority P] [-labels a,b] [-assignee me|unassign|ACCOUNT_ID] [-parent KEY]")
	}
	key := fs.Arg(0)

	conn := mustConnect()
	fields, err := buildEditFields(conn, *summary, *descriptionFile, *priority, *labels, *assignee)
	if err != nil {
		fatal("%v", err)
	}
	if len(fields) == 0 && *parent == "" {
		fatal("nothing to edit: pass at least one of -summary, -description-file, -priority, -labels, -assignee, -parent")
	}

	if len(fields) > 0 {
		if err := api.UpdateIssue(conn, key, fields); err != nil {
			fatal("editing %s: %v", key, err)
		}
	}
	if *parent != "" {
		if err := api.AddIssuesToEpic(conn, *parent, []string{key}); err != nil {
			fatal("linking %s to parent %s: %v", key, *parent, err)
		}
	}
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Edited ticket:"), tui.KeyStyle.Render(key))
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}

// buildEditFields translates edit flags into a Jira fields payload for
// UpdateIssue. Only flags that were actually given produce an entry.
func buildEditFields(conn api.JiraConnection, summary, descriptionFile, priority, labels, assignee string) (map[string]any, error) {
	fields := map[string]any{}
	if summary != "" {
		fields["summary"] = summary
	}
	if descriptionFile != "" {
		data, err := os.ReadFile(descriptionFile) // #nosec G304 -- user-supplied CLI input
		if err != nil {
			return nil, fmt.Errorf("reading description file: %w", err)
		}
		fields["description"] = buildDescription(string(data))
	}
	if priority != "" {
		fields["priority"] = map[string]any{"name": priority}
	}
	if labels != "" {
		fields["labels"] = template.SplitLabels(labels)
	}
	if assignee != "" {
		resolved, err := template.ResolveAssigneeArg(conn, assignee)
		if err != nil {
			return nil, err
		}
		fields["assignee"] = resolved
	}
	return fields, nil
}
