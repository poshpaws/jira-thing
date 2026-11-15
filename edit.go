package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"jira-thing/internal/api"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/tui"
)

// editFlags holds runEdit's parsed flags and ticket key.
type editFlags struct {
	key             string
	req             ops.UpdateRequest
	descriptionFile string
	dryRun          bool
	json            bool
}

// parseEditFlags parses runEdit's flag set, exiting on a parse error or a missing key.
func parseEditFlags(args []string) editFlags {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	summary := fs.String("summary", "", "New summary/title")
	descriptionFile := fs.String("description-file", "", "Path to a markdown file for the new description")
	priority := fs.String("priority", "", "New priority name (e.g. High, Medium, Low)")
	labels := fs.String("labels", "", "Comma-separated labels (replaces all existing labels)")
	assignee := fs.String("assignee", "", `Who to assign: "me", "unassign", or a Jira account ID`)
	parent := fs.String("parent", "", "Epic/parent ticket key to link this ticket to")
	dryRun := fs.Bool("dry-run", false, "Show the resolved payload without changing anything")
	jsonOut := fs.Bool("json", false, "Print {key, url, status} as JSON instead of styled text")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 1 {
		fatal("usage: jira-thing edit <TICKET-KEY> [-summary S] [-description-file F] [-priority P] [-labels a,b] [-assignee me|unassign|ACCOUNT_ID] [-parent KEY] [-dry-run] [-json]")
	}
	req := ops.UpdateRequest{
		Key: fs.Arg(0), Summary: *summary, Priority: *priority,
		Labels: template.SplitLabels(*labels), Assignee: *assignee, Parent: *parent,
	}
	return editFlags{key: fs.Arg(0), req: req, descriptionFile: *descriptionFile, dryRun: *dryRun, json: *jsonOut}
}

// runEdit mutates fields on an existing Jira ticket non-interactively: only
// the flags given are changed, everything else on the ticket is left alone.
// This is the field-level counterpart to "update", which only adds comments.
func runEdit(args []string) {
	flags := parseEditFlags(args)
	description, err := readDescriptionFile(flags.descriptionFile)
	if err != nil {
		fatal("%v", err)
	}
	flags.req.Description = description

	conn := mustConnect()
	res, err := ops.UpdateTicket(conn, flags.req, flags.dryRun)
	if errors.Is(err, ops.ErrNothingToUpdate) {
		fatal("nothing to edit: pass at least one of -summary, -description-file, -priority, -labels, -assignee, -parent")
	}
	if err != nil {
		fatal("%v", err)
	}
	reportEditResult(conn, res, flags.json)
}

// reportEditResult prints the dry-run requests, or the edited ticket.
func reportEditResult(conn api.JiraConnection, res ops.UpdateResult, jsonOut bool) {
	if res.Status == ops.StatusDryRun {
		printEditDryRun(conn, res)
		return
	}
	if jsonOut {
		printJSONResult(jsonResult{Key: res.Key, URL: res.URL, Status: res.Status})
		return
	}
	fmt.Printf("%s %s\n", tui.SuccessStyle.Render("Edited ticket:"), tui.KeyStyle.Render(res.Key))
	fmt.Printf("URL: %s\n", res.URL)
}

// printEditDryRun shows what "edit" would send: the field-update PUT if any
// fields were given, and the parent-link call if -parent was given.
func printEditDryRun(conn api.JiraConnection, res ops.UpdateResult) {
	if len(res.Fields) > 0 {
		printDryRun("PUT", conn.BaseURL+api.IssueEndpoint+"/"+res.Key, res.Fields)
	}
	if res.Parent != "" {
		fmt.Printf("Would also link %s to parent %s\n", res.Key, res.Parent)
	}
}

// readDescriptionFile returns the markdown in path, or "" when no path is given.
func readDescriptionFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path) // #nosec G304 -- user-supplied CLI input
	if err != nil {
		return "", fmt.Errorf("reading description file: %w", err)
	}
	return string(data), nil
}
