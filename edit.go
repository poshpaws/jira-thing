package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"jira-thing/internal/api"
	"jira-thing/internal/config"
	"jira-thing/internal/ops"
	"jira-thing/internal/template"
	"jira-thing/internal/termsafe"
	"jira-thing/internal/tui"
)

// editUsage is the edit command's usage line.
const editUsage = "usage: jira-thing edit <TICKET-KEY> [-summary S] [-description-file F] [-priority P] [-labels a,b] [-assignee me|unassign|NAME|EMAIL|ACCOUNT_ID] [-parent KEY] [-label [-]L]... [-component [-]C]... [-fix-version [-]V]... [-type T] [-points N] [-custom Name=value]... [-dry-run] [-json]"

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
	descriptionFile := fs.String("description-file", "", `Path to a markdown file for the new description, or "-" for stdin`)
	priority := fs.String("priority", "", "New priority name (e.g. High, Medium, Low)")
	labels := fs.String("labels", "", "Comma-separated labels (replaces all existing labels)")
	assignee := fs.String("assignee", "", `Who to assign: "me", "unassign", a name, an email, or a Jira account ID`)
	parent := fs.String("parent", "", "Epic/parent ticket key to link this ticket to")
	issueType := fs.String("type", "", "New issue type, e.g. Task (only types the ticket's edit screen allows)")
	var custom customFlag
	fs.Var(&custom, "custom", customHelp)
	var labelEdits, componentEdits, versionEdits listEditFlag
	fs.Var(&labelEdits, "label", `Add a label, or remove one with "-name" (repeatable; keeps the others)`)
	fs.Var(&componentEdits, "component", `Add a component, or remove one with "-name" (repeatable)`)
	fs.Var(&versionEdits, "fix-version", `Add a fix version, or remove one with "-name" (repeatable)`)
	storyPoints := fs.String("points", "", `Story points, e.g. 3 or 0.5 (field: "story_points_field" in config, else the usual defaults)`)
	dryRun := fs.Bool("dry-run", false, "Show the resolved payload without changing anything")
	jsonOut := fs.Bool("json", false, "Print {key, url, status} as JSON instead of styled text")
	if err := fs.Parse(reorderFlagsBeforeKey(fs, args)); err != nil || fs.NArg() < 1 {
		fatal(editUsage)
	}
	req := ops.UpdateRequest{
		Key: fs.Arg(0), Summary: *summary, Priority: *priority,
		Labels: template.SplitLabels(*labels), Assignee: *assignee, Parent: *parent, IssueType: *issueType,
		StoryPoints: parsePoints(*storyPoints), PointFields: configuredPointFields(), Custom: []ops.CustomValue(custom),
		ListEdits: ops.ListEdits{Labels: labelEdits, Components: componentEdits, FixVersions: versionEdits},
	}
	return editFlags{key: fs.Arg(0), req: req, descriptionFile: *descriptionFile, dryRun: *dryRun, json: *jsonOut}
}

// parsePoints reads -points, nil when not given; it exits on a non-number.
func parsePoints(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fatal("-points %q: %v", s, ops.ErrBadStoryPoints)
	}
	return &v
}

// configuredPointFields returns the story point fields to try, the
// configured one first; an unreadable config falls back to the defaults.
func configuredPointFields() []string {
	cfg, err := config.Load()
	if err != nil {
		return ops.DefaultStoryPointFields
	}
	return ops.StoryPointFields(cfg)
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
		fatal("nothing to edit: pass at least one of -summary, -description-file, -priority, -labels, -assignee, -parent, -label, -component, -fix-version, -type, -points, -custom")
	}
	if err != nil {
		fatal("%s", termsafe.Clean(err.Error()))
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
	if res.IssueType != "" {
		fmt.Printf("Type: %s\n", termsafe.Line(res.IssueType))
	}
	if res.PointsField != "" {
		fmt.Printf("Story points: %v (%s)\n", res.Fields[res.PointsField], res.PointsField)
	}
	fmt.Printf("URL: %s\n", res.URL)
}

// printEditDryRun shows what "edit" would send: the field-update PUT if any
// fields were given, and the parent-link call if -parent was given.
func printEditDryRun(conn api.JiraConnection, res ops.UpdateResult) {
	if body := api.EditBody(res.Fields, res.Update); len(body) > 0 {
		printDryRun("PUT", conn.BaseURL+api.IssueEndpoint+"/"+res.Key, body)
	}
	if res.Parent != "" {
		fmt.Printf("Would also link %s to parent %s\n", res.Key, res.Parent)
	}
}

// stdinPath is the -description-file value that reads standard input.
const stdinPath = "-"

// maxStdinDescription caps a description read from stdin, so a runaway pipe
// fails clearly instead of being sent to Jira.
const maxStdinDescription = 1 << 20

// readDescriptionFile returns the markdown in path ("-" for stdin), or ""
// when no path is given. Empty stdin is an error, not a cleared description.
func readDescriptionFile(path string) (string, error) {
	switch path {
	case "":
		return "", nil
	case stdinPath:
		return readStdinDescription()
	}
	data, err := os.ReadFile(path) // #nosec G304 -- user-supplied CLI input
	if err != nil {
		return "", fmt.Errorf("reading description file: %w", err)
	}
	return string(data), nil
}

// readStdinDescription reads a markdown description piped on stdin.
func readStdinDescription() (string, error) {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinDescription+1))
	switch {
	case err != nil:
		return "", fmt.Errorf("reading description from stdin: %w", err)
	case len(data) > maxStdinDescription:
		return "", fmt.Errorf("description on stdin is over %d MB", maxStdinDescription>>20)
	case strings.TrimSpace(string(data)) == "":
		return "", errors.New("description on stdin is empty")
	}
	return string(data), nil
}
