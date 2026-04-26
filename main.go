package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"jira-client/internal/api"
	"jira-client/internal/auth"
	"jira-client/internal/template"
)

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
		auth.ClearCredentials()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: jira-client <command> [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  template <TICKET-KEY> [-o output.json]  Fetch a ticket and save as template")
	fmt.Fprintln(os.Stderr, "  create [-t template.json]               Create a new ticket from a template")
	fmt.Fprintln(os.Stderr, "  my-tasks [-notupdated]                  List open tasks assigned to you")
	fmt.Fprintln(os.Stderr, "  clear-auth                              Clear stored credentials")
}

func buildConnection() (api.JiraConnection, error) {
	url, email, token, err := auth.GetCredentials()
	if err != nil {
		return api.JiraConnection{}, err
	}
	return api.JiraConnection{BaseURL: url, Email: email, APIToken: token}, nil
}

func runTemplate(args []string) {
	fs := flag.NewFlagSet("template", flag.ExitOnError)
	output := fs.String("o", "", "Output file path")
	if err := fs.Parse(args); err != nil || fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: jira-client template <TICKET-KEY> [-o output.json]")
		os.Exit(1)
	}
	ticketKey := fs.Arg(0)

	conn, err := buildConnection()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Fetching %s...\n", ticketKey)
	issue, err := api.FetchIssue(conn, ticketKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching issue: %v\n", err)
		os.Exit(1)
	}

	tmpl := template.Build(issue)
	saved, err := template.Save(tmpl, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error saving template: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Template saved to %s\n", saved)
	out, _ := json.MarshalIndent(tmpl, "", "  ")
	fmt.Println(string(out))
}

func runCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	templatePath := fs.String("t", "", "Path to template file")
	fs.Parse(args) //nolint:errcheck

	tmpl, err := template.Load(*templatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading template: %v\n", err)
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter ticket summary: ")
	summary, _ := reader.ReadString('\n')
	summary = strings.TrimSpace(summary)
	if summary == "" {
		fmt.Fprintln(os.Stderr, "Summary is required.")
		os.Exit(1)
	}

	fmt.Print("Enter ticket description: ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	tmpl["summary"] = summary
	tmpl["description"] = map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": description},
				},
			},
		},
	}

	conn, err := buildConnection()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	result, err := api.CreateIssue(conn, tmpl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating issue: %v\n", err)
		os.Exit(1)
	}

	key, _ := result["key"].(string)
	fmt.Printf("Created ticket: %s\n", key)
	fmt.Printf("URL: %s/browse/%s\n", conn.BaseURL, key)
}

func runMyTasks(args []string) {
	fs := flag.NewFlagSet("my-tasks", flag.ExitOnError)
	notUpdated := fs.Bool("notupdated", false, "Only show tasks with no updates in the last 3 business days")
	fs.Parse(args) //nolint:errcheck

	jql := `assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC`
	if *notUpdated {
		cutoff := threeBusinessDaysAgo(time.Now())
		jql = fmt.Sprintf(
			`assignee = currentUser() AND statusCategory != Done AND updated <= "%s" ORDER BY updated ASC`,
			cutoff.Format("2006/01/02"),
		)
	}

	conn, err := buildConnection()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fields := []string{"summary", "status", "priority", "updated"}
	result, err := api.SearchIssues(conn, jql, fields, 100)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching tasks: %v\n", err)
		os.Exit(1)
	}

	if result.Total == 0 {
		fmt.Println("No tasks found.")
		return
	}

	label := "open"
	if *notUpdated {
		label = "stale (no updates in 3+ business days)"
	}
	fmt.Printf("Found %d %s task(s):\n\n", result.Total, label)

	for _, issue := range result.Issues {
		key, _ := issue["key"].(string)
		f, _ := issue["fields"].(map[string]any)
		summary, _ := f["summary"].(string)
		status := nestedString(f, "status", "name")
		priority := nestedString(f, "priority", "name")
		updated, _ := f["updated"].(string)
		if len(updated) >= 10 {
			updated = updated[:10] // trim to YYYY-MM-DD
		}
		fmt.Printf("  %-12s  %-14s  %-8s  updated: %s  %s\n", key, status, priority, updated, summary)
	}
}

// nestedString extracts a string from nested maps: m[key1][key2].
func nestedString(m map[string]any, key1, key2 string) string {
	if inner, ok := m[key1].(map[string]any); ok {
		s, _ := inner[key2].(string)
		return s
	}
	return ""
}

// threeBusinessDaysAgo returns the date that is 3 business days before now,
// skipping Saturdays and Sundays.
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
