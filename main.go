package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"jira-thing/internal/api"
	"jira-thing/internal/auth"
	"jira-thing/internal/template"
)

// getCredentialsFn resolves Jira credentials; replaced in tests.
var getCredentialsFn = auth.GetCredentials

// osExit calls os.Exit; replaced in tests to prevent process termination.
var osExit = os.Exit

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "template":
		runTemplate(os.Args[2:])
	case "create":
		runCreate(os.Args[2:])
	case "my-tasks":
		runMyTasks(os.Args[2:])
	case "clear-auth":
		if err := auth.ClearCredentials(); err != nil {
			fatal("clearing credentials: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

// printUsage writes the command summary to stderr.
func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: jira-thing <command> [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  template <TICKET-KEY> [-o output.json]  Fetch a ticket and save as template")
	fmt.Fprintln(os.Stderr, "  create [-t template.json]               Create a new ticket from a template")
	fmt.Fprintln(os.Stderr, "  my-tasks [-notupdated]                  List open tasks assigned to you")
	fmt.Fprintln(os.Stderr, "  clear-auth                              Clear stored credentials")
}

// buildConnection resolves credentials and returns a JiraConnection.
func buildConnection() (api.JiraConnection, error) {
	creds, err := getCredentialsFn()
	if err != nil {
		return api.JiraConnection{}, err
	}
	return api.JiraConnection{BaseURL: creds.URL, Email: creds.Email, APIToken: creds.Token}, nil
}

// mustConnect calls buildConnection and exits on failure.
func mustConnect() api.JiraConnection {
	conn, err := buildConnection()
	if err != nil {
		fatal("connecting: %v", err)
	}
	return conn
}

// fatal prints an error to stderr and exits with code 1.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	osExit(1)
}

// runTemplate fetches a Jira ticket and saves it as a local JSON template.
func runTemplate(args []string) {
	fs := flag.NewFlagSet("template", flag.ExitOnError)
	output := fs.String("o", "", "Output file path")
	if err := fs.Parse(args); err != nil || fs.NArg() < 1 {
		fatal("usage: jira-thing template <TICKET-KEY> [-o output.json]")
	}
	conn := mustConnect()
	fmt.Printf("Fetching %s...\n", fs.Arg(0))
	issue, err := api.FetchIssue(conn, fs.Arg(0))
	if err != nil {
		fatal("fetching issue: %v", err)
	}
	tmpl := template.Build(issue)
	saved, err := template.Save(tmpl, *output)
	if err != nil {
		fatal("saving template: %v", err)
	}
	fmt.Printf("Template saved to %s\n", saved)
	out, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		fatal("marshalling template: %v", err)
	}
	fmt.Println(string(out))
}

// runCreate loads a template, prompts for summary/description, and creates a Jira ticket.
func runCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	templatePath := fs.String("t", "", "Path to template file")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	tmpl, err := template.Load(*templatePath)
	if err != nil {
		fatal("loading template: %v", err)
	}
	summary, description, err := promptTicketFields()
	if err != nil {
		fatal("%v", err)
	}
	tmpl["summary"] = summary
	tmpl["description"] = buildDescription(description)

	conn := mustConnect()
	result, err := api.CreateIssue(conn, tmpl)
	if err != nil {
		fatal("creating issue: %v", err)
	}
	key := getString(result, "key")
	fmt.Printf("Created ticket: %s\n", key)
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}

// promptTicketFields reads summary and description from stdin.
func promptTicketFields() (summary, description string, err error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter ticket summary: ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("reading summary: %w", err)
	}
	summary = strings.TrimSpace(line)
	if summary == "" {
		return "", "", fmt.Errorf("summary is required")
	}
	fmt.Print("Enter ticket description: ")
	line, err = reader.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("reading description: %w", err)
	}
	return summary, strings.TrimSpace(line), nil
}

// buildDescription wraps plain text in the Jira Atlassian Document Format structure.
func buildDescription(text string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type":    "paragraph",
				"content": []any{map[string]any{"type": "text", "text": text}},
			},
		},
	}
}

// runMyTasks lists open Jira tasks assigned to the current user.
// With -notupdated, restricts to tasks idle for 3+ business days.
func runMyTasks(args []string) {
	fs := flag.NewFlagSet("my-tasks", flag.ExitOnError)
	notUpdated := fs.Bool("notupdated", false, "Only show tasks with no updates in the last 3 business days")
	if err := fs.Parse(args); err != nil {
		fatal("parsing flags: %v", err)
	}

	conn := mustConnect()
	q := api.SearchQuery{
		JQL:        buildMyTasksJQL(*notUpdated),
		Fields:     []string{"summary", "status", "priority", "updated"},
		MaxResults: 100,
	}
	result, err := api.SearchIssues(conn, q)
	if err != nil {
		fatal("fetching tasks: %v", err)
	}
	if result.Total == 0 {
		fmt.Println("No tasks found.")
		return
	}
	fmt.Printf("Found %d %s task(s):\n\n", result.Total, taskLabel(*notUpdated))
	printTasks(result.Issues)
}

// buildMyTasksJQL constructs the JQL for the my-tasks query.
// When notUpdated is true, it adds an upper bound on the updated date.
func buildMyTasksJQL(notUpdated bool) string {
	base := `assignee = currentUser() AND resolution = Unresolved`
	if notUpdated {
		cutoff := threeBusinessDaysAgo(time.Now())
		return fmt.Sprintf(`%s AND updated <= "%s" ORDER BY updated ASC`, base, cutoff.Format("2006/01/02"))
	}
	return base + ` ORDER BY updated DESC`
}

// taskLabel returns a human-readable description for the filter mode.
func taskLabel(notUpdated bool) string {
	if notUpdated {
		return "stale (no updates in 3+ business days)"
	}
	return "open"
}

// printTasks renders each issue as a formatted row.
func printTasks(issues []map[string]any) {
	for _, issue := range issues {
		printTaskRow(issue)
	}
}

// printTaskRow prints a single issue line: key, status, priority, last-updated date, summary.
func printTaskRow(issue map[string]any) {
	key := getString(issue, "key")
	f, ok := issue["fields"].(map[string]any)
	if !ok {
		f = map[string]any{}
	}
	summary := getString(f, "summary")
	updated := getString(f, "updated")
	if len(updated) >= 10 {
		updated = updated[:10]
	}
	fmt.Printf("  %-12s  %-14s  %-8s  updated: %s  %s\n",
		key, nestedString(f, "status", "name"), nestedString(f, "priority", "name"), updated, summary)
}

// getString safely extracts a string value from a map by key.
func getString(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return v
}

// nestedString safely extracts m[key1][key2] as a string.
func nestedString(m map[string]any, key1, key2 string) string {
	if inner, ok := m[key1].(map[string]any); ok {
		return getString(inner, key2)
	}
	return ""
}

// threeBusinessDaysAgo returns the date 3 weekdays before now, skipping Sat/Sun.
func threeBusinessDaysAgo(now time.Time) time.Time {
	t := now.Truncate(24 * time.Hour)
	count := 0
	for count < 3 {
		t = t.AddDate(0, 0, -1)
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			count++
		}
	}
	return t
}
